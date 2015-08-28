package diff

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"syscall"
	"unsafe"
)

// In most usecases, it's probably better to spuriously signal as changed than
// to not detect changes.
const resultOnFailure = -1

func mmapCompare(p1, p2 string, size int64) int {
	// mmap fails on files with zero size, but zero-size files necessarily have
	// the same contents.
	if size == 0 {
		return 0
	}
	f1, err := os.OpenFile(p1, os.O_RDONLY, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file %s in mmapCompare: %s\n", p1, err.Error())
		return resultOnFailure
	}
	defer f1.Close()
	f2, err := os.OpenFile(p2, os.O_RDONLY, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening file %s in mmapCompare: %s\n", p2, err.Error())
		return resultOnFailure
	}
	defer f2.Close()

	mm1, err := mmap(f1, size)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error mmap'ing file %s in mmapCompare: %s\n", p1, err.Error())
		return resultOnFailure
	}
	defer unmap(mm1)
	mm2, err := mmap(f2, size)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error mmap'ing file %s in mmapCompare: %s\n", p2, err.Error())
		return resultOnFailure
	}
	defer unmap(mm2)

	return bytes.Compare(*mm1, *mm2)
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
