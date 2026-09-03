package note

import (
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
)

// TestOneNoteGetsExactlyOneGovernanceAnswer pins the fact the reading page acts
// on before it draws a status face: a note is governed, withheld by the declared
// knowledge layer, held outside the lifecycle, or beyond either authority's
// reach — never two of those and never none.
//
// The derived answers are checked together with the placement itself, because
// the page reads them and not the placement, and because their relationship is
// the whole point: nonInstance is not the negation of instance, and a note the
// layer withheld is still one. A request whose lifecycle or artifact authority
// has closed is none of them, and the row that asserts it here is the one that
// used to render a governed page for a note nothing could vouch for.
func TestOneNoteGetsExactlyOneGovernanceAnswer(t *testing.T) {
	t.Parallel()

	contract, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}
	open := homeStatusView(t, contract, contract.Governance())
	policy := contract.ArtifactPolicy()
	if !policy.Available() {
		t.Fatalf("the fixture contract declares an artifact policy; Available() = false")
	}

	tests := []struct {
		name             string
		lifecycle        status.Authority
		policy           schema.ArtifactPolicy
		relPath          string
		want             governance
		wantInstance     bool
		wantNonInstance  bool
		wantOutsideLayer bool
	}{
		{
			name:      "both authorities answer and the folder governs the note",
			lifecycle: open, policy: policy, relPath: "Writing/lesson-01.md",
			want: governedInstance, wantInstance: true, wantNonInstance: false,
		},
		{
			name:      "both authorities answer and the folder holds the note outside its lifecycle",
			lifecycle: open, policy: policy, relPath: "System/templates/lesson.md",
			want: readableArtifact, wantInstance: false, wantNonInstance: true,
		},
		{
			// The fixture contract declares Writing alone as its knowledge
			// layer, so this note is one the state machine never reaches. It
			// stays an instance: the layer says where the lifecycle runs, not
			// what a note is, and the page still reads its status.
			name:      "both authorities answer and the declared layer does not reach the note",
			lifecycle: open, policy: policy, relPath: "System/agent-guides/L05.md",
			want: outsideKnowledgeLayer, wantInstance: true, wantNonInstance: false, wantOutsideLayer: true,
		},
		{
			name:      "the lifecycle view is closed, so the note is placed nowhere",
			lifecycle: status.Authority{}, policy: policy, relPath: "Writing/lesson-01.md",
			want: governanceUnavailable, wantInstance: false, wantNonInstance: false,
		},
		{
			// The path is one the policy would have called an artifact if it
			// could be asked, so this row also says an unavailable policy is
			// not quietly read as "no artifact here".
			name:      "the artifact policy is unavailable, so the note is placed nowhere",
			lifecycle: open, policy: schema.ArtifactPolicy{}, relPath: "System/templates/lesson.md",
			want: governanceUnavailable, wantInstance: false, wantNonInstance: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classifyGovernance(tt.lifecycle, tt.policy, tt.relPath)
			if got != tt.want {
				t.Errorf("classifyGovernance(%q) = %d, want %d", tt.relPath, got, tt.want)
			}
			state := governanceState{placement: got}
			if state.instance() != tt.wantInstance {
				t.Errorf("instance() = %v, want %v", state.instance(), tt.wantInstance)
			}
			if state.nonInstance() != tt.wantNonInstance {
				t.Errorf("nonInstance() = %v, want %v", state.nonInstance(), tt.wantNonInstance)
			}
			if state.outsideLayer() != tt.wantOutsideLayer {
				t.Errorf("outsideLayer() = %v, want %v", state.outsideLayer(), tt.wantOutsideLayer)
			}
		})
	}
}
