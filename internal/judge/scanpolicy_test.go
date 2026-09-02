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
const frontmatterContract = `schema_version = "1"

[enums]
type = ["note"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status"]

[scan]
knowledge_dirs = ["Notes"]
skip_basenames = []
SCAN_DECLARATION

[navigation]
path_types = []
map_types = []

[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
`

// inboxContract declares one required field set for ordinary notes and a
// different one for undefined-shape captures, so the two cannot be satisfied by
// the same note.
const inboxContract = `schema_version = "1"

[enums]
type = ["inbox", "note"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type", "status"]
required_inbox = ["title", "created"]
known = ["title", "type", "status", "created"]

[scan]
knowledge_dirs = ["Inbox", "Notes"]
skip_basenames = []

[navigation]
path_types = []
map_types = []

[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
`

// TestCheckHonoursNoFrontmatterDeclaration asserts a vault that says a note
// without a frontmatter block is not legal gets told which notes have none,
// while a vault that says nothing, or says such a note is legal, is left alone.
// A file that opens a block and never closes it carries no readable block
// either, so it is faulted the same way.
func TestCheckHonoursNoFrontmatterDeclaration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		declaration string
		wantFault   bool
	}{
		{name: "undeclared", declaration: "", wantFault: false},
		{name: "declared legal", declaration: "no_frontmatter_is_legal = true", wantFault: false},
		{name: "declared not legal", declaration: "no_frontmatter_is_legal = false", wantFault: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath,
				strings.Replace(frontmatterContract, "SCAN_DECLARATION", tt.declaration, 1))
			write(t, root, "Notes/Raw.md", "just a transcript\nno fence\n")
			write(t, root, "Notes/Unclosed.md", "---\ntitle: Unclosed\ntype: note\n")
			write(t, root, "Notes/Whole.md", "---\ntitle: Whole\ntype: note\n---\n\nBody.\n")

			got, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunCheck() error = %v", err)
			}
			for _, path := range []string{"Notes/Raw.md", "Notes/Unclosed.md"} {
				faulted := bytes.Contains(got, []byte(`"rule_id":"schema.frontmatter","severity":"error","path":"`+path+
					`","message":"frontmatter is missing"`))
				if faulted != tt.wantFault {
					t.Errorf("%s faulted = %v, want %v; output:\n%s", path, faulted, tt.wantFault, got)
				}
			}
			if bytes.Contains(got, []byte("Notes/Whole.md")) {
				t.Errorf("a note carrying a frontmatter block was faulted:\n%s", got)
			}
		})
	}
}

// TestCheckHoldsCapturesToTheGeneralSetWhenNoInboxSetIsDeclared asserts a vault
// that never declared what a capture must carry keeps holding one to the same
// fields as any other note, with the domain waiver it has always had. Silence
// about captures is not a licence to require nothing of them.
func TestCheckHoldsCapturesToTheGeneralSetWhenNoInboxSetIsDeclared(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath,
		strings.Replace(inboxContract, "required_inbox = [\"title\", \"created\"]\n", "", 1))
	write(t, root, "Inbox/Bare.md", "---\ntitle: Bare\ntype: inbox\n---\n")

	got, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck() error = %v", err)
	}
	if !bytes.Contains(got, []byte(`"path":"Inbox/Bare.md","field":"status","message":"status is required"`)) {
		t.Errorf("a capture escaped the general required set:\n%s", got)
	}
}

// TestCheckDemandsDomainWhenTheInboxSetNamesIt asserts a contract that lists
// domain among the fields a capture must carry gets it demanded. The waiver
// that spares a capture from being classified by knowledge domain belongs to a
// vault that said nothing about captures; it does not survive a vault that
// spelled the set out and put domain in it.
func TestCheckDemandsDomainWhenTheInboxSetNamesIt(t *testing.T) {
	t.Parallel()

	contract := strings.Replace(inboxContract,
		`required_inbox = ["title", "created"]`, `required_inbox = ["title", "domain"]`, 1)
	contract = strings.Replace(contract,
		`known = ["title", "type", "status", "created"]`,
		`known = ["title", "type", "status", "created", "domain"]`, 1)

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contract)
	write(t, root, "Inbox/Bare.md", "---\ntitle: Bare\ntype: inbox\n---\n")

	got, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck() error = %v", err)
	}
	if !bytes.Contains(got, []byte(`"path":"Inbox/Bare.md","field":"domain","message":"domain is required"`)) {
		t.Errorf("the declared inbox set asked for domain and it was waived away:\n%s", got)
	}
}

// TestCheckHonoursRequiredInbox asserts the contract's inbox required-field set
// governs an undefined-shape capture, in both directions: a field the general
// set requires is not demanded of a capture, and a field the inbox set requires
// is.
func TestCheckHonoursRequiredInbox(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, inboxContract)
	write(t, root, "Inbox/Complete.md", "---\ntitle: Complete\ntype: inbox\ncreated: 2026-01-01\n---\n")
	write(t, root, "Inbox/Bare.md", "---\ntitle: Bare\ntype: inbox\n---\n")
	write(t, root, "Notes/Plain.md", "---\ntitle: Plain\ntype: note\n---\n")

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
			name: "a capture carrying the inbox set is not faulted",
			want: false,
			line: `"path":"Inbox/Complete.md"`,
		},
		{
			name: "a capture is not asked for a field only ordinary notes need",
			want: false,
			line: `"path":"Inbox/Bare.md","field":"status"`,
		},
		{
			name: "a capture missing a field the inbox set requires is faulted",
			want: true,
			line: `"path":"Inbox/Bare.md","field":"created","message":"created is required"`,
		},
		{
			name: "an ordinary note still answers to the general set",
			want: true,
			line: `"path":"Notes/Plain.md","field":"status","message":"status is required"`,
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
