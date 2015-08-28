package diff

import (
	"fmt"
)

type Change struct {
	kind ChangeType
	path string
}

// ChangeType represents the change type.
type ChangeType int

const (
	// ChangeModify represents the modify operation.
	ChangeModify = iota
	// ChangeAdd represents the add operation.
	ChangeAdd
	// ChangeDelete represents the delete operation.
	ChangeDelete
)

func reportChange(path string, k ChangeType) {
	c := Change{k, path}
	fmt.Printf("%s\n", c)
}

func (c Change) String() string {
	var kind string
	switch c.kind {
	case ChangeModify:
		kind = "M"
	case ChangeAdd:
		kind = "A"
	case ChangeDelete:
		kind = "D"
	}
	return fmt.Sprintf("%s\t%s", kind, c.path)
}
