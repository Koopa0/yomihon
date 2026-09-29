package pages

import (
	"net/url"
	"strings"
	"testing"
)

func TestObsidianNewHrefCreatesWithoutAppendOrOverwrite(t *testing.T) {
	t.Parallel()
	const root = "/Users/reader/A & B+vault"
	const rel = "Notes/what?# 50%+日.md"
	const content = "---\nbased_on: \"[[a & b#章節]]\"\n---\n+ ? # % & = </textarea>\n"
	address := ObsidianNewHref(root, rel, content)
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "obsidian" || parsed.Host != "new" || parsed.Path != "" || parsed.Fragment != "" {
		t.Errorf("new-note URI has unexpected action or fragment: %q", address)
	}
	query := parsed.Query()
	if len(query) != 2 || len(query["path"]) != 1 || len(query["content"]) != 1 {
		t.Fatalf("new-note URI must carry exactly path and content, got %v", query)
	}
	if got := query.Get("path"); got != "/Users/reader/A & B+vault/Notes/what?# 50%+日.md" {
		t.Errorf("path = %q, want the complete absolute file path", got)
	}
	if got := query.Get("content"); got != content {
		t.Errorf("content = %q, want exact Markdown %q", got, content)
	}
	if strings.Contains(parsed.RawQuery, "+") {
		t.Error("spaces or literal plus signs are exposed as + in the editor URI")
	}
}

func TestObsidianNewHrefNeedsVaultAndDestination(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ root, rel string }{{}, {root: "/vault"}, {rel: "thought.md"}} {
		if got := ObsidianNewHref(tc.root, tc.rel, "content"); got != "" {
			t.Errorf("ObsidianNewHref(%q, %q) = %q, want no link", tc.root, tc.rel, got)
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
