package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

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

// TestDomainMatchesItsFolderInEitherSpelling pins that a note naming the folder
// it sits in is read as naming it, in whichever of the two spellings its author
// wrote — while a note naming a different word is still reported, so the
// agreement is about that one word and not about everything falling silent.
//
// A scan reports composed paths, so the folder side is canonical before any
// rule sees it; the value side is the author's own bytes and is folded where it
// is compared. The finding keeps those bytes: a reader is shown what their file
// says, and the fingerprint keys on it.
//
// The rows drive the check command over a vault on disk rather than the linting
// seam, because the folder's spelling is the variable here and only a real scan
// settles what a directory written one way is called afterwards.
func TestDomainMatchesItsFolderInEitherSpelling(t *testing.T) {
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
		// folder is the spelling handed to the filesystem; value is the
		// spelling written on the note's domain line.
		folder string
		value  string
		// wantMismatch marks a row whose value names a different word, which
		// stays a finding in either spelling.
		wantMismatch bool
	}{
		{name: "composed folder composed value", folder: composedDomain, value: composedDomain},
		{name: "composed folder decomposed value", folder: composedDomain, value: decomposedDomain},
		{name: "decomposed folder composed value", folder: decomposedDomain, value: composedDomain},
		{name: "decomposed folder decomposed value", folder: decomposedDomain, value: decomposedDomain},
		{name: "composed folder other word composed", folder: composedDomain, value: composedOther, wantMismatch: true},
		{name: "composed folder other word decomposed", folder: composedDomain, value: decomposedOther, wantMismatch: true},
		{name: "decomposed folder other word composed", folder: decomposedDomain, value: composedOther, wantMismatch: true},
		{name: "decomposed folder other word decomposed", folder: decomposedDomain, value: decomposedOther, wantMismatch: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath, contractFixture(t, nil,
				[2]string{`domain = ["golang", "japanese", "meta"]`,
					`domain = ["golang", "japanese", "meta", "` + composedDomain + `", "` + composedOther + `"]`},
				[2]string{`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
					`knowledge_dirs = ["Concepts"]`},
			))
			rel := "Concepts/" + tt.folder + "/N.md"
			write(t, root, rel, "---\ntitle: N\ntype: concept\ndomain: "+tt.value+
				"\nstatus: seedling\ncreated: 2026-06-01\nupdated: 2026-06-01\nsource_locator: book p.1\n---\n\nbody\n")

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
					RuleID:          "schema.domain_folder",
					Severity:        SeverityError,
					Path:            notePath,
					Field:           new("domain"),
					Message:         `domain "` + tt.value + `" does not match its folder ` + composedDomain,
					Evidence:        "frontmatter validated against vault-schema.toml",
					SuggestedAction: "fix the frontmatter to match the schema",
					SourceRule:      sourceContractRules,
					Target:          new(tt.value),
					Fingerprint:     fingerprint("schema.domain_folder", notePath, "domain\x1f"+tt.value),
				})
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Check() findings on %s mismatch (-want +got):\n%s", rel, diff)
			}

			// Named separately from the comparison above, because it is the
			// property the fold is most likely to cost: a value folded on its
			// way into the evidence reads as the author's word while being
			// different bytes, and moves the fingerprint off the file.
			if !strings.ContainsRune(tt.value, voicedMark) || len(got) != 1 {
				return
			}
			switch target := got[0].Target; {
			case target == nil:
				t.Errorf("Check() named no target, want the bytes the file wrote, %q", tt.value)
			case *target != tt.value:
				t.Errorf("Check() reported target %q, want the bytes the file wrote, %q", *target, tt.value)
			}
			if !strings.Contains(got[0].Message, tt.value) {
				t.Errorf("Check() message %q does not quote the bytes the file wrote, %q", got[0].Message, tt.value)
			}
		})
	}
}
