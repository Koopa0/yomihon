package snapshot

import "sync/atomic"

// Progress is how far the first build has got, readable from another goroutine
// while that build runs. The server answers requests during the first scan and
// says how much has been read; nothing in the build waits on the reader of it.
// The zero value is ready, and a nil *Progress is a build nobody is watching.
type Progress struct {
	total atomic.Int64
	done  atomic.Int64
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

func (p *Progress) begin(total int) {
	if p == nil {
		return
	}
	p.total.Store(int64(total))
	p.done.Store(0)
}

func (p *Progress) reached(done int) {
	if p == nil {
		return
	}
	p.done.Store(int64(done))
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
