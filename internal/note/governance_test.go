package note

import (
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestOneNoteGetsExactlyOneGovernanceAnswer pins the fact the reading page acts
// on before it dresses a lesson: a note is an instance, held outside the
// instances as an artifact, or beyond either authority's reach — never two of
// those and never none.
//
// A request whose contract or artifact authority has closed is neither, and
// the row that asserts it here is the one that used to render a governed page
// for a note nothing could vouch for. A note outside the declared knowledge
// layer is still an instance: the layer says where the judge looks, not what a
// note is.
func TestOneNoteGetsExactlyOneGovernanceAnswer(t *testing.T) {
	t.Parallel()

	contract, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}
	open := schema.CaptureAuthority(contract, contract.Governance())
	policy := contract.ArtifactPolicy()
	if !policy.Available() {
		t.Fatalf("the fixture contract declares an artifact policy; Available() = false")
	}

	tests := []struct {
		name         string
		authority    schema.Authority
		policy       schema.ArtifactPolicy
		relPath      string
		want         governance
		wantInstance bool
	}{
		{
			name:      "both authorities answer and the folder holds the note as an instance",
			authority: open, policy: policy, relPath: "Writing/lesson-01.md",
			want: governedInstance, wantInstance: true,
		},
		{
			name:      "both authorities answer and the folder holds the note as an artifact",
			authority: open, policy: policy, relPath: "System/templates/lesson.md",
			want: readableArtifact, wantInstance: false,
		},
		{
			name:      "both authorities answer and the declared layer does not reach the note",
			authority: open, policy: policy, relPath: "System/agent-guides/L05.md",
			want: governedInstance, wantInstance: true,
		},
		{
			name:      "the contract view is closed, so the note is placed nowhere",
			authority: schema.Authority{}, policy: policy, relPath: "Writing/lesson-01.md",
			want: governanceUnavailable, wantInstance: false,
		},
		{
			// The path is one the policy would have called an artifact if it
			// could be asked, so this row also says an unavailable policy is
			// not quietly read as "no artifact here".
			name:      "the artifact policy is unavailable, so the note is placed nowhere",
			authority: open, policy: schema.ArtifactPolicy{}, relPath: "System/templates/lesson.md",
			want: governanceUnavailable, wantInstance: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classifyGovernance(tt.authority, tt.policy, tt.relPath)
			if got != tt.want {
				t.Errorf("classifyGovernance(%q) = %d, want %d", tt.relPath, got, tt.want)
			}
			state := governanceState{placement: got}
			if state.instance() != tt.wantInstance {
				t.Errorf("instance() = %v, want %v", state.instance(), tt.wantInstance)
			}
		})
	}
}
