package pages

import (
	"net/url"
	"strings"
	"testing"
)

func TestObsidianNewHrefNamesTheVaultAndFileOnly(t *testing.T) {
	t.Parallel()
	const root = "/Users/reader/A & B+vault"
	const name = "what? # 50%+日 的想法 2026-10-30 1523"
	const content = "---\nbased_on: \"[[a & b#章節]]\"\n---\n+ ? # % & = </textarea>\n"
	address := ObsidianNewHref(root, name, content)
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "obsidian" || parsed.Host != "new" || parsed.Path != "" || parsed.Fragment != "" {
		t.Errorf("new-note URI has unexpected action or fragment: %q", address)
	}
	query := parsed.Query()
	if len(query) != 3 || len(query["vault"]) != 1 || len(query["name"]) != 1 || len(query["content"]) != 1 {
		t.Fatalf("new-note URI must carry exactly vault, name and content, got %v", query)
	}
	if got := query.Get("vault"); got != "A & B+vault" {
		t.Errorf("vault = %q, want the vault folder's own name", got)
	}
	if got := query.Get("name"); got != name {
		t.Errorf("name = %q, want %q", got, name)
	}
	if got := query.Get("content"); got != content {
		t.Errorf("content = %q, want exact Markdown %q", got, content)
	}
	if strings.Contains(address, "/Users/reader") || strings.Contains(address, "%2FUsers") {
		t.Errorf("the machine's absolute path reached the URI: %q", address)
	}
	if strings.Contains(parsed.RawQuery, "+") {
		t.Error("spaces or literal plus signs are exposed as + in the editor URI")
	}
}

func TestObsidianNewHrefNeedsVaultAndName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ root, name string }{{}, {root: "/vault"}, {name: "thought"}} {
		if got := ObsidianNewHref(tc.root, tc.name, "content"); got != "" {
			t.Errorf("ObsidianNewHref(%q, %q) = %q, want no link", tc.root, tc.name, got)
		}
	}
}

func TestThoughtHrefSeparatesSourcePathAndSection(t *testing.T) {
	t.Parallel()
	const rel = "Notes/a?# &+日.md"
	const section = "a #?&+%日"
	parsed, err := url.Parse(thoughtHref(rel, section))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/thought/Notes/a?# &+日.md" || parsed.Fragment != "" {
		t.Errorf("thought URL lost its source path: %q", parsed.String())
	}
	query := parsed.Query()
	if len(query) != 1 || len(query["section"]) != 1 || query.Get("section") != section {
		t.Errorf("thought URL lost its section or injected another query: %v", query)
	}
	if got := thoughtHref("Source.md", ""); got != "/thought/Source.md" {
		t.Errorf("whole-source thought URL = %q", got)
	}
}
