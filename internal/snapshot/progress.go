package snapshot

import (
	"strings"
	"sync/atomic"
)

// Progress is how far the first build has got, readable from another goroutine
// while that build runs. The server answers requests during the first scan and
// says how much has been read; nothing in the build waits on the reader of it.
// The zero value is ready, and a nil *Progress is a build nobody is watching.
type Progress struct {
	total atomic.Int64
	done  atomic.Int64
	// folder is the directory of the file the build is at, and last is the
	// build's own copy of it, kept so that a folder is published once rather
	// than once per file. The build is the only writer of last.
	folder atomic.Pointer[string]
	last   string
}

// Files reports how many of the vault's files the build has gone through and
// how many there are. The total is zero until the folder has been walked, which
// is the part of the wait before the number is known.
func (p *Progress) Files() (done, total int) {
	if p == nil {
		return 0, 0
	}
	return int(p.done.Load()), int(p.total.Load())
}

// Folder is the folder of the file the build is reading, as a path within the
// vault, and empty for the vault's own folder or before the build has begun.
func (p *Progress) Folder() string {
	if p == nil {
		return ""
	}
	if folder := p.folder.Load(); folder != nil {
		return *folder
	}
	return ""
}

func (p *Progress) begin(total int) {
	if p == nil {
		return
	}
	p.total.Store(int64(total))
	p.done.Store(0)
}

// reached records that the build is at file, which is the done-th of the
// folder, so the folder it sits in is the one being read.
func (p *Progress) reached(done int, file string) {
	if p == nil {
		return
	}
	p.done.Store(int64(done))
	folder, _, found := strings.CutLast(file, "/")
	if !found {
		// A file at the top of the vault is in no folder; CutLast hands back the
		// whole name when there is no separator.
		folder = ""
	}
	if folder != p.last {
		p.last = folder
		p.folder.Store(&folder)
	}
}

// Option adjusts how New builds the first generation.
type Option func(*options)

type options struct {
	progress *Progress
}

// WithProgress has New report the first build into p.
func WithProgress(p *Progress) Option {
	return func(o *options) { o.progress = p }
}
