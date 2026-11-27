package mark

import "time"

// SetClock gives k a clock, so that a test moves time on without waiting.
func (vm *VaultMarks) SetClock(now func() time.Time) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.now = now
}
