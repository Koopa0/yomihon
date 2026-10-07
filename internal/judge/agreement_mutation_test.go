package judge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

type agreementMutation struct {
	Name     string
	Property string
	Identity string
	File     string
	Function string
	Needle   string
	Fault    string
	Body     string
}

// Every independently variable block producer has its own compiling fault.
func agreementMutations() []agreementMutation {
	return []agreementMutation{
		{Name: "p0-diagnostic", Property: "P0", Identity: "diagnostic-html", File: "internal/render/render.go", Function: "report", Needle: "c.diags = append(c.diags, *d)", Fault: "if d.Kind != DiagWikilinkBroken { c.diags = append(c.diags, *d) }", Body: "[[Absent]]"},
		{Name: "p0-markdown-diagnostic", Property: "P0", Identity: "markdown-diagnostic-html", File: "internal/render/markdownlink.go", Function: "resolveMarkdownLinks", Needle: "col.report(&Diagnostic{Kind: DiagMarkdownBroken, Target: written, Message: message})", Fault: "if !result.Outside { col.report(&Diagnostic{Kind: DiagMarkdownBroken, Target: written, Message: message}) }", Body: "[out](../../../etc/passwd.md)\n"},
		{Name: "p1-occurrence", Property: "P1", Identity: "citation-occurrences", File: "internal/judge/planned.go", Function: "LinkTargets", Needle: "return targets", Fault: "if len(targets) > 0 { return targets[:len(targets)-1] }; return targets", Body: "[[A]] [[A]]"},
		{Name: "p2-code", Property: "P2", Identity: "wikilink-in-code", File: "internal/render/wikilink.go", Function: "convertWikilinks", Needle: "spans := codeSpanRanges(text)", Fault: "_ = codeSpanRanges(text); var spans [][2]int", Body: "`[[A]]`"},
		{Name: "p3-check", Property: "P3", Identity: "block-three-way", File: "internal/judge/fragment.go", Function: "blockAddressed", Needle: "return true", Fault: "return false", Body: "first ^a\n\nsecond\n"},
		{Name: "p3-page", Property: "P3", Identity: "block-three-way", File: "internal/render/blockanchor.go", Function: "blockAnchorSpan", Needle: "`<span id=\"` + html.EscapeString(id) + `\">`", Fault: "`<span>`", Body: "first ^a\n\nsecond\n"},
		{Name: "p3-excerpt", Property: "P3", Identity: "block-three-way", File: "internal/render/section.go", Function: "Excerpt", Needle: "return slice, matches > 0", Fault: "return slice, matches < 0", Body: "first ^a\n\nsecond\n"},
		{Name: "p4-heading", Property: "P4", Identity: "literal-heading-id", File: "internal/judge/note.go", Function: "readNote", Needle: "n.sectionAnchors, n.excerptSectionAnchors, n.blockAnchorLines = anchorSurfaceFrom(body, facts.comments)", Fault: "n.sectionAnchors, n.excerptSectionAnchors, n.blockAnchorLines = anchorSurfaceFrom(body, facts.comments); n.sectionAnchors = nil; n.excerptSectionAnchors = nil", Body: "## A\n"},
		{Name: "p0-citation-shape", Property: "P0", Identity: "citation-shape", File: "internal/render/wikilink.go", Function: "resolvedWikilink", Needle: "`<a href=\"%s\" class=\"wikilink\"%s>%s</a>`", Fault: "`<span href=\"%s\" class=\"wikilink\"%s>%s</span>`", Body: "[[A]]"},
		{Name: "p1-provenance", Property: "P1", Identity: "provenance-identity", File: "internal/render/wikilink.go", Function: "embedSourceLine", Needle: "`<a href=\"` + attributeEscaper.Replace(notesHref(relPath)) + `\">` + html.EscapeString(noteName(relPath)) + `</a></p>`", Fault: "html.EscapeString(noteName(relPath)) + `</p>`", Body: "![[Notes/Child]]\n"},
	}
}

func TestAgreementMutationControl(t *testing.T) {
	for _, mode := range agreementMutations() {
		t.Run(mode.Name, func(t *testing.T) {
			if mode.Name == "p0-markdown-diagnostic" {
				// Select an independent literal for this Markdown stimulus, never
				// the generic P0 control's wiki-only Absent expectation.
				control := agreementProjectionCases()[0]
				if control.Name != "outside-markdown" || control.Body != mode.Body {
					t.Fatal("not-applied: outside Markdown literal selection changed")
				}
				agreementProjectionControl(t, control)
				t.Logf("AGREEMENT-INVOKED %s/%s", mode.Property, mode.Name)
				return
			}
			if mode.Identity == "provenance-identity" {
				bodies := capturedBodies{
					"Notes/A.md":     "A\n",
					"Notes/Child.md": "## Child Heading\n\n[[A|alias]] [[A]] `[[A]]`\n",
				}
				result, _ := agreementTransclusionControl(t, mode.Body, bodies)
				if len(result.Diagnostics) != 0 {
					t.Errorf("caught: P0 transclusion-diagnostics diagnostics=%+v", result.Diagnostics)
				}
				t.Logf("AGREEMENT-INVOKED %s/%s", mode.Property, mode.Name)
				return
			}
			idx := graph.BuildFromNotes(nil, nil)
			bodies := capturedBodies{}
			if mode.Property == "P2" || mode.Identity == "citation-shape" {
				idx = graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/A.md"}}, nil)
				bodies["Notes/A.md"] = "A\n"
			}
			page := render.New(idx, bodies, noTitlesDeclared{}, everyFileHeld{})
			result := page.HTML("Notes/Reading.md", "", mode.Body, wording.En)
			var actual agreementHTML
			if mode.Property == "P2" || mode.Identity == "citation-shape" {
				actual = agreementObserveKnown(t, result.HTML, map[string]string{"Notes/A.md": "A"})
			} else {
				actual = agreementObserve(t, result.HTML)
			}
			for _, failure := range agreementPageFailures(mode.Body, result, actual) {
				if mode.Name == "p1-occurrence" && failure.Property == "P1" && failure.Identity == "citation-occurrences" {
					t.Errorf("caught: P1 citation-occurrences pure-two-a body=%q observations=%s", mode.Body, failure.Observation)
				} else {
					t.Errorf("caught: %s %s body=%q observations=%s", failure.Property, failure.Identity, mode.Body, failure.Observation)
				}
			}
			switch mode.Property {
			case "P0":
				want := []agreementCitation{{Target: "Absent", State: "wikilink-broken"}}
				if mode.Identity == "citation-shape" {
					want = []agreementCitation{{Target: "A", State: "wikilink"}}
				}
				if diff := cmp.Diff(want, actual.Citations); diff != "" {
					t.Errorf("caught: P0 %s literal carrier (-want +got):\n%s", mode.Identity, diff)
				}
			case "P1":
				if mode.Body != "[[A]] [[A]]" {
					t.Fatal("not-applied: pure occurrence literal selection changed")
				}
				want := []agreementCitation{{Target: "A", State: "wikilink-broken"}, {Target: "A", State: "wikilink-broken"}}
				if diff := cmp.Diff(want, actual.Citations); diff != "" {
					t.Errorf("caught: P1 citation-occurrences literal carriers (-want +got):\n%s", diff)
				}
				t.Log("AGREEMENT-PURE-INVOKED P1/p1-occurrence")
				// The original pure two-A control remains above. Offer the mixed
				// three-carrier boundary unchanged in both overlay states too.
				mixed := agreementProjectionCases()[1]
				if mixed.Name != "outside-markdown-with-wiki" || mixed.Body != "[out](../../../etc/passwd.md) [[A]] [[A]]\n" {
					t.Fatal("not-applied: mixed occurrence literal selection changed")
				}
				agreementProjectionControl(t, mixed)
				t.Log("AGREEMENT-MIXED-INVOKED P1/p1-occurrence")
			case "P2":
				if len(actual.Citations) != 0 {
					t.Errorf("quoted control citations = %+v, want none", actual.Citations)
				}
			case "P3":
				cut, found := render.Excerpt(mode.Body, "^a")
				want := struct {
					Cut   string
					Found bool
				}{Cut: "first ^a", Found: true}
				got := struct {
					Cut   string
					Found bool
				}{Cut: cut, Found: found}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("caught: P3 bounded-excerpt (-want +got):\n%s", diff)
				}
				agreementFragments(t, []agreementCase{{Name: mode.Name, Body: mode.Body}}, []agreementHTML{actual})
			case "P4":
				if diff := cmp.Diff([]string{"a"}, actual.Headings); diff != "" {
					t.Fatalf("not-applied: heading control (-want +got):\n%s", diff)
				}
				agreementFragments(t, []agreementCase{{Name: mode.Name, Body: mode.Body}}, []agreementHTML{actual})
			}
			t.Logf("AGREEMENT-INVOKED %s/%s", mode.Property, mode.Name)
		})
	}
}

// Child selection excludes this driver, so neither corpus shards nor nested
// overlay processes enter the bounded qualification run.
func TestAgreementMutations(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("not-applied: resolve repository root: %v", err)
	}
	for _, mode := range agreementMutations() {
		t.Run(mode.Name, func(t *testing.T) {
			for _, red := range []bool{true, false} {
				state := "green"
				if red {
					state = "red"
				}
				overlay := agreementMutationOverlay(t, root, mode, red)
				compile := agreementMutationCommand(t, root, "test", "-overlay="+overlay, "-short", "-run=^$", "./internal/judge")
				if compile.Status != 0 {
					t.Fatalf("not-applied: %s compile-only qualification failed status=%d\n%s", state, compile.Status, compile.Output)
				}
				selected := "TestAgreementMutationControl/" + mode.Name
				child := agreementMutationCommand(t, root, "test", "-overlay="+overlay, "-short", "-count=1", "-timeout=90s", "-json", "-run=^TestAgreementMutationControl$/^"+mode.Name+"$", "./internal/judge")
				t.Logf("mode=%s state=%s status=%d\n%s", mode.Name, state, child.Status, child.Output)
				agreementMutationReceipt(t, mode, selected, state, red, child)
			}
		})
	}
}

type agreementMutationOutput struct {
	Status int
	Output []byte
}

func agreementMutationCommand(t *testing.T, root string, args ...string) agreementMutationOutput {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...) // #nosec G204 -- fixed Go tool and test-owned overlay arguments
	cmd.Dir = root
	cmd.WaitDelay = 5 * time.Second
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("not-applied: qualification command timed out: %v\n%s", ctx.Err(), output)
	}
	status := 0
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			status = exit.ExitCode()
		} else {
			t.Fatalf("not-applied: start qualification command: %v", err)
		}
	}
	return agreementMutationOutput{Status: status, Output: output}
}

func agreementMutationOverlay(t *testing.T, root string, mode agreementMutation, red bool) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(mode.File))
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("not-applied: read production source: %v", err)
	}
	set := token.NewFileSet()
	parsed, err := parser.ParseFile(set, path, source, 0)
	if err != nil {
		t.Fatalf("not-applied: parse production source: %v", err)
	}
	var function *ast.FuncDecl
	count := 0
	for _, declaration := range parsed.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Name.Name == mode.Function {
			function = fn
			count++
		}
	}
	if count != 1 || function.Body == nil {
		t.Fatalf("not-applied: %s function matches=%d", mode.Name, count)
	}
	start, end := set.Position(function.Body.Pos()).Offset, set.Position(function.Body.End()).Offset
	body := string(source[start:end])
	if count := strings.Count(body, mode.Needle); count != 1 {
		t.Fatalf("not-applied: %s needle matches=%d", mode.Name, count)
	}
	state := "green"
	if red {
		state = "red"
		body = strings.Replace(body, mode.Needle, mode.Fault, 1)
	}
	// The receipt is produced by the real edited function in both states; the
	// control's separate receipt proves its observations reached the assertion.
	marker := "AGREEMENT-SINK " + mode.Name + "/" + state
	body = "{\nprintln(" + strconv.Quote(marker) + ")\n" + body[1:]
	backing := t.TempDir()
	copyPath := filepath.Join(backing, "production.go")
	alternate := append(bytes.Clone(source[:start]), []byte(body)...)
	alternate = append(alternate, source[end:]...)
	if _, err := parser.ParseFile(token.NewFileSet(), copyPath, alternate, 0); err != nil {
		t.Fatalf("not-applied: alternate Go source: %v", err)
	}
	if err := os.WriteFile(copyPath, alternate, 0o600); err != nil {
		t.Fatalf("not-applied: write alternate source: %v", err)
	}
	data, err := json.Marshal(struct{ Replace map[string]string }{Replace: map[string]string{path: copyPath}})
	if err != nil {
		t.Fatalf("not-applied: encode overlay: %v", err)
	}
	overlay := filepath.Join(backing, "overlay.json")
	if err := os.WriteFile(overlay, data, 0o600); err != nil {
		t.Fatalf("not-applied: write overlay: %v", err)
	}
	return overlay
}

func agreementMutationReceipt(t *testing.T, mode agreementMutation, selected, state string, red bool, child agreementMutationOutput) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(child.Output))
	invoked, sink, caught, terminal := false, false, false, false
	pureInvoked, pureCaught := false, false
	mixedInvoked, mixedCaught := false, false
	for {
		var event struct {
			Action string
			Test   string
			Output string
		}
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("not-applied: child JSON events: %v", err)
		}
		if strings.Contains(strings.ToLower(event.Output), "not-applied") {
			t.Fatalf("not-applied: %s/%s child setup failed: %s", mode.Name, state, event.Output)
		}
		if event.Action == "fail" && event.Test != "" && event.Test != selected && event.Test != "TestAgreementMutationControl" {
			t.Fatalf("not-applied: unrelated child failure %q", event.Test)
		}
		if event.Test == selected {
			invoked = invoked || strings.Contains(event.Output, "AGREEMENT-INVOKED "+mode.Property+"/"+mode.Name)
			sink = sink || strings.Contains(event.Output, "AGREEMENT-SINK "+mode.Name+"/"+state)
			caught = caught || strings.Contains(event.Output, "caught: "+mode.Property+" "+mode.Identity)
			pureInvoked = pureInvoked || strings.Contains(event.Output, "AGREEMENT-PURE-INVOKED P1/p1-occurrence")
			pureCaught = pureCaught || strings.Contains(event.Output, "caught: P1 citation-occurrences pure-two-a ")
			mixedInvoked = mixedInvoked || strings.Contains(event.Output, "AGREEMENT-MIXED-INVOKED P1/p1-occurrence")
			mixedCaught = mixedCaught || strings.Contains(event.Output, "caught: P1 citation-occurrences literal check case=outside-markdown-with-wiki")
			terminal = terminal || (red && event.Action == "fail") || (!red && event.Action == "pass")
		}
	}
	if !invoked || !sink {
		t.Fatalf("not-applied: %s invoked=%t sink=%t", mode.Name, invoked, sink)
	}
	if mode.Name == "p1-occurrence" && (!pureInvoked || !mixedInvoked || (red && (!pureCaught || !mixedCaught))) {
		t.Fatalf("not-applied: occurrence boundaries pure-invoked=%t mixed-invoked=%t pure-caught=%t mixed-caught=%t", pureInvoked, mixedInvoked, pureCaught, mixedCaught)
	}
	want := 0
	if red {
		want = 1
	}
	if child.Status != want || !terminal || (red && !caught) {
		t.Fatalf("qualification %s/%s status=%d want=%d terminal=%t expected-caught=%t", mode.Name, state, child.Status, want, terminal, caught)
	}
}
