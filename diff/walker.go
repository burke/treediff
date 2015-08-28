package diff

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

type walker struct {
	dir1                    string
	dir2                    string
	err                     error
	compareContentsRequests chan compareContentsRequest
	walkRequests            chan walkRequest
	changes                 chan Change
	requests                sync.WaitGroup
}

func (w *walker) walk(path string, i1, i2 os.FileInfo) (err error) {
	is1Dir := i1 != nil && i1.IsDir()
	is2Dir := i2 != nil && i2.IsDir()

	sameDevice := false
	if i1 != nil && i2 != nil {
		si1 := i1.Sys().(*syscall.Stat_t)
		si2 := i2.Sys().(*syscall.Stat_t)
		if si1.Dev == si2.Dev {
			sameDevice = true
		}
	}

	is1File := i1 != nil && !i1.IsDir()
	is2File := i2 != nil && !i2.IsDir()

	if is1File && !is2File {
		reportChange(path, ChangeDelete)
	} else if !is1File && is2File {
		reportChange(path, ChangeAdd)
	} else if is1File && is2File {
		// maybe modified?
		s1 := i1.Sys().(*syscall.Stat_t)
		s2 := i2.Sys().(*syscall.Stat_t)
		if statDifferent(s1, s2) {
			reportChange(path, ChangeModify)
		} else {
			w.requests.Add(1)
			w.compareContentsRequests <- compareContentsRequest{path, w.dir1 + path, w.dir2 + path, s1.Size}
		}
	}

	// If these files are both non-existent, or leaves (non-dirs), we are done.
	if !is1Dir && !is2Dir {
		return nil
	}

	// Fetch the names of all the files contained in both directories being walked:
	var names1, names2 []nameIno
	if is1Dir {
		names1, err = readdirnames(filepath.Join(w.dir1, path)) // getdents(2): fs access
		if err != nil {
			return err
		}
	}
	if is2Dir {
		names2, err = readdirnames(filepath.Join(w.dir2, path)) // getdents(2): fs access
		if err != nil {
			return err
		}
	}

	// We have lists of the files contained in both parallel directories, sorted
	// in the same order. Walk them in parallel, generating a unique merged list
	// of all items present in either or both directories.
	var names []string
	ix1 := 0
	ix2 := 0

	for {
		if ix1 >= len(names1) {
			break
		}
		if ix2 >= len(names2) {
			break
		}

		ni1 := names1[ix1]
		ni2 := names2[ix2]

		switch bytes.Compare([]byte(ni1.name), []byte(ni2.name)) {
		case -1: // ni1 < ni2 -- advance ni1
			// we will not encounter ni1 in names2
			names = append(names, ni1.name)
			ix1++
		case 0: // ni1 == ni2
			if ni1.ino != ni2.ino || !sameDevice {
				names = append(names, ni1.name)
			}
			ix1++
			ix2++
		case 1: // ni1 > ni2 -- advance ni2
			// we will not encounter ni2 in names1
			names = append(names, ni2.name)
			ix2++
		}
	}
	for ix1 < len(names1) {
		names = append(names, names1[ix1].name)
		ix1++
	}
	for ix2 < len(names2) {
		names = append(names, names2[ix2].name)
		ix2++
	}

	// For each of the names present in either or both of the directories being
	// iterated, stat the name under each root, and recurse the pair of them:
	for _, name := range names {
		fname := filepath.Join(path, name)
		var cInfo1, cInfo2 os.FileInfo
		if is1Dir {
			cInfo1, err = os.Lstat(filepath.Join(w.dir1, fname)) // lstat(2): fs access
			if err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if is2Dir {
			cInfo2, err = os.Lstat(filepath.Join(w.dir2, fname)) // lstat(2): fs access
			if err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		// TODO(burke): long-running workers instead of one-offs
		// if err = w.walk(fname, cInfo1, cInfo2); err != nil {
		// 	return err
		// }
		w.requests.Add(1)
		select {
		case w.walkRequests <- walkRequest{fname, cInfo1, cInfo2}:
		default:
			go func() {
				w.walkRequests <- walkRequest{fname, cInfo1, cInfo2}
			}()
		}
	}
	return nil
}

func statDifferent(o *syscall.Stat_t, n *syscall.Stat_t) bool {
	return o.Mode != n.Mode || o.Uid != n.Uid || o.Gid != n.Gid ||
		o.Rdev != n.Rdev || o.Size != n.Size
}
