package judge

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestMarkdownPathReferenceKeepsItsWrittenIdentity(t *testing.T) {
	t.Parallel()
	got := extractPathRefs("Heading\n\n[missing](Nothing%20here.md?at=1#part)\n", 5)
	want := []pathRef{{target: "Nothing%20here.md", line: 7, code: false}}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(pathRef{})); diff != "" {
		t.Errorf("encoded Markdown reference (-want +got):\n%s", diff)
	}
}

func TestMarkdownCheckSerializesTheWrittenPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Concepts/golang/Source.md", "intro\n\n[missing](Nothing%20here.md)\n")
	writeTestContract(t, root, nil)
	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	var paths []Finding
	for _, finding := range findings {
		if finding.RuleID == "link.broken.path" {
			paths = append(paths, finding)
		}
	}
	var buf bytes.Buffer
	if err := WriteJSONL(&buf, paths); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	want := `{"rule_id":"link.broken.path","severity":"warn","path":"Concepts/golang/Source.md","line":3,"message":"link to Nothing%20here.md resolves to no file","evidence":"Concepts/golang/Nothing here.md does not exist in the vault","suggested_action":"fix the path, restore the file, or remove the reference","source_rule":"yomihon","target":"Nothing%20here.md","fingerprint":"v1:d506cf9915df6962"}` + "\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Errorf("encoded Markdown JSONL (-want +got):\n%s", diff)
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
			findings, err := Check(t.Context(), root)
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

func TestEncodedBacktickReferencesKeepTheirOldDomain(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"`Notes/Missing%20note.md`", "[invalid](Bad%2.md)", "[remote](https://host/Remote.md)", "[resource](picture.png)"} {
		if got := extractPathRefs(source, 1); len(got) != 0 {
			t.Errorf("unsupported reference %q reached the path rule: %+v", source, got)
		}
	}
	if got := extractPathRefs("`Notes/missing.md`", 1); len(got) != 1 || !got[0].code || !strings.Contains(got[0].target, "Notes/") {
		t.Errorf("plain backtick reference changed: %+v", got)
	}
}

func TestMarkdownOutsideFindingKeepsItsWireIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Concepts/golang/Source.md", "[outside](../../../Outside.md)\n")
	writeTestContract(t, root, nil)
	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	var paths []Finding
	for _, finding := range findings {
		if finding.RuleID == "link.broken.path" {
			paths = append(paths, finding)
		}
	}
	var wire bytes.Buffer
	if err := WriteJSONL(&wire, paths); err != nil {
		t.Fatal(err)
	}
	want := `{"rule_id":"link.broken.path","severity":"info","path":"Concepts/golang/Source.md","line":1,"message":"link to ../../../Outside.md points outside the vault root","evidence":"external path, not stat'd (existence varies by environment)","suggested_action":"if it should be in the vault, fix the path; otherwise informational","source_rule":"yomihon","target":"../../../Outside.md","fingerprint":"v1:8bf4a52083ad6a90"}` + "\n"
	if diff := cmp.Diff(want, wire.String()); diff != "" {
		t.Errorf("outside Markdown JSONL (-want +got):\n%s", diff)
	}
}

func TestMarkdownPrivateSourceNeverInspectsTheTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestContract(t, root, []string{"Restricted"})
	authority := loadTestAuthority(t, root)
	source := note{path: "Restricted/source.md"}
	finding, found := classifyCapturedPathRef(&source, "Restricted", pathRef{target: "../Public/Other.md"}, diskRefContext{
		authority: authority, contains: func(p string) bool {
			t.Errorf("private source inspected target %q", p)
			return true
		},
	})
	if found {
		t.Errorf("private source published finding: %+v", finding)
	}
}

func TestMarkdownUnreadableCapturedFileStillOwnsItsPath(t *testing.T) {
	t.Parallel()
	got := graph.ResolveMarkdown("Notes/source.md", "unreadable.md", func(string) bool { return true }, func(p string) bool {
		return p == "Notes/unreadable.md"
	})
	if got.Kind != graph.KindUnique || got.RelPath != "Notes/unreadable.md" {
		t.Errorf("captured unreadable path was discarded: %+v", got)
	}
}

func TestMarkdownCheckWithholdsTheReportWhenTheCallerCancels(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Concepts/golang/Source.md", "[missing](Nothing%20here.md)\n")
	writeTestContract(t, root, nil)
	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := findingByRule(findings, "link.broken.path"); !found {
		t.Fatal("live Markdown scan did not reach the path finding")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	findings, err = Check(ctx, root)
	if err == nil || findings != nil {
		t.Errorf("cancelled Markdown scan returned findings=%+v error=%v, want a refusal without a report", findings, err)
	}
}

func TestMarkdownPrivateSourcesStayOutOfPublicCheck(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestContract(t, root, []string{"Restricted"})
	write(t, root, "Concepts/golang/Public.md", "[public](Public%20missing.md)\n")
	write(t, root, "Restricted/source.md", "[private](../Concepts/golang/Secret%20missing.md)\n")
	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	var wire bytes.Buffer
	if err := WriteJSONL(&wire, findings); err != nil {
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
			stdout, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, All: tt.all, Paths: tt.paths})
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
