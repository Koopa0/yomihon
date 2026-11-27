package mark

import "time"

// SetClock gives k a clock, so that a test moves time on without waiting.
func (k *Kept) SetClock(now func() time.Time) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.now = now
}
