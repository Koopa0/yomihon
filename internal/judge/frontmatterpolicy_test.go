package judge

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// frontmatterContract is a vault of plain notes with one knowledge directory.
// The caller substitutes the scan declaration, which is the only thing under
// test here.
const frontmatterContract = `[folders]
checked = ["Notes"]
templates = []
skipped_files = []
SCAN_DECLARATION

[enums]

[fields]
common = ["type"]
required = ["type"]

[types.note]

[privacy]
never_egress_dirs = []
`

// inboxContract declares one required field set for every note and one more
// field a capture must carry, so the two sets cannot be satisfied by the same
// note.
const inboxContract = `[folders]
checked = ["Inbox", "Notes"]
templates = []

[enums]

[fields]
common = ["type", "created"]
required = ["type", "created"]

[types.inbox]
fields = ["level"]
required = ["level"]

[types.note]

[privacy]
never_egress_dirs = []
`

// TestCheckHonoursNoFrontmatterDeclaration asserts a vault that says a note
// without a frontmatter block is not legal gets told which notes have none,
// while a vault that says nothing, or says such a note is legal, is left alone.
func TestCheckHonoursNoFrontmatterDeclaration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		declaration string
		want        []string
	}{
		{name: "undeclared", declaration: ""},
		{name: "declared legal", declaration: "frontmatter_optional = true"},
		{name: "declared not legal", declaration: "frontmatter_optional = false", want: []string{"schema.frontmatter Notes/Raw.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath,
				strings.Replace(frontmatterContract, "SCAN_DECLARATION", tt.declaration, 1))
			write(t, root, "Notes/Raw.md", "just a transcript\nno fence\n")
			write(t, root, "Notes/Whole.md", "---\ntype: note\n---\n# Whole\n\nBody.\n")

			out, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunCheck() error = %v", err)
			}
			var got []string
			for _, f := range decodeFindings(t, out) {
				got = append(got, string(f.RuleID)+" "+f.Path)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("RunCheck() findings (-want +got):\n%s", diff)
			}
		})
	}
}

// TestCheckHonoursATypesOwnRequiredFields asserts a type's own required set is
// added to the one every note answers to, in both directions: a capture carries
// the general set like any other note and the field its type adds, and an
// ordinary note is not asked for the field only captures need.
func TestCheckHonoursATypesOwnRequiredFields(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, inboxContract)
	write(t, root, "Inbox/Complete.md", "---\ntype: inbox\ncreated: 2026-01-01\nlevel: first\n---\n")
	write(t, root, "Inbox/Bare.md", "---\ntype: inbox\n---\n")
	write(t, root, "Notes/Plain.md", "---\ntype: note\n---\n")

	out, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck() error = %v", err)
	}
	var got []string
	for _, f := range decodeFindings(t, out) {
		got = append(got, f.Path+" "+string(f.RuleID)+" "+*f.Field)
	}
	want := []string{
		"Inbox/Bare.md schema.required created",
		"Inbox/Bare.md schema.required level",
		"Notes/Plain.md schema.required created",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RunCheck() findings (-want +got):\n%s", diff)
	}
}
