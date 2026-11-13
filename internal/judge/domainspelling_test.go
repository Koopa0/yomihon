package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// The two domain words these rows judge with, each written twice: once with
// the precomposed code point an editor hands over, once with the bare letter
// and the combining voiced sound mark a Mac keyboard and a filesystem hand
// over. They are spelled as code points rather than typed, so the bytes are
// readable here and no editor can quietly fold them into each other.
const (
	composedDomain   = "\u304c\u3044\u306d\u3093"
	decomposedDomain = "\u304b\u3099\u3044\u306d\u3093"
	composedOther    = "\u304e\u3058\u3085\u3064"
	decomposedOther  = "\u304d\u3099\u3057\u3099\u3085\u3064"
	voicedMark       = '\u3099'
)

// TestDomainEnumFoldsTheFolderSpelling pins that a note's domain, read off the
// folder it sits in, is the word the contract declares in whichever of the two
// spellings the folder or the contract was written — while a folder naming a
// different word is still reported, so the agreement is about that one word
// and not about everything falling silent.
//
// A scan reports composed paths and the contract folds its own values as it
// loads, so both sides are canonical before the rule compares them. The finding
// carries the folder's composed name, which is the word everything downstream
// of the scan calls it.
//
// The rows drive the check command over a vault on disk rather than the linting
// seam, because the folder's spelling is the variable here and only a real scan
// settles what a directory written one way is called afterwards.
func TestDomainEnumFoldsTheFolderSpelling(t *testing.T) {
	t.Parallel()

	// These rows are about spelling only if the two spellings are different
	// bytes for one word. Both halves are checked: had the code points been
	// mistyped into one string, every row below would pass against an
	// implementation that folds nothing.
	for _, pair := range []struct {
		name                 string
		composed, decomposed string
	}{
		{"domain", composedDomain, decomposedDomain},
		{"other", composedOther, decomposedOther},
	} {
		if pair.composed == pair.decomposed {
			t.Fatalf("the %s fixture spells one word twice as the same bytes, so no row below varies anything", pair.name)
		}
		if got := vault.NormalizeNFC(pair.decomposed); got != pair.composed {
			t.Fatalf("the %s fixture holds two words rather than two spellings: NormalizeNFC(%q) = %q, want %q",
				pair.name, pair.decomposed, got, pair.composed)
		}
	}

	for _, tt := range []struct {
		name string
		// folder is the spelling handed to the filesystem; declared is the
		// spelling the contract's domain vocabulary is written in.
		folder   string
		declared string
		// wantMismatch marks a row whose folder names a different word, which
		// stays a finding in either spelling.
		wantMismatch bool
	}{
		{name: "composed folder composed contract", folder: composedDomain, declared: composedDomain},
		{name: "decomposed folder composed contract", folder: decomposedDomain, declared: composedDomain},
		{name: "composed folder decomposed contract", folder: composedDomain, declared: decomposedDomain},
		{name: "decomposed folder decomposed contract", folder: decomposedDomain, declared: decomposedDomain},
		{name: "other word composed folder", folder: composedOther, declared: composedDomain, wantMismatch: true},
		{name: "other word decomposed folder", folder: decomposedOther, declared: composedDomain, wantMismatch: true},
		{name: "other word decomposed folder, decomposed contract", folder: decomposedOther, declared: decomposedDomain, wantMismatch: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath, contractFixture(t, nil,
				[2]string{`domain = ["golang", "japanese", "meta"]`, `domain = ["golang", "japanese", "meta", "` + tt.declared + `"]`},
				[2]string{`checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, `checked = ["Concepts"]`},
			))
			rel := "Concepts/" + tt.folder + "/N.md"
			write(t, root, rel, "---\ntype: concept\ncreated: 2026-06-01\nsource_locator: book p.1\n---\n# N\n\nbody\n")

			// Which spelling a volume keeps is the volume's to decide, and one
			// that folds turns this row into its sibling rather than into
			// nothing. Naming the physical case keeps a green run readable.
			entries, err := os.ReadDir(filepath.Join(root, "Concepts"))
			if err != nil {
				t.Fatalf("read the domain root: %v", err)
			}
			if len(entries) != 1 {
				t.Fatalf("the domain root holds %d directories, want the one this row wrote", len(entries))
			}
			if stored := entries[0].Name(); stored != tt.folder {
				t.Logf("this volume stored the folder as %q rather than %q", stored, tt.folder)
			}

			findings, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			// A scan reports composed paths, so this is the note's name in
			// everything downstream of it, whichever spelling was written.
			notePath := vault.NormalizeNFC(rel)
			var got []Finding
			for _, f := range findings {
				if f.Path == notePath {
					got = append(got, f)
				}
			}

			var want []Finding
			if tt.wantMismatch {
				want = append(want, Finding{
					RuleID:          "schema.enum",
					Severity:        SeverityError,
					Path:            notePath,
					Field:           new("domain"),
					Message:         `domain "` + composedOther + `" is not an allowed value`,
					Evidence:        "frontmatter validated against vault-schema.toml",
					SuggestedAction: "fix the frontmatter to match the schema",
					SourceRule:      sourceContract,
					Target:          new(composedOther),
				})
			}
			if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(Finding{}, "Fingerprint")); diff != "" {
				t.Errorf("Check() findings on %s mismatch (-want +got):\n%s", rel, diff)
			}
			if tt.wantMismatch && len(got) == 1 && strings.ContainsRune(got[0].Message, voicedMark) {
				t.Errorf("Check() message %q carries a decomposed spelling, want the folder's composed name", got[0].Message)
			}
		})
	}
}
