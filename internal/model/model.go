// Package model defines the core data types of adrm: items currently sitting
// in the trash bin and append-only reflog entries describing every operation.
package model

// Item is a file or directory currently inside the trash bin (table "bin").
// Rows are removed from the bin table when an item is restored or purged;
// the global auto-increment id is never reused.
type Item struct {
	ID         int64
	OrigPath   string // absolute path before recycling
	IsDir      bool
	Size       int64 // bytes; for directories the recursive sum at recycle time
	Mode       uint32
	UID        int
	GID        int
	Owner      string
	Group      string
	Mtime      int64 // original modification time, unix seconds
	TrashPath  string
	RecycledAt int64
	ExpireAt   int64
	Method     string // "rename" or "copy+delete"
	Note       string // error detail when the item is in an exception state
}

// Exception reports whether the item needs attention.
func (it *Item) Exception() bool { return it.Note != "" }

// Operation kinds recorded in the reflog (table "reflog").
const (
	OpRecycle   = "recycle"
	OpRestore   = "restore"
	OpPurge     = "purge"
	OpExpire    = "expire"
	OpException = "exception"
	OpReset     = "reset"
	OpEmpty     = "empty"
)

// Reflog is one immutable history row. It survives the file it describes.
type Reflog struct {
	Seq       int64
	Ts        int64
	Op        string
	ItemID    int64
	OrigPath  string
	TrashPath string
	Size      int64
	Detail    string
}

// SortKey orders query results.
type SortKey struct {
	Field string
	Desc  bool
}

// Filter selects items from the bin or reflog tables. All non-zero fields
// are combined with AND.
type Filter struct {
	IDs        []int64
	Name       string
	PathPrefix string
	Expired    bool
	States     []string // bin: recycled / exception
	Ops        []string // reflog: recycle / restore / purge / expire / exception / reset / empty
	RecycledLo *int64
	RecycledHi *int64
	Mtime      *TimeSpec
	Size       *SizeSpec
	OpLo       *int64 // reflog ts window
	OpHi       *int64
	Last       int
	Sort       []SortKey
}
