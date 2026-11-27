package judge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestMarkdownCheckReportsTheWrittenPath holds a Markdown link's finding to
// the path as its author wrote it, still encoded.
func TestMarkdownCheckReportsTheWrittenPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Concepts/golang/Source.md", "intro\n\n[missing](Nothing%20here.md)\n")
	writeTestContract(t, root, nil)
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if diff := cmp.Diff([]string{"link.broken.path Nothing%20here.md"}, findingKeys(findings)); diff != "" {
		t.Errorf("Check() findings (-want +got):\n%s", diff)
	}
}

func TestMarkdownCheckIsRelativeAndExact(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, destination, want string }{
		{"root spelling remains relative", "Areas/Other%20note.md", "Concepts/golang/Areas/Other note.md does not exist in the vault"},
		{"basename is not rescued", "Other%20note.md", "Concepts/golang/Other note.md does not exist in the vault"},
		{"explicit relative", "../../Areas/Other%20note.md", ""},
		{"unrelated public and private claimants", "./Other.md", ""},
		{"wrong case", "other.md", "Concepts/golang/other.md does not exist in the vault"},
		{"NFC equivalent", "cafe%CC%81.md", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, "Concepts/golang/Source.md", "[link]("+tt.destination+")\n")
			for _, p := range []string{"Areas/Other note.md", "Areas/Other.md", "Concepts/golang/Other.md", "Restricted/Other.md", "Concepts/golang/café.md"} {
				write(t, root, p, "target\n")
			}
			writeTestContract(t, root, []string{"Restricted"})
			findings, err := runCheckAction(t.Context(), root, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, f := range findings {
				if f.RuleID == "link.broken.path" {
					got = append(got, f.Evidence)
				}
			}
			var want []string
			if tt.want != "" {
				want = []string{tt.want}
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("relative Markdown verdict (-want +got):\n%s", diff)
			}
		})
	}
}

// TestMarkdownLinkToAFolderIsNotAFinding holds that a link to a directory is
// outside the rule's domain, which is Markdown note paths: a folder link
// routes to the folder's page, so it raises no finding whether it is written
// with or without the trailing slash.
func TestMarkdownLinkToAFolderIsNotAFinding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Concepts/golang/Source.md", "[a](lab/)\n[b](lab)\n[c](../golang/lab/code/)\n[d](missing/)\n")
	write(t, root, "Concepts/golang/lab/code/main.go", "package main\n")
	writeTestContract(t, root, nil)
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, f := range findings {
		if strings.HasPrefix(string(f.RuleID), "link.broken") {
			t.Errorf("folder link raised a finding: %+v", f)
		}
	}
}

// A link into what the scan left out by the vault's .gitignore or a built-in
// rule is a link into somewhere this reader does not look, not one to a file
// somebody lost, so it raises no finding; a link to a path nobody wrote still
// does.
func TestLinkIntoWhatTheScanLeftOutIsNotAFinding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, ".gitignore", "drafts/\n")
	write(t, root, "drafts/plan.md", "left out by the .gitignore\n")
	write(t, root, "drafts/figure.png", "png\n")
	write(t, root, "node_modules/pkg/README.md", "installed copy\n")
	write(t, root, "env/pyvenv.cfg", "home = /usr/bin\n")
	write(t, root, "env/lib/README.md", "a virtual environment's own document\n")
	write(t, root, "Concepts/golang/Source.md",
		"[a](../../drafts/plan.md)\n[b](../../node_modules/pkg/README.md)\n[c](../../env/lib/README.md)\n"+
			"![d](../../drafts/figure.png)\n[gone](../../Nowhere.md)\n")
	writeTestContract(t, root, nil)
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	var got []string
	for _, f := range findings {
		if strings.HasPrefix(string(f.RuleID), "link.broken") {
			got = append(got, f.Evidence)
		}
	}
	if diff := cmp.Diff([]string{"Nowhere.md does not exist in the vault"}, got); diff != "" {
		t.Errorf("broken-link findings (-want +got):\n%s", diff)
	}
}

// TestMarkdownOutsideFindingIsInformational holds a link that climbs out of
// the vault to an info finding on the written path: what lies outside is not
// this vault's to check.
func TestMarkdownOutsideFindingIsInformational(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Concepts/golang/Source.md", "[outside](../../../Outside.md)\n")
	writeTestContract(t, root, nil)
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	f := findFinding(t, findings, "link.broken.path", "Concepts/golang/Source.md", "../../../Outside.md")
	if f.Severity != SeverityInfo {
		t.Errorf("Severity = %v, want info", f.Severity)
	}
}

func TestMarkdownPrivateSourcesStayOutOfPublicCheck(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestContract(t, root, []string{"Restricted"})
	write(t, root, "Concepts/golang/Public.md", "[public](Public%20missing.md)\n")
	write(t, root, "Restricted/source.md", "[private](../Concepts/golang/Secret%20missing.md)\n")
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var wire bytes.Buffer
	if err := writeJSONL(&wire, findings); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(wire.String(), "Restricted") || strings.Contains(wire.String(), "Secret") || !strings.Contains(wire.String(), "Public%20missing.md") {
		t.Errorf("Check privacy lost: %s", wire.String())
	}
	for _, tt := range []struct {
		name  string
		all   bool
		paths []string
	}{
		{name: "default"}, {name: "all", all: true}, {name: "scoped", paths: []string{"Concepts"}}, {name: "all scoped", all: true, paths: []string{"Concepts"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stdout, exit, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, All: tt.all, Paths: tt.paths})
			if err != nil {
				t.Fatal(err)
			}
			if exit != 0 {
				t.Fatalf("RunCheck exit=%d, want 0", exit)
			}
			if bytes.Contains(stdout, []byte("Restricted")) || bytes.Contains(stdout, []byte("Secret")) || !bytes.Contains(stdout, []byte("Public%20missing.md")) {
				t.Errorf("RunCheck private source escaped/full public report lost: %s", stdout)
			}
		})
	}
}
