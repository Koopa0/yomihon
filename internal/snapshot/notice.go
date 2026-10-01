package snapshot

import (
	"errors"
	"slices"
	"strconv"

	"github.com/koopa0/yomihon/internal/vault"
)

// NoticeReason names why the pages owe their reader a standing sentence about
// the folder, over and above the sources a build could not read. It is a closed
// set: a surface that words a notice switches over it, so a reason added here
// is a gap the linter finds in every place that has no words for it yet.
type NoticeReason uint8

const (
	// NoticeNamesCollide: two names in the folder normalize to one path, so
	// every scan is refused and nothing written since the last good one is
	// being published.
	NoticeNamesCollide NoticeReason = iota + 1

	// noticeReasonEnd closes the set [NoticeReasons] walks. It is not a reason.
	noticeReasonEnd
)

// NoticeReasons is every reason there is, derived from the declarations above
// so that a reason added there is in it without anyone remembering to say so.
func NoticeReasons() []NoticeReason {
	reasons := make([]NoticeReason, 0, noticeReasonEnd-1)
	for reason := NoticeNamesCollide; reason < noticeReasonEnd; reason++ {
		reasons = append(reasons, reason)
	}
	return reasons
}

// noticeReasonNames is each reason's name, sized by the sentinel so that a
// reason added above is a slot here that is empty until it is filled.
var noticeReasonNames = [noticeReasonEnd]string{
	NoticeNamesCollide: "names_collide",
}

// String is the reason's name, for the log line or the failure message that
// would otherwise print a number.
func (r NoticeReason) String() string {
	if r < noticeReasonEnd && noticeReasonNames[r] != "" {
		return noticeReasonNames[r]
	}
	return "notice_" + strconv.Itoa(int(r))
}

// Notice is one standing fact about how the pages relate to the folder: why,
// and which files it is about. It carries no words, because which language to
// say it in is the surface's to decide.
type Notice struct {
	Reason NoticeReason
	// Paths are the files the reason is about, as the filesystem spells them.
	// Two spellings of one name print alike, so these are the raw strings and a
	// surface that shows them has to tell the pair apart itself.
	Paths []string
}

// attempt is the live account as it stands now, never nil. Only the
// reconciliation loop calls it, because only that loop writes the account.
func (s *Store) attempt() *liveAttempt {
	if current := s.fresh.Load(); current != nil {
		return current
	}
	return &liveAttempt{}
}

// noteRefusedScan records what a scan that failed says about the folder. A
// refusal that names two colliding files becomes the notice that says so; any
// other failure leaves the folder unexplained, which is what it was before,
// and takes the collision notice down because that is no longer what is
// refusing the scan.
func (s *Store) noteRefusedScan(err error) {
	if collision, ok := errors.AsType[*vault.CollisionError](err); ok {
		s.setNotice(Notice{Reason: NoticeNamesCollide, Paths: slices.Clone(collision.Paths[:])})
		return
	}
	s.clearNotice(NoticeNamesCollide)
}

// setNotice puts n in the live account in place of any notice of its reason.
func (s *Store) setNotice(n Notice) {
	current := s.attempt()
	for _, held := range current.notices {
		if held.Reason == n.Reason && slices.Equal(held.Paths, n.Paths) {
			return
		}
	}
	s.fresh.Store(current.withoutNotice(n.Reason).withNotice(n))
}

// clearNotice takes the notice of one reason out of the live account.
func (s *Store) clearNotice(reason NoticeReason) {
	current := s.attempt()
	if next := current.withoutNotice(reason); next != current {
		s.fresh.Store(next)
	}
}

// rebuilt is a copy of a with a new account of the blocked sources and the
// failed attempts behind them. The notices are kept: they are about the scan,
// not about what a build could read, and a build finishing does not retire
// them.
func (a *liveAttempt) rebuilt(blocked []BlockedSource, failedRetries int) *liveAttempt {
	return &liveAttempt{blocked: blocked, failedRetries: failedRetries, notices: a.notices}
}

// withNotice is a copy of a that also holds n.
func (a *liveAttempt) withNotice(n Notice) *liveAttempt {
	next := *a
	next.notices = append(slices.Clone(a.notices), n)
	return &next
}

// withoutNotice is a copy of a holding no notice of the given reason, or a
// itself when it held none.
func (a *liveAttempt) withoutNotice(reason NoticeReason) *liveAttempt {
	if !slices.ContainsFunc(a.notices, func(n Notice) bool { return n.Reason == reason }) {
		return a
	}
	next := *a
	next.notices = slices.DeleteFunc(slices.Clone(a.notices), func(n Notice) bool { return n.Reason == reason })
	return &next
}
