package snapshot

import (
	"context"
	"time"
)

// Rescan performs one reconciliation attempt. Run is otherwise its only
// caller, and a reading surface built outside this package cannot be driven
// through a degraded generation without waiting out the real scan cadence.
func (s *Store) Rescan(ctx context.Context) { s.rescan(ctx) }

// SetClock replaces the clock the retry schedule reads, so a caller can stand
// at the next attempt rather than wait for it.
func (s *Store) SetClock(now func() time.Time) { s.now = now }

// DegradeAfter is how many incomplete attempts in a row publish a degraded
// generation, so a caller driving attempts can stop counting them by hand.
const DegradeAfter = degradeAfter
