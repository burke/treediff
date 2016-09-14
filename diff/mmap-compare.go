package diff

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"syscall"
	"unsafe"
)

// In most usecases, it's probably better to spuriously signal as changed than
// to not detect changes.
const resultOnFailure = -1

// 0: files are equal, no copy necessary
// 1 or -1: files are not equal, must be copied.
func mmapCompare(p1, p2 string, size int64) int {
	var (
		err, origErr error
		f1, f2       *os.File
		mm1, mm2     *[]byte
	)
	// mmap fails on files with zero size, but zero-size files necessarily have
	// the same contents.
	if size == 0 {
		return 0
	}
	f1, err = os.OpenFile(p1, os.O_RDONLY, 0)
	if err != nil {
		origErr = fmt.Errorf("error opening file %s: %s", p1, err.Error())
		goto maybeSymlink
	}
	defer f1.Close()
	f2, err = os.OpenFile(p2, os.O_RDONLY, 0)
	if err != nil {
		origErr = fmt.Errorf("error opening file %s: %s", p2, err.Error())
		goto maybeSymlink
	}
	defer f2.Close()

	mm1, err = mmap(f1, size)
	if err != nil {
		origErr = fmt.Errorf("error mmap'ing file %s: %s", p1, err.Error())
		goto maybeSymlink
	}
	defer unmap(mm1)
	mm2, err = mmap(f2, size)
	if err != nil {
		origErr = fmt.Errorf("error mmap'ing file %s: %s", p2, err.Error())
		goto maybeSymlink
	}
	defer unmap(mm2)

	return bytes.Compare(*mm1, *mm2)

maybeSymlink:

	stat1, err := os.Lstat(p1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lstat failed for %s: %s\n", p1, err.Error())
	}
	_ = stat1

	stat2, err := os.Lstat(p2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lstat failed for %s: %s\n", p2, err.Error())
	}
	_ = stat2

	p1IsSym := stat1.Mode()&os.ModeSymlink != 0
	p2IsSym := stat2.Mode()&os.ModeSymlink != 0

	if p1IsSym && p2IsSym {
		// both files are symlinks that can't be resolved in the host FS.
		// check that both resolve to the same path instead of comparing their contents.
		rl1, err := os.Readlink(p1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "couldn't readlink on %s: %s\n", p1, err.Error())
			return resultOnFailure
		}

		rl2, err := os.Readlink(p2)
		if err != nil {
			fmt.Fprintf(os.Stderr, "couldn't readlink on %s: %s\n", p2, err.Error())
			return resultOnFailure
		}

		return strings.Compare(rl1, rl2)
	} else if !p1IsSym && !p2IsSym {
		// open failed with no good reason.
		fmt.Fprintln(os.Stderr, origErr)
		return resultOnFailure
	}
	// only one is a symlink
	return 1 // guess they're different...?
}

func mmap(f *os.File, size int64) (*[]byte, error) {
	fd := int(f.Fd())
	bs, err := syscall.Mmap(fd, 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	return &bs, err
}

func unmap(b *[]byte) {
	dh := (*reflect.SliceHeader)(unsafe.Pointer(b))
	syscall.Syscall(syscall.SYS_MUNMAP, dh.Data, uintptr(dh.Len), 0)
}
