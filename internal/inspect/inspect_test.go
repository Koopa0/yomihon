package inspect_test

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/inspect"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
)

// openVault reads a folder the way the command does: the gate first, then one
// generation under the rules file the gate read.
func openVault(t *testing.T, root string) (*snapshot.Generation, *egress.Gate) {
	t.Helper()
	source, err := vault.Open(root)
	if err != nil {
		t.Fatalf("open the folder: %v", err)
	}
	t.Cleanup(func() {
		if err := source.Close(); err != nil {
			t.Errorf("close the folder: %v", err)
		}
	})
	gate, err := egress.Open(t.Context(), source, judge.LinkTargets)
	if err != nil {
		t.Fatalf("open the gate: %v", err)
	}
	rules := gate.Rules()
	source.SetBoundary(rules.ScanBoundary())
	store, err := snapshot.New(t.Context(), source, slog.New(slog.DiscardHandler), rules, rules.Governance())
	if err != nil {
		t.Fatalf("read the folder: %v", err)
	}
	return store.Current().Capture(), gate
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("make the folder of %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil { // #nosec G703 -- test fixture paths are owned by each caller's temporary root
		t.Fatalf("write %s: %v", rel, err)
	}
}

// guarded is a folder with a rules file that withholds Private, and notes that
// point at, and are pointed at from, a note in it.
func guarded(t *testing.T) (*snapshot.Generation, *egress.Gate) {
	t.Helper()
	root := t.TempDir()
	rules, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read the rules fixture: %v", err)
	}
	writeFile(t, root, "System/schemas/vault-schema.toml", string(rules)+"\n[privacy]\nnever_egress_dirs = [\"Private\"]\n")
	writeFile(t, root, "Notes/Plain.md", "# Plain\n\nsee [[Secret Plan]] and [[Other]]\n\n## Plans for [[Secret Plan]]\n\n## Open\n")
	writeFile(t, root, "Notes/Other.md", "# Other\n\n[[Plain]]\n")
	writeFile(t, root, "Notes/Cites.md", "---\nbased_on: [\"[[Secret Plan]]\", \"[[Other]]\"]\n---\n# Cites\n\n[[Plain]]\n")
	writeFile(t, root, "Private/Secret Plan.md", "# Secret Plan\n\nback to [[Plain]]\n\n[[Cites]]\n")
	writeFile(t, root, "Private/Another.md", "# Another\n")
	return openVault(t, root)
}

func encode(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The example vault withholds its Diary, and has one note in it.
func TestAVaultCountsNoNoteItCannotName(t *testing.T) {
	t.Parallel()

	gen, gate := openVault(t, filepath.Join("..", "..", "examples", "vault"))
	everything := inspect.ForReader(gen).Vault()
	allowed := inspect.ForAgent(gen, gate).Vault()
	if everything.Notes != gen.NoteCount() {
		t.Errorf("ForReader counts %d notes, the generation has %d", everything.Notes, gen.NoteCount())
	}
	if allowed.Notes != everything.Notes-1 {
		t.Errorf("ForAgent counts %d notes, want one fewer than the %d the reader has: the one under Diary", allowed.Notes, everything.Notes)
	}
	if allowed.NoRules || everything.NoRules {
		t.Errorf("a vault with a rules file says it has none: %+v %+v", allowed, everything)
	}
}

func TestAVaultNamesOnlyTheCoursesAndIndexesItCanName(t *testing.T) {
	t.Parallel()

	gen, gate := openVault(t, filepath.Join("..", "..", "examples", "vault"))
	everything := inspect.ForReader(gen).Vault()
	if len(everything.Indexes) == 0 || len(everything.Courses) == 0 {
		t.Fatalf("the example vault shows %d indexes and %d courses, want some of each: %+v", len(everything.Indexes), len(everything.Courses), everything)
	}
	allowed := inspect.ForAgent(gen, gate).Vault()
	readable := gate.Readable(gen)
	for _, entry := range append(allowed.Courses, allowed.Indexes...) {
		if !readable(entry.Path) {
			t.Errorf("ForAgent names %q, which the gate does not let out", entry.Path)
		}
	}
	if strings.Contains(encode(t, allowed), "Diary") {
		t.Errorf("ForAgent says Diary: %s", encode(t, allowed))
	}
}

// What a path comes to is the same for a withheld note and for a withheld path
// where no note is, so asking cannot tell whether one is there.
func TestAWithheldPathIsAnsweredAlikeWhetherOrNotANoteIsThere(t *testing.T) {
	t.Parallel()

	gen, gate := openVault(t, filepath.Join("..", "..", "examples", "vault"))
	agent := inspect.ForAgent(gen, gate)
	for _, tt := range []struct {
		name string
		path string
		want inspect.Outcome
	}{
		{"a visible note", "Notes/Privacy and egress.md", inspect.Found},
		{"a visible path that names no note", "Notes/Nothing here.md", inspect.Missing},
		{"a withheld note", "Diary/2026-09-22.md", inspect.Withheld},
		{"a withheld path with no note", "Diary/1999-01-01.md", inspect.Withheld},
		{"a withheld folder", "Diary", inspect.Withheld},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if note, got := agent.Note(tt.path); got != tt.want {
				t.Errorf("Note(%q) = %v, want %v", tt.path, got, tt.want)
			} else if got != inspect.Found && encode(t, note) != encode(t, inspect.Note{}) {
				t.Errorf("Note(%q) answered %s with no note", tt.path, encode(t, note))
			}
		})
	}
	if _, got := inspect.ForReader(gen).Note("Diary/2026-09-22.md"); got != inspect.Found {
		t.Errorf("ForReader.Note(Diary/2026-09-22.md) = %v, want the reader's own page to say it", got)
	}
}

func TestANoteLeavesOutWhatNamesWithheldGround(t *testing.T) {
	t.Parallel()

	gen, gate := guarded(t)
	agent, reader := inspect.ForAgent(gen, gate), inspect.ForReader(gen)

	plain, outcome := agent.Note("Notes/Plain.md")
	if outcome != inspect.Found {
		t.Fatalf("Note(Notes/Plain.md) = %v", outcome)
	}
	if len(plain.LinksOut) != 1 || plain.LinksOut[0].Path != "Notes/Other.md" {
		t.Errorf("LinksOut = %+v, want only Notes/Other.md", plain.LinksOut)
	}
	if len(plain.Sections) != 2 || plain.Sections[0].Text != "Plain" || plain.Sections[1].Text != "Open" {
		t.Errorf("Sections = %+v, want the headings that name no withheld note", plain.Sections)
	}
	if plain.Title != "Plain" || plain.TitleFrom != inspect.FromHeading {
		t.Errorf("Title = %q from %q, want Plain from its heading", plain.Title, plain.TitleFrom)
	}
	for _, finding := range plain.Findings {
		if strings.Contains(finding.Target, "Secret") {
			t.Errorf("Findings = %+v, want none that names a withheld note", plain.Findings)
		}
	}

	cites, _ := agent.Note("Notes/Cites.md")
	if len(cites.BasedOn) != 1 || cites.BasedOn[0].Path != "Notes/Other.md" {
		t.Errorf("BasedOn = %+v, want only Notes/Other.md", cites.BasedOn)
	}
	other, _ := agent.Note("Notes/Other.md")
	for _, link := range append(other.LinksIn, other.BasedOnBy...) {
		if strings.HasPrefix(link.Path, "Private/") {
			t.Errorf("a note links in from %q, which is withheld", link.Path)
		}
	}
	if got := encode(t, plain) + encode(t, cites) + encode(t, other); strings.Contains(got, "Secret") || strings.Contains(got, "Private") {
		t.Errorf("a note says what is withheld: %s", got)
	}

	// The reader's own view keeps all of it.
	whole, _ := reader.Note("Notes/Plain.md")
	if len(whole.LinksOut) != 2 || len(whole.Sections) != 3 {
		t.Errorf("ForReader: %d links out and %d sections, want 2 and 3: %+v", len(whole.LinksOut), len(whole.Sections), whole)
	}
	wholeCites, _ := reader.Note("Notes/Cites.md")
	if len(wholeCites.BasedOn) != 2 {
		t.Errorf("ForReader BasedOn = %+v, want both", wholeCites.BasedOn)
	}
	if secret, outcome := reader.Note("Private/Secret Plan.md"); outcome != inspect.Found || len(secret.LinksIn) != 1 {
		t.Errorf("ForReader on the withheld note = %v with links in %+v", outcome, secret.LinksIn)
	}
}

func TestAVaultWithNoRulesFileSaysSo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "Notes/A.md", "# A\n")
	gen, gate := openVault(t, root)
	if got := inspect.ForAgent(gen, gate).Vault(); !got.NoRules || got.Notes != 1 {
		t.Errorf("Vault() = %+v, want a vault of one note with no rules file", got)
	}
}
