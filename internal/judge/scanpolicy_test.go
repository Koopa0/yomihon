package judge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// frontmatterContract is a vault of plain notes with one knowledge directory.
// The caller substitutes the scan declaration, which is the only thing under
// test here.
const frontmatterContract = `schema_version = "2"

[folders]
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
const inboxContract = `schema_version = "2"

[folders]
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
// A file that opens a block and never closes it is not that case. It has a
// block with no closing line, so every declaration is told so in words about
// the fence, and none is told the block is missing.
func TestCheckHonoursNoFrontmatterDeclaration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		declaration string
		wantFault   bool
	}{
		{name: "undeclared", declaration: "", wantFault: false},
		{name: "declared legal", declaration: "frontmatter_optional = true", wantFault: false},
		{name: "declared not legal", declaration: "frontmatter_optional = false", wantFault: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath,
				strings.Replace(frontmatterContract, "SCAN_DECLARATION", tt.declaration, 1))
			write(t, root, "Notes/Raw.md", "just a transcript\nno fence\n")
			write(t, root, "Notes/Unclosed.md", "---\ntype: note\n")
			write(t, root, "Notes/Whole.md", "---\ntype: note\n---\n# Whole\n\nBody.\n")

			got, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunCheck() error = %v", err)
			}
			const fault = `"rule_id":"schema.frontmatter","severity":"error","path":"`
			faulted := bytes.Contains(got, []byte(fault+`Notes/Raw.md","message":"frontmatter is missing"`))
			if faulted != tt.wantFault {
				t.Errorf("Notes/Raw.md faulted = %v, want %v; output:\n%s", faulted, tt.wantFault, got)
			}
			if !bytes.Contains(got, []byte(fault+`Notes/Unclosed.md","message":"frontmatter opens on line 1 and never closes"`)) {
				t.Errorf("Notes/Unclosed.md was not told its fence never closes; output:\n%s", got)
			}
			if bytes.Contains(got, []byte(fault+`Notes/Unclosed.md","message":"frontmatter is missing"`)) {
				t.Errorf("Notes/Unclosed.md was told its block is missing; output:\n%s", got)
			}
			if bytes.Contains(got, []byte("Notes/Whole.md")) {
				t.Errorf("a note carrying a frontmatter block was faulted:\n%s", got)
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

	got, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck() error = %v", err)
	}
	tests := []struct {
		name string
		want bool
		line string
	}{
		{
			name: "a capture carrying both sets is not faulted",
			want: false,
			line: `"path":"Inbox/Complete.md"`,
		},
		{
			name: "a capture still answers to the general set",
			want: true,
			line: `"path":"Inbox/Bare.md","field":"created","message":"created is required"`,
		},
		{
			name: "a capture missing the field its type requires is faulted",
			want: true,
			line: `"path":"Inbox/Bare.md","field":"level","message":"level is required"`,
		},
		{
			name: "an ordinary note still answers to the general set",
			want: true,
			line: `"path":"Notes/Plain.md","field":"created","message":"created is required"`,
		},
		{
			name: "an ordinary note is not asked for the field only captures need",
			want: false,
			line: `"path":"Notes/Plain.md","field":"level"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if found := bytes.Contains(got, []byte(tt.line)); found != tt.want {
				t.Errorf("contains(%s) = %v, want %v; output:\n%s", tt.line, found, tt.want, got)
			}
		})
	}
}
