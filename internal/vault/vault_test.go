package vault_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/kurodo/internal/vault"
)

func writeNote(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestReadNote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		wantTitle  string
		wantStatus string
		wantSlug   string
		wantDiag   bool
		wantInBody string
	}{
		{
			name:       "lesson slug is read as the join key",
			content:    "---\ntitle: L01\ntype: lesson\nstatus: draft\nslug: jp-minna-l01\n---\n\nbody\n",
			wantTitle:  "L01",
			wantStatus: "draft",
			wantSlug:   "jp-minna-l01",
			wantInBody: "body",
		},
		{
			name:       "frontmatter and body",
			content:    "---\ntitle: 數量詞の位置\ntype: concept\nstatus: seedling\n---\n\n# 本文\n",
			wantTitle:  "數量詞の位置",
			wantStatus: "seedling",
			wantInBody: "# 本文",
		},
		{
			name:       "no frontmatter is legal",
			content:    "# 假名 quiz\n",
			wantTitle:  "note",
			wantInBody: "# 假名 quiz",
		},
		{
			name:       "broken yaml yields diagnostic not error",
			content:    "---\ntitle: [broken\n---\nbody\n",
			wantTitle:  "note",
			wantDiag:   true,
			wantInBody: "body",
		},
		{
			name:       "wikilink value survives the split",
			content:    "---\ntitle: t\nbased_on: \"[[大家的日本語 第11課]]\"\n---\nbody\n",
			wantTitle:  "t",
			wantInBody: "body",
		},
		{
			// The concept-resolver corruption guard, at the layer where the
			// guarantee lives (yomihon's TestResolverDoesNotCorruptFrontmatter):
			// a lesson's frontmatter based_on holds [[wikilinks]]; because the
			// split happens HERE, before any body preprocessing, a later concept
			// resolver rewriting [[...]] to a trigger can never reach the YAML and
			// empty the meta (which would drop status:ready and hide the lesson).
			// The status must survive intact alongside the wikilink-valued field.
			name:       "concept resolver cannot corrupt frontmatter status",
			content:    "---\ntitle: L00 テスト課\ntype: lesson\nstatus: ready\nbased_on: \"[[大家的日本語 第1課]]\"\nslug: jp-minna-l00\n---\n\nSee [[は]] and [[です]].\n",
			wantTitle:  "L00 テスト課",
			wantStatus: "ready",
			wantSlug:   "jp-minna-l00",
			wantInBody: "[[は]]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeNote(t, root, "Concepts/note.md", tt.content)

			n, err := vault.ReadNote(root, "Concepts/note.md")
			if err != nil {
				t.Fatalf("ReadNote() = %v", err)
			}
			if got := n.Title(); got != tt.wantTitle {
				t.Errorf("Title() = %q, want %q", got, tt.wantTitle)
			}
			if got := n.Status(); got != tt.wantStatus {
				t.Errorf("Status() = %q, want %q", got, tt.wantStatus)
			}
			if got := n.Slug(); got != tt.wantSlug {
				t.Errorf("Slug() = %q, want %q", got, tt.wantSlug)
			}
			if (n.FMDiagnostic != "") != tt.wantDiag {
				t.Errorf("FMDiagnostic = %q, want diagnostic: %v", n.FMDiagnostic, tt.wantDiag)
			}
			if !strings.Contains(n.Body, tt.wantInBody) {
				t.Errorf("Body does not contain %q:\n%s", tt.wantInBody, n.Body)
			}
		})
	}
}

func TestReadNoteRejectsEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if _, err := vault.ReadNote(root, "../outside.md"); err == nil {
		t.Error("ReadNote(../outside.md) = nil error, want path escape error")
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	writeNote(t, root, "Concepts/A.md", "a\n")
	writeNote(t, root, "Diagrams/x.canvas", "{}\n")
	writeNote(t, root, ".obsidian/workspace.json", "{}\n")
	writeNote(t, root, ".git/HEAD", "ref: refs/heads/main\n")

	got, err := vault.List(root)
	if err != nil {
		t.Fatalf("List(%q) = %v", root, err)
	}

	want := []string{"Concepts/A.md", "Diagrams/x.canvas"}
	slices.Sort(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("List(%q) mismatch (-want +got):\n%s", root, diff)
	}
}

// TestListNormalizesToNFC guards docs/vault-model.md:18 and
// docs/design.md's requirement that internal/vault NFC-normalize every
// walked path, mirroring kura's vault.rs::relative_key (.nfc().collect()
// at walk time). macOS filesystems can hold a filename as raw NFD bytes
// regardless of how it was typed or how the filesystem otherwise
// preserves names; a walker that hands back those bytes untouched would
// leak them into graph.Index's stored path values, rendered <a href>
// targets, and diagnostic candidate lists — silently diverging from
// kura's canonical NFC path representation.
func TestListNormalizesToNFC(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// だ (U+3060) decomposes to た (U+305F) + the combining voiced sound
	// mark (U+3099) — the real failure mode this vault hits in
	// Writing/lessons/japanese/, where dakuten kana are common in
	// filenames (docs/vault-model.md's 163-file lessons folder).
	const composed = "だ体.md"
	decomposed := norm.NFD.String(composed)
	if decomposed == composed {
		t.Fatalf("test setup invalid: NFD form of %q did not change", composed)
	}
	writeNote(t, root, decomposed, "body\n")

	got, err := vault.List(root)
	if err != nil {
		t.Fatalf("List(%q) = %v", root, err)
	}
	if len(got) != 1 {
		t.Fatalf("List(%q) = %v, want exactly one entry", root, got)
	}
	if got[0] != composed {
		t.Errorf("List(%q)[0] = %q (% x), want NFC-normalized %q (% x)",
			root, got[0], []byte(got[0]), composed, []byte(composed))
	}
}
