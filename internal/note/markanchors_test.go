package note_test

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReadingMarkAnchors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, source, id string
		accepted         bool
	}{
		{"heading", "## Accepted position\n", "accepted-position", true},
		{"block", "Paragraph. ^safe\n", "^safe", true},
		{"slash and query", "Paragraph. ^a/b?c\n", "^a/b?c", false},
		{"quoted block", "Paragraph. ^q\"x\n", "^q\"x", false},
		{"ampersand", "Paragraph. ^a&b\n", "^a&b", true},
		{"unicode boundary", "## " + strings.Repeat("漢", 85) + "a\n", strings.Repeat("漢", 85) + "a", true},
		{"over boundary", "## " + strings.Repeat("漢", 86) + "\n", strings.Repeat("漢", 86), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "anchors.md"), []byte(tt.source), 0o600); err != nil {
				t.Fatal(err)
			}
			srv := newServer(t, root)
			code, body := get(t, srv.Client(), srv.URL+"/notes/anchors.md")
			if code != 200 {
				t.Fatalf("GET reading page = %d, want200", code)
			}
			var found bool
			for _, tag := range regexp.MustCompile(`<[^>]+ id="([^"]*)"[^>]*>`).FindAllStringSubmatch(body, -1) {
				if html.UnescapeString(tag[1]) != tt.id {
					continue
				}
				found = true
				if got := strings.Contains(tag[0], " data-mark-anchor"); got != tt.accepted {
					t.Errorf("anchor %q eligible=%v, want%v", tt.id, got, tt.accepted)
				}
			}
			if !found {
				t.Fatalf("reading page lost original anchor %q", tt.id)
			}
		})
	}
}

func TestCompareMarksUseFinalQualifiedIDs(t *testing.T) {
	t.Parallel()
	id := strings.Repeat("a", 255)
	fixture := comparePairVault()
	for path, source := range fixture {
		if strings.HasSuffix(path, ".md") {
			fixture[path] = source + "\n## " + id + "\n"
		}
	}
	srv := newServerWithContract(t, writeNotes(t, fixture), loadHomeContract(t))
	code, body := get(t, srv.Client(), srv.URL+compareAddress)
	if code != 200 {
		t.Fatalf("GET compare = %d, want 200", code)
	}
	for _, prefix := range []string{"a-", "b-"} {
		var found bool
		for _, tag := range regexp.MustCompile(`<[^>]+ id="([^"]*)"[^>]*>`).FindAllStringSubmatch(body, -1) {
			if tag[1] != prefix+id {
				continue
			}
			found = true
			if strings.Contains(tag[0], " data-mark-anchor") {
				t.Errorf("%s heading over 256 bytes after qualification is eligible", prefix)
			}
		}
		if !found {
			t.Errorf("compare page lost original qualified heading %q", prefix+id)
		}
	}
}
