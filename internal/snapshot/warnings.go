package snapshot

import (
	"time"

	"github.com/koopa0/yomihon/internal/vault"
)

const (
	warningRead    = "source"
	warningSidecar = "sidecar"
	warningSize    = "size"
	warningPanic   = "panic"
	warningVerdict = "verdict"
	warningHead    = "head"
)

type warningKey struct {
	path     string
	kind     string
	size     int64
	modified time.Time
	message  string
}

// warningAttempt remembers offered diagnostics separately from publication:
// even a retained candidate observed recovery, but a canceled walk did not.
type warningAttempt struct {
	reported map[warningKey]struct{}
	seen     map[warningKey]struct{}
}

func newWarningAttempt(reported map[warningKey]struct{}) *warningAttempt {
	return &warningAttempt{reported: reported, seen: make(map[warningKey]struct{})}
}

// record keeps a suppressed warning in this observation and remembers a new
// report immediately, so cancellation cannot cause its next attempt to repeat.
func (a *warningAttempt) record(entry vault.Entry, kind, message string) bool {
	if a == nil {
		return true
	}
	key := warningKey{
		path:     entry.Path(),
		kind:     kind,
		size:     entry.Size(),
		modified: entry.ModTime().Round(0).UTC(),
		message:  message,
	}
	_, repeated := a.seen[key]
	a.seen[key] = struct{}{}
	if _, reported := a.reported[key]; reported || repeated {
		return false
	}
	if a.reported != nil {
		a.reported[key] = struct{}{}
	}
	return true
}

// complete forgets only problems absent from a finished observation. Calls
// that abandon their candidate leave the previous episode history intact.
func (a *warningAttempt) complete() {
	if a.reported == nil {
		return
	}
	clear(a.reported)
	for key := range a.seen {
		a.reported[key] = struct{}{}
	}
}
