package judge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	Name        string
	Property    string
	Identity    string
	File        string
	Function    string
	Needle      string
	Fault       string
	Body        string
	Package     string
	ControlTest string
}

// Every independently variable block producer has its own compiling fault.
func agreementMutations() []agreementMutation {
	modes := []agreementMutation{
		{Name: "p0-diagnostic", Property: "P0", Identity: "diagnostic-html", File: "internal/render/render.go", Function: "report", Needle: "c.diags = append(c.diags, *d)", Fault: "if d.Kind != DiagWikilinkBroken { c.diags = append(c.diags, *d) }", Body: "[[Absent]]"},
		{Name: "p0-markdown-diagnostic", Property: "P0", Identity: "markdown-diagnostic-html", File: "internal/render/markdownlink.go", Function: "resolveMarkdownLinks", Needle: "col.report(&Diagnostic{Kind: DiagMarkdownBroken, Target: written, Message: message})", Fault: "if !result.Outside { col.report(&Diagnostic{Kind: DiagMarkdownBroken, Target: written, Message: message}) }", Body: "[out](../../../etc/passwd.md)\n"},
		{Name: "p1-occurrence", Property: "P1", Identity: "citation-occurrences", File: "internal/judge/planned.go", Function: "LinkTargets", Needle: "return targets", Fault: "if len(targets) > 0 { return targets[:len(targets)-1] }; return targets", Body: "[[A]] [[A]]"},
		{Name: "p2-code", Property: "P2", Identity: "wikilink-in-code", File: "internal/render/wikilink.go", Function: "convertWikilinks", Needle: "return replaceOutside(text, spans, wikilinkToken,", Fault: "return replaceOutside(text, nil, wikilinkToken,", Body: "`[[A]]`"},
		{Name: "p3-check", Property: "P3", Identity: "block-three-way", File: "internal/judge/fragment.go", Function: "blockAddressed", Needle: "return slices.Contains(addresses, want)", Fault: "return false && slices.Contains(addresses, want)", Body: "first ^a\n\nsecond\n"},
		{Name: "p3-page", Property: "P3", Identity: "block-three-way", File: "internal/render/blockanchor.go", Function: "blockAnchorSpan", Needle: "`<span id=\"` + html.EscapeString(id) + `\">`", Fault: "`<span` + `>`", Body: "first ^a\n\nsecond\n"},
		{Name: "p3-excerpt", Property: "P3", Identity: "block-three-way", File: "internal/render/section.go", Function: "Excerpt", Needle: "return stripped.sourceSlice(slice), matches > 0", Fault: "return stripped.sourceSlice(slice), matches < 0", Body: "first ^a\n\nsecond\n"},
		{Name: "p4-heading", Property: "P4", Identity: "literal-heading-id", File: "internal/judge/note.go", Function: "readNote", Needle: "n.sectionAnchors, n.excerptSectionAnchors, n.blockAddresses = anchorSurfaceFrom(body, facts.comments)", Fault: "n.sectionAnchors, n.excerptSectionAnchors, n.blockAddresses = anchorSurfaceFrom(body, facts.comments); n.sectionAnchors = nil; n.excerptSectionAnchors = nil", Body: "## A\n"},
		{Name: "p0-citation-shape", Property: "P0", Identity: "citation-shape", File: "internal/render/wikilink.go", Function: "resolvedWikilink", Needle: "`<a href=\"%s\" class=\"wikilink\"%s>%s</a>`", Fault: "`<span href=\"%s\" class=\"wikilink\"%s>%s</span>`", Body: "[[A]]"},
		{Name: "p1-provenance", Property: "P1", Identity: "provenance-identity", File: "internal/render/wikilink.go", Function: "embedSourceLine", Needle: "`<a href=\"` + attributeEscaper.Replace(notesHref(relPath)) + `\">` + html.EscapeString(noteName(relPath)) + `</a></p>`", Fault: "html.EscapeString(noteName(relPath)) + `</p>`", Body: "![[Notes/Child]]\n"},
	}
	modes = append(modes, snapshotBodyMutations()...)
	modes = append(modes, agreementMutation{Name: "f3-wikilink-target", Property: "F3", Identity: "wikilink-target", File: "internal/graph/wikilink.go", Function: "ParseWikilink", Needle: "beforeBlock = strings.TrimRight(beforeBlock, `\\`)", Fault: "beforeBlock = strings.TrimRight(beforeBlock, \"\")", Package: "./internal/judge", ControlTest: "TestAgreementWikilinkTargets"})
	modes = append(modes, bodyValueMutations()...)
	modes = append(modes, bodyFieldMutations()...)
	modes = append(modes, stage4Mutations()...)
	return append(modes, stage5Mutations()...)
}

func TestAgreementMutationControl(t *testing.T) {
	modes := agreementMutations()
	for i := range modes {
		mode := &modes[i]
		t.Run(mode.Name, func(t *testing.T) {
			if mode.ControlTest != "" {
				agreementNativeMutationControl(t, mode)
				return
			}
			if mode.Property == "F3" {
				bodyValueMutationControl(t, mode)
				t.Logf("AGREEMENT-INVOKED F3/%s", mode.Name)
				return
			}
			if mode.Name == "p0-markdown-diagnostic" {
				// Select an independent literal for this Markdown stimulus, never
				// the generic P0 control's wiki-only Absent expectation.
				control := agreementProjectionCases()[0]
				if control.Name != "outside-markdown" || control.Body != mode.Body {
					t.Fatal("not-applied: outside Markdown literal selection changed")
				}
				agreementProjectionControl(t, &control)
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
			failures := agreementPageFailures(mode.Body, &result, &actual)
			for failureIndex := range failures {
				failure := &failures[failureIndex]
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
				agreementProjectionControl(t, &mixed)
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
					t.Fatalf("not-applied: setup-status=2 heading control (-want +got):\n%s", diff)
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
	t.Parallel()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("not-applied: setup-status=2 resolve repository root: %v", err)
	}
	modes := agreementMutations()
	for i := range modes {
		mode := &modes[i]
		t.Run(mode.Name, func(t *testing.T) {
			t.Parallel()
			t.Cleanup(func() {
				if t.Failed() {
					t.Logf("not-applied: %s/%s setup-status=2; Go wrapper failure status is 1", mode.Property, mode.Name)
				}
			})
			packagePath := mode.Package
			controlTest := mode.ControlTest
			if packagePath == "" {
				packagePath = "./internal/judge"
			}
			if controlTest == "" {
				controlTest = "TestAgreementMutationControl"
			}
			for _, red := range []bool{true, false} {
				state := "green"
				if red {
					state = "red"
				}
				overlay := agreementMutationOverlay(t, root, mode, red)
				binary := filepath.Join(t.TempDir(), "qualified.test")
				compile := agreementMutationCommand(t, root, "test", "-overlay="+overlay, "-c", "-o="+binary, packagePath)
				if compile.Status != 0 {
					t.Fatalf("not-applied: setup-status=2 %s compile-only qualification failed status=%d\n%s", state, compile.Status, compile.Output)
				}
				selected := controlTest
				run := "^" + controlTest + "$"
				if controlTest == "TestAgreementMutationControl" {
					selected += "/" + mode.Name
					run += "/^" + mode.Name + "$"
				}
				args := []string{"tool", "test2json", "-t", "-p", packagePath, binary, "-test.short", "-test.count=1", "-test.timeout=90s", "-test.v=test2json", "-test.run=" + run}
				sourceDigest := ""
				if packagePath == "./internal/judge" {
					alternate := agreementMutationSource(t, overlay)
					args = append(args, "-agreement-source="+alternate)
					if mode.Name == "f3-consumer-parse" || (strings.HasPrefix(mode.Name, "stage4-") || strings.HasPrefix(mode.Name, "stage5-")) {
						sourceDigest = agreementMutationSourceDigest(t, alternate)
					}
				}
				// The assertion runs the exact binary whose compilation qualified
				// this overlay, in the package directory its fixtures belong to.
				packageRoot := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(packagePath, "./")))
				child := agreementMutationCommand(t, packageRoot, args...)
				t.Logf("mode=%s state=%s status=%d\n%s", mode.Name, state, child.Status, child.Output)
				agreementMutationReceipt(t, mode, selected, state, red, child, sourceDigest)
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
	status := 0
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			status = exit.ExitCode()
		} else {
			t.Fatalf("not-applied: setup-status=2 start qualification command actual-status=unavailable: %v", err)
		}
	}
	if ctx.Err() != nil {
		t.Fatalf("not-applied: setup-status=2 qualification command timed out: %v actual-status=%d\n%s", ctx.Err(), status, output)
	}
	return agreementMutationOutput{Status: status, Output: output}
}

func agreementMutationOverlay(t *testing.T, root string, mode *agreementMutation, red bool) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(mode.File))
	repo, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("not-applied: setup-status=2 open production root: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := repo.Close(); closeErr != nil {
			t.Errorf("not-applied: setup-status=2 close production root: %v", closeErr)
		}
	})
	source, err := repo.ReadFile(filepath.FromSlash(mode.File))
	if err != nil {
		t.Fatalf("not-applied: setup-status=2 read production source: %v", err)
	}
	set := token.NewFileSet()
	parsed, err := parser.ParseFile(set, path, source, 0)
	if err != nil {
		t.Fatalf("not-applied: setup-status=2 parse production source: %v", err)
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
		t.Fatalf("not-applied: setup-status=2 %s function matches=%d", mode.Name, count)
	}
	start, end := set.Position(function.Body.Pos()).Offset, set.Position(function.Body.End()).Offset
	body := string(source[start:end])
	if count := strings.Count(body, mode.Needle); count != 1 {
		t.Fatalf("not-applied: setup-status=2 %s needle matches=%d", mode.Name, count)
	}
	state := "green"
	if red {
		state = "red"
		body = strings.Replace(body, mode.Needle, mode.Fault, 1)
		if strings.Count(body, mode.Fault) != 1 {
			t.Fatalf("not-applied: setup-status=2 %s transformed site is ambiguous; setup-status=2", mode.Name)
		}
	}
	// The receipt is produced by the real edited function in both states; the
	// control's separate receipt proves its observations reached the assertion.
	marker := "AGREEMENT-SINK " + mode.Name + "/" + state
	body = "{\nprintln(" + strconv.Quote(marker) + ")\n" + body[1:]
	backing := t.TempDir()
	copyPath := filepath.Join(backing, "production.go")
	alternate := append(bytes.Clone(source[:start]), []byte(body)...)
	alternate = append(alternate, source[end:]...)
	if _, parseErr := parser.ParseFile(token.NewFileSet(), copyPath, alternate, 0); parseErr != nil {
		t.Fatalf("not-applied: setup-status=2 alternate Go source: %v", parseErr)
	}
	owned, err := os.OpenRoot(backing)
	if err != nil {
		t.Fatalf("not-applied: setup-status=2 open alternate root: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := owned.Close(); closeErr != nil {
			t.Errorf("not-applied: setup-status=2 close alternate root: %v", closeErr)
		}
	})
	if writeErr := owned.WriteFile("production.go", alternate, 0o600); writeErr != nil {
		t.Fatalf("not-applied: setup-status=2 write alternate source: %v", writeErr)
	}
	data, err := json.Marshal(struct{ Replace map[string]string }{Replace: map[string]string{path: copyPath}})
	if err != nil {
		t.Fatalf("not-applied: setup-status=2 encode overlay: %v", err)
	}
	overlay := filepath.Join(backing, "overlay.json")
	if writeErr := owned.WriteFile("overlay.json", data, 0o600); writeErr != nil {
		t.Fatalf("not-applied: setup-status=2 write overlay: %v", writeErr)
	}
	return overlay
}

func agreementMutationReceipt(t *testing.T, mode *agreementMutation, selected, state string, red bool, child agreementMutationOutput, sourceDigest string) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(child.Output))
	invoked, sink, caught, terminal := false, false, false, false
	sourceConsumed := sourceDigest == ""
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
			t.Fatalf("not-applied: setup-status=2 child JSON events: %v", err)
		}
		if strings.Contains(strings.ToLower(event.Output), "not-applied") {
			t.Fatalf("not-applied: setup-status=2 %s/%s child setup failed: %s", mode.Name, state, event.Output)
		}
		if event.Action == "fail" && event.Test != "" && event.Test != selected && event.Test != "TestAgreementMutationControl" {
			t.Fatalf("not-applied: setup-status=2 unrelated child failure %q", event.Test)
		}
		if event.Test == selected {
			sourceConsumed = sourceConsumed || agreementSourceReceipt(event.Output, sourceDigest)
			invoked = invoked || strings.Contains(event.Output, "AGREEMENT-INVOKED "+mode.Property+"/"+mode.Name)
			sink = sink || strings.Contains(event.Output, "AGREEMENT-SINK "+mode.Name+"/"+state)
			caughtNeedle := "caught: " + mode.Property + " " + mode.Identity
			if mode.Property == "F3" {
				caughtNeedle += " "
			}
			caught = caught || strings.Contains(event.Output, caughtNeedle)
			pureInvoked = pureInvoked || strings.Contains(event.Output, "AGREEMENT-PURE-INVOKED P1/p1-occurrence")
			pureCaught = pureCaught || strings.Contains(event.Output, "caught: P1 citation-occurrences pure-two-a ")
			mixedInvoked = mixedInvoked || strings.Contains(event.Output, "AGREEMENT-MIXED-INVOKED P1/p1-occurrence")
			mixedCaught = mixedCaught || strings.Contains(event.Output, "caught: P1 citation-occurrences literal check case=outside-markdown-with-wiki")
			terminal = terminal || (red && event.Action == "fail") || (!red && event.Action == "pass")
		}
	}
	if !invoked || !sink || !sourceConsumed {
		t.Fatalf("not-applied: setup-status=2 %s invoked=%t sink=%t source-consumed=%t", mode.Name, invoked, sink, sourceConsumed)
	}
	if mode.Name == "p1-occurrence" && (!pureInvoked || !mixedInvoked || (red && (!pureCaught || !mixedCaught))) {
		t.Fatalf("not-applied: setup-status=2 occurrence boundaries pure-invoked=%t mixed-invoked=%t pure-caught=%t mixed-caught=%t", pureInvoked, mixedInvoked, pureCaught, mixedCaught)
	}
	want := 0
	if red {
		want = 1
	}
	if child.Status != want || !terminal || (red && !caught) {
		t.Fatalf("not-applied: setup-status=2 qualification %s/%s status=%d want=%d terminal=%t expected-caught=%t", mode.Name, state, child.Status, want, terminal, caught)
	}
}

func agreementMutationSource(t *testing.T, overlay string) string {
	t.Helper()
	data, err := os.ReadFile(overlay) // #nosec G304 -- test-owned overlay.json returned by agreementMutationOverlay, never product input
	if err != nil {
		t.Fatalf("not-applied: setup-status=2 read overlay: %v", err)
	}
	var value struct{ Replace map[string]string }
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("not-applied: setup-status=2 decode overlay: %v", err)
	}
	if len(value.Replace) != 1 {
		t.Fatal("not-applied: setup-status=2 overlay must replace exactly one source")
	}
	for _, alternate := range value.Replace {
		return alternate
	}
	t.Fatal("not-applied: setup-status=2 overlay source absent")
	return ""
}

func agreementMutationSourceDigest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304 -- test-owned production.go selected from the generated mutation overlay, never product input
	if err != nil {
		t.Fatalf("not-applied: setup-status=2 read actual alternate source: %v", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func agreementSourceReceipt(output, digest string) bool {
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] == "AGREEMENT-SOURCE-CONSUMED" && fields[i+1] == "sha256="+digest {
				return true
			}
		}
	}
	return false
}

// Registered positive controls reach the native graph or judge boundary directly.
func agreementNativeMutationControl(t *testing.T, mode *agreementMutation) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("not-applied: %s/%s setup-status=2 resolve native control root: %v", mode.Property, mode.Name, err)
	}
	child := agreementMutationCommand(t, root, "test", "-short", "-count=1", "-timeout=90s", "-json", "-run=^"+mode.ControlTest+"$", mode.Package)
	t.Logf("native-positive mode=%s actual-status=%d\n%s", mode.Name, child.Status, child.Output)
	decoder := json.NewDecoder(bytes.NewReader(child.Output))
	invoked, terminal, caught, failed := false, false, false, false
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
			t.Fatalf("not-applied: %s/%s setup-status=2 native control JSON: %v", mode.Property, mode.Name, err)
		}
		if strings.Contains(strings.ToLower(event.Output), "not-applied") {
			t.Fatalf("not-applied: %s/%s setup-status=2 native control setup: %s", mode.Property, mode.Name, event.Output)
		}
		if event.Test == mode.ControlTest {
			invoked = invoked || strings.Contains(event.Output, "AGREEMENT-INVOKED "+mode.Property+"/"+mode.Name)
			terminal = terminal || event.Action == "pass"
			failed = failed || event.Action == "fail"
			caught = caught || strings.Contains(event.Output, "caught: "+mode.Property+" "+mode.Identity+" ")
		}
	}
	if invoked && failed && caught && child.Status == 1 {
		t.Errorf("caught: %s %s native positive assertion failed actual-status=1", mode.Property, mode.Identity)
		t.Logf("AGREEMENT-INVOKED %s/%s", mode.Property, mode.Name)
		return
	}
	if !invoked || !terminal || child.Status != 0 {
		t.Fatalf("not-applied: %s/%s setup-status=2 native positive invoked=%t pass=%t actual-status=%d", mode.Property, mode.Name, invoked, terminal, child.Status)
	}
	t.Logf("AGREEMENT-INVOKED %s/%s", mode.Property, mode.Name)
}
