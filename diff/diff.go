package diff

import (
	"os"
	"strings"
	"sync"
)

const (
	walkRequestWorkers            = 8
	compareContentsRequestWorkers = 8
)

type compareContentsRequest struct {
	path, p1, p2 string
	size         int64
}

type walkRequest struct {
	path   string
	i1, i2 os.FileInfo
}

// Changes scans two parallel directories concurrently. It generates a list of
// files that have changed between the two trees, along with whether that file
// as added, deleted, or modified from the first tree to the next. The output
// format is very similar to `git diff --name-status --no-renames`; however,
// the lines are not likely to be sorted.
//
// Changes also specifically does not consider mtime/ctime changes to be
// sufficient to consider a file "changed" -- files with only mtime changes but
// no other changes (including contents) will not be reported.
func Changes(dir1, dir2 string) (changes []Change, err error) {
	if !strings.HasSuffix(dir1, "/") {
		dir1 += "/"
	}
	if !strings.HasSuffix(dir2, "/") {
		dir2 += "/"
	}
	w := &walker{
		dir1: dir1,
		dir2: dir2,
		compareContentsRequests: make(chan compareContentsRequest, 32),
		walkRequests:            make(chan walkRequest, 4096),
		changes:                 make(chan Change, 16),
	}

	var workers sync.WaitGroup

	// Spawn a worker to follow the changes channel, enqueueing new changes to
	// the return slice.
	workers.Add(1)
	go func() {
		defer workers.Done()
		for chg := range w.changes {
			changes = append(changes, chg)
		}
	}()

	// Spawn workers to recursively compare two parallel directories. Each
	// invocation will enqueue more requests for children.
	workers.Add(walkRequestWorkers)
	for i := 0; i < 8; i++ {
		go func() {
			defer workers.Done()
			for req := range w.walkRequests {
				if err := w.walk(req.path, req.i1, req.i2); err != nil {
					w.err = err
				}
				w.requests.Done()
			}
		}()
	}

	// Spawn workers to compare two parallel files which may or may not have
	// identical contents, recording a change if the contents differ.
	workers.Add(compareContentsRequestWorkers)
	for i := 0; i < 8; i++ {
		go func() {
			defer workers.Done()
			for req := range w.compareContentsRequests {
				if 0 != mmapCompare(req.p1, req.p2, req.size) {
					reportChange(req.path, ChangeModify)
				}
				w.requests.Done()
			}
		}()
	}

	// Run the first iteration of the walker synchronously. Children of this
	// parent directory will be enqueued to run in the walkRequest workers above.
	if err := kickOffWalker(w); err != nil {
		return nil, err
	}

	w.requests.Wait()                // wait until trees have been fully traversed
	close(w.compareContentsRequests) // close request channels
	close(w.walkRequests)            // (idem)
	close(w.changes)                 // (idem)
	workers.Wait()                   // wait for channel ranges to shut down

	return changes, w.err
}

func kickOffWalker(w *walker) error {
	i1, err := os.Lstat(w.dir1)
	if err != nil {
		return err
	}
	i2, err := os.Lstat(w.dir2)
	if err != nil {
		return err
	}
	if err := w.walk("", i1, i2); err != nil {
		return err
	}
	return nil
}
