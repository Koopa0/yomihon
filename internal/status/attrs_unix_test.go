//go:build linux || darwin

package status

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestXattrIgnorableCoversListFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "ENOTSUP", err: unix.ENOTSUP, want: true},
		{name: "EOPNOTSUPP", err: unix.EOPNOTSUPP, want: true},
		{name: "ENOSYS", err: unix.ENOSYS, want: true},
		{name: "EPERM", err: unix.EPERM, want: true},
		{name: "EACCES", err: unix.EACCES, want: true},
		{name: "EIO", err: unix.EIO, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := xattrIgnorable(tt.err); got != tt.want {
				t.Errorf("xattrIgnorable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
