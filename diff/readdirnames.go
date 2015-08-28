package diff

import (
	"os"
	"sort"
	"syscall"
	"unsafe"
)

// {name,inode} pairs used to support the early-pruning logic of the walker type
type nameIno struct {
	name string
	ino  uint64
}

// readdirnames is a hacked-apart version of the Go stdlib code, exposing inode
// numbers further up the stack when reading directory contents. Unlike
// os.Readdirnames, which returns a list of filenames, this function returns a
// list of {filename,inode} pairs.
func readdirnames(dirname string) (names []nameIno, err error) {
	var (
		size = 100
		buf  = make([]byte, 4096)
		nbuf int
		bufp int
		nb   int
	)

	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	names = make([]nameIno, 0, size) // Empty with room to grow.
	for {
		// Refill the buffer if necessary
		if bufp >= nbuf {
			bufp = 0
			nbuf, err = syscall.ReadDirent(int(f.Fd()), buf) // getdents on linux
			if nbuf < 0 {
				nbuf = 0
			}
			if err != nil {
				return nil, os.NewSyscallError("readdirent", err)
			}
			if nbuf <= 0 {
				break // EOF
			}
		}

		// Drain the buffer
		nb, names = parseDirent(buf[bufp:nbuf], names)
		bufp += nb
	}

	sl := nameInoSlice(names)
	sort.Sort(sl)
	return sl, nil
}

type nameInoSlice []nameIno

func (s nameInoSlice) Len() int           { return len(s) }
func (s nameInoSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s nameInoSlice) Less(i, j int) bool { return s[i].name < s[j].name }

// parseDirent is a minor modification of syscall.ParseDirent (linux version)
// which returns {name,inode} pairs instead of just names.
func parseDirent(buf []byte, names []nameIno) (consumed int, newnames []nameIno) {
	origlen := len(buf)
	for len(buf) > 0 {
		dirent := (*syscall.Dirent)(unsafe.Pointer(&buf[0]))
		buf = buf[dirent.Reclen:]
		if dirent.Ino == 0 { // File absent in directory.
			continue
		}
		bytes := (*[10000]byte)(unsafe.Pointer(&dirent.Name[0]))
		var name = string(bytes[0:clen(bytes[:])])
		if name == "." || name == ".." { // Useless names
			continue
		}
		names = append(names, nameIno{name, dirent.Ino})
	}
	return origlen - len(buf), names
}

func clen(n []byte) int {
	for i := 0; i < len(n); i++ {
		if n[i] == 0 {
			return i
		}
	}
	return len(n)
}
