package note_test

import (
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestThePageOffersOneStepOnwardInTheFoot holds one reading page to one order
// and one place to take it. Where several study paths teach a note, the foot
// walks the one whose domain the note shares; where nothing picks a course out,
// it keeps the folder's alphabetical neighbour. Either way the step onward is
// offered once, under the prose, and the rail beside it carries no second copy.
func TestThePageOffersOneStepOnwardInTheFoot(t *testing.T) {
	t.Parallel()

	const (
		current     = "/notes/Writing/lessons/golang/Current.md"
		folderNext  = "/notes/Writing/lessons/golang/Folder%20next.md"
		primaryNext = "/notes/Writing/lessons/golang/Primary%20next.md"
	)
	body := func(title string) string {
		return "---\ntype: lesson\n---\n# " + title + "\n\nbody\n"
	}
	// The folder sorts Current, Folder next, Other next, Primary next, so the
	// file beside this one is never the lesson any course teaches next.
	notes := map[string]string{
		"Writing/lessons/golang/Current.md":      body("Current"),
		"Writing/lessons/golang/Folder next.md":  body("Folder next"),
		"Writing/lessons/golang/Other next.md":   body("Other next"),
		"Writing/lessons/golang/Primary next.md": body("Primary next"),
	}
	// A course's domain is the folder it sits in, as a lesson's is.
	syllabus := func(title, second string) string {
		return "---\ntype: study-path\n---\n# " + title + "\n\n" +
			"## Part {sequence=primary}\n\n- [[Current]]\n- [[" + second + "]]\n"
	}

	tests := []struct {
		name string
		// wantKind is the map the rail carries, named so that an absent rail
		// step cannot stand in for a rail that never showed a book.
		courses  map[string]string
		wantKind string
		wantFoot string
	}{
		{
			name: "two courses teach it and one of them is its own",
			courses: map[string]string{
				"Maps/golang/Primary path.md":     syllabus("Primary path", "Primary next"),
				"Maps/japanese/Secondary path.md": syllabus("Secondary path", "Other next"),
			},
			wantKind: "book",
			wantFoot: primaryNext,
		},
		{
			name: "one course teaches it",
			courses: map[string]string{
				"Maps/golang/Primary path.md": syllabus("Primary path", "Primary next"),
			},
			wantKind: "book",
			wantFoot: primaryNext,
		},
		{
			// Two courses and neither is this note's own: nothing here knows
			// which one the reader is walking, so the rail carries the folder
			// and the foot has to walk it too.
			name: "two courses teach it and neither is its own",
			courses: map[string]string{
				"Maps/japanese/Primary path.md": syllabus("Primary path", "Primary next"),
				"Maps/meta/Secondary path.md":   syllabus("Secondary path", "Other next"),
			},
			wantKind: "folder",
			wantFoot: folderNext,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			files := make(map[string]string, len(notes)+len(tt.courses))
			maps.Copy(files, notes)
			maps.Copy(files, tt.courses)
			srv := newServerWithContract(t, writeNotes(t, files), loadHomeContractWithDomainRoots(t, `["Writing/lessons", "Maps"]`))

			code, page := get(t, srv.Client(), srv.URL+current)
			if code != http.StatusOK {
				t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
			}
			if kind := `data-reading-rail="` + tt.wantKind + `"`; !strings.Contains(page, kind) {
				t.Fatalf("the page carries no %s rail, so its step onward proves nothing", tt.wantKind)
			}
			if strings.Contains(page, `class="y-lessonsteps"`) {
				t.Error("the rail offers a step onward as well as the foot; one page offers one")
			}
			if foot := footOnwardHref(t, page); foot != tt.wantFoot {
				t.Errorf("the foot offers %q as the step onward, want %q", foot, tt.wantFoot)
			}
		})
	}
}

// footOnwardHref is the address the foot of the article offers as the step
// after this note.
func footOnwardHref(t *testing.T, page string) string {
	t.Helper()
	foot := stepsBlock(t, page)
	at := strings.Index(foot, "y-steps__link--next")
	if at < 0 {
		return ""
	}
	return linkHref(t, foot, at)
}

// linkHref reads the address of the link whose text or class carries the mark
// at the given offset.
func linkHref(t *testing.T, markup string, at int) string {
	t.Helper()
	open := strings.LastIndex(markup[:at], "<a ")
	if open < 0 {
		t.Fatalf("the step onward is not a link; markup = %q", markup)
	}
	const marker = `href="`
	href := strings.Index(markup[open:], marker)
	if href < 0 {
		t.Fatalf("the step onward carries no address; markup = %q", markup)
	}
	value := markup[open+href+len(marker):]
	end := strings.IndexByte(value, '"')
	if end < 0 {
		t.Fatalf("the step onward has an unterminated address; markup = %q", markup)
	}
	return value[:end]
}

// loadHomeContractWithDomainRoots is the shared contract with its domain roots
// replaced by roots, a TOML list.
func loadHomeContractWithDomainRoots(t *testing.T, roots string) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema test contract: %v", err)
	}
	const declaration = `domain_from = ["Concepts"]`
	if strings.Count(string(data), declaration) != 1 {
		t.Fatal("the contract fixture does not declare its domain roots once")
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(strings.Replace(string(data), declaration, "domain_from = "+roots, 1)), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under this test's TempDir
		t.Fatalf("write contract: %v", writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	return contract
}
