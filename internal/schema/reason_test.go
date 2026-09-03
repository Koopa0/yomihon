package schema_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestARejectionReasonNamesItself pins the operator-facing words. They are not
// the sentence a reader sees — that one is chosen from the dictionary at the
// surface, in the language of whoever asked — and keeping the two apart is the
// reason this enum exists at all.
func TestARejectionReasonNamesItself(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		reason schema.Reason
		want   string
	}{
		{schema.ReasonUnstated, "unstated"},
		{schema.ReasonContractUnreadable, "contract-unreadable"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()

			if got := tc.reason.String(); got != tc.want {
				t.Errorf("Reason.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestARejectionReasonRefusesToNameAValueItDoesNotDeclare keeps a reason added
// to the block later from borrowing the words of one that is already there.
func TestARejectionReasonRefusesToNameAValueItDoesNotDeclare(t *testing.T) {
	t.Parallel()

	defer func() {
		recovered := recover()
		text, isText := recovered.(string)
		if !isText || !strings.Contains(text, "77") {
			t.Errorf("panic = %v, want a message naming the value 77", recovered)
		}
	}()
	_ = schema.Reason(77).String()
	t.Error("Reason(77).String() returned instead of panicking")
}
