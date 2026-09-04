package status

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestTheRenameRungLooksAtWhatItInstalled covers the last rung's one remaining
// promise. It cannot keep the version it replaces — the rename destroys that
// name before anything can read it, which is why the two rungs above it exist —
// but it can look at what the note carries once the rename returns, and a flip
// that reports success while the file holds somebody else's bytes is a page
// telling a reader something the disk does not say.
//
// The wording is checked too, because this rung must not borrow the sentence
// the rungs above it earn: those put the other program's bytes back, and this
// one has no copy to put back.
func TestTheRenameRungLooksAtWhatItInstalled(t *testing.T) {
	t.Parallel()

	installed := []byte("---\nstatus: ready\n---\nbody\n")
	foreign := []byte("---\nstatus: ready\n---\nsomebody else was here\n")

	tests := []struct {
		name    string
		back    []byte
		readErr error
		wantErr error
	}{
		{name: "the note carries what was installed", back: installed},
		{name: "the note carries somebody else's bytes", back: foreign, wantErr: ErrConcurrentWrite},
		{name: "the note cannot be read back", readErr: errors.New("permission denied"), wantErr: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			renamed := false
			ops := installOps{
				rename: func(string, string) error { renamed = true; return nil },
				read: func(string) ([]byte, error) {
					if tt.readErr != nil {
						return nil, tt.readErr
					}
					return tt.back, nil
				},
				remove: func(string) error { return nil },
			}
			err := installByRename(ops, "Writing/L01.md", "tmp", &fileSnapshot{name: "L01.md", data: []byte("older")}, installed)
			if !renamed {
				t.Fatal("the rung returned without renaming, so nothing below is about an install")
			}
			switch {
			case tt.readErr != nil:
				if err == nil || errors.Is(err, ErrConcurrentWrite) {
					t.Errorf("a read-back failure answered %v; it is neither success nor a concurrent write", err)
				}
			case tt.wantErr == nil:
				if err != nil {
					t.Errorf("installing bytes the note then carries answered %v, want success", err)
				}
			default:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("answered %v, want %v", err, tt.wantErr)
				}
				if got := fmt.Sprint(err); !containsAll(got, "no copy", "put back") {
					t.Errorf("the message is %q; it must say this rung restored nothing", got)
				}
			}
		})
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
