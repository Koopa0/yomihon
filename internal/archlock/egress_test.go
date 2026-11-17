package archlock

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The agent-facing commands hand their output to a program, so what the rules
// file withholds must not be in it. internal/egress is where that is decided:
// it asks the rules file, it names the link extractor every withheld title is
// matched with, and it makes the only value a command may write. The walks
// here hold the rest of the tree to that, from the syntax tree, so a command
// that forgets cannot be written without one of them failing.

const (
	egressImport = "github.com/koopa0/yomihon/internal/egress"
	judgeImport  = "github.com/koopa0/yomihon/internal/judge"
	egressDir    = "internal/egress/"
	judgeDir     = "internal/judge/"
	// mainFile is the one file that may name the process's standard output.
	mainFile = "cmd/yomihon/main.go"
)

// importedAs maps the local name each import is reached by to its path, and
// reports a dot import of one of the watched packages, which hides every
// selector from the walks below and so is refused rather than audited.
func importedAs(path string, fset *token.FileSet, file *ast.File, watched ...string) (names map[string]string, found []site) {
	names = map[string]string{}
	for _, imp := range file.Imports {
		imported, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		local := imported[strings.LastIndexByte(imported, '/')+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		if local == "." {
			for _, w := range watched {
				if imported == w {
					found = append(found, site{path: path, line: fset.Position(imp.Pos()).Line, text: "dot import of " + imported})
				}
			}
			continue
		}
		names[local] = imported
	}
	return names, found
}

// withheldQuestions are the two ways to ask the rules file what it withholds.
var withheldQuestions = map[string]bool{"EgressAllowed": true, "WithheldDirs": true}

// withheldAsks finds every spelling of the two questions in a file, called or
// held as a value. It does not look at the receiver, so a method of the same
// name on another type is reported too; there is none, and a second would be
// a second place that decides what is withheld.
func withheldAsks(path string, fset *token.FileSet, file *ast.File) (found []site, seen map[string]int) {
	seen = map[string]int{}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || !withheldQuestions[selector.Sel.Name] {
			return true
		}
		seen[selector.Sel.Name]++
		found = append(found, site{path: path, line: fset.Position(selector.Pos()).Line, text: selector.Sel.Name})
		return true
	})
	return found, seen
}

// TestOnlyTheEgressGateAsksWhatIsWithheld keeps the question in one place. A
// command that asked the rules file itself would decide for itself what a
// withheld path is, and a decision made twice can come out two ways: the
// matching of a title against withheld ground already did, and was moved here.
//
// It has to have seen the gate asking both questions, because a walk that
// reached nothing reports a tree where nobody asks, which looks like success.
func TestOnlyTheEgressGateAsksWhatIsWithheld(t *testing.T) {
	t.Parallel()

	var outside []site
	inside := map[string]int{}
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		found, seen := withheldAsks(path, fset, file)
		if strings.HasPrefix(path, egressDir) {
			for name, count := range seen {
				inside[name] += count
			}
			return
		}
		outside = append(outside, found...)
	})
	for name := range withheldQuestions {
		if inside[name] == 0 {
			t.Fatalf("%s is never asked in %s, so this walk either read nothing or the gate stopped asking, and the rule below guards a question nobody puts", name, egressDir)
		}
	}
	report(t, "this asks the rules file what it withholds; ask it through an *egress.Gate, which is the one place that decides", outside)
}

// standardOutputSpellings finds every way a file can write to the process's
// standard output without naming a writer it was given: the stream itself, a
// descriptor taken for it, the fmt functions that print to it, and the two
// builtins that print to standard error and so are no better a way out.
func standardOutputSpellings(path string, fset *token.FileSet, file *ast.File) (found []site, stdout int) {
	names, dotted := importedAs(path, fset, file, "os", "fmt")
	found = append(found, dotted...)
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.SelectorExpr:
			qualifier, ok := node.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch names[qualifier.Name] + "." + node.Sel.Name {
			case "os.Stdout":
				stdout++
				found = append(found, site{path: path, line: fset.Position(node.Pos()).Line, text: "os.Stdout"})
			case "os.NewFile", "fmt.Print", "fmt.Printf", "fmt.Println":
				found = append(found, site{path: path, line: fset.Position(node.Pos()).Line, text: names[qualifier.Name] + "." + node.Sel.Name})
			}
		case *ast.CallExpr:
			if builtin, ok := node.Fun.(*ast.Ident); ok && (builtin.Name == "print" || builtin.Name == "println") {
				found = append(found, site{path: path, line: fset.Position(node.Pos()).Line, text: builtin.Name})
			}
		}
		return true
	})
	return found, stdout
}

// TestOnlyTheCommandsMainNamesStandardOutput keeps the process's standard
// output out of reach of everything but the one file that hands it to the
// commands as a writer. An agent-facing command that wrote to it directly
// would put out what it chose without the gate having looked.
//
// The file that does name it has to have been seen doing so.
func TestOnlyTheCommandsMainNamesStandardOutput(t *testing.T) {
	t.Parallel()

	var found []site
	named := 0
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		sites, stdout := standardOutputSpellings(path, fset, file)
		if path == mainFile {
			named += stdout
			return
		}
		found = append(found, sites...)
	})
	if named == 0 {
		t.Fatalf("%s never names os.Stdout, so this walk either read nothing or the writer is handed down some other way, and the rule below guards a stream nobody holds", mainFile)
	}
	report(t, "this writes to the process's standard output; an agent-facing output leaves through an egress.Payload written to the writer main hands the command", found)
}

// openCalls reads every use of egress.Open in a file. A call is sound when its
// last argument is the judge's link extractor: from inside the judge it is
// named bare, from anywhere else through the judge's package name. The second
// result counts the sound calls.
func openCalls(path string, fset *token.FileSet, file *ast.File) (found []site, sound int) {
	names, dotted := importedAs(path, fset, file, egressImport)
	found = append(found, dotted...)
	inJudge := strings.HasPrefix(path, judgeDir)
	called := map[*ast.SelectorExpr]bool{}
	isOpen := func(selector *ast.SelectorExpr) bool {
		qualifier, ok := selector.X.(*ast.Ident)
		return ok && selector.Sel.Name == "Open" && names[qualifier.Name] == egressImport
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.CallExpr:
			selector, ok := node.Fun.(*ast.SelectorExpr)
			if !ok || !isOpen(selector) {
				return true
			}
			called[selector] = true
			if len(node.Args) == 3 && isJudgeLinkTargets(node.Args[2], names, inJudge) {
				sound++
				return true
			}
			found = append(found, site{path: path, line: fset.Position(node.Pos()).Line, text: "egress.Open without judge.LinkTargets as its extractor"})
		case *ast.SelectorExpr:
			if isOpen(node) && !called[node] {
				found = append(found, site{path: path, line: fset.Position(node.Pos()).Line, text: "egress.Open used as a value"})
			}
		}
		return true
	})
	return found, sound
}

func isJudgeLinkTargets(arg ast.Expr, names map[string]string, inJudge bool) bool {
	switch arg := arg.(type) {
	case *ast.Ident:
		return inJudge && arg.Name == "LinkTargets"
	case *ast.SelectorExpr:
		qualifier, ok := arg.X.(*ast.Ident)
		return ok && arg.Sel.Name == "LinkTargets" && names[qualifier.Name] == judgeImport
	}
	return false
}

// TestEveryGateIsOpenedWithTheJudgesLinkExtractor holds the one input the gate
// cannot check for itself. Spells matches a withheld title against the links a
// piece of text writes, and it reads those with the extractor Open was given:
// one that read fewer links than the page renders would let a withheld name
// through, and nothing at runtime says which extractor it was.
//
// Production opens a gate in two places, the judge's three commands and the
// marks command; the rule has to have seen a sound call in each kind of
// place, since a walk that matched no call would pass on any extractor.
func TestEveryGateIsOpenedWithTheJudgesLinkExtractor(t *testing.T) {
	t.Parallel()

	var found []site
	insideJudge, outsideJudge := 0, 0
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		sites, sound := openCalls(path, fset, file)
		found = append(found, sites...)
		if strings.HasPrefix(path, judgeDir) {
			insideJudge += sound
		} else {
			outsideJudge += sound
		}
	})
	report(t, "a gate must be opened with judge.LinkTargets, the extractor the page and the check read links with", found)
	// A call that was refused above is already the answer; counting only says
	// something when the walk found no call at all.
	if len(found) == 0 && (insideJudge == 0 || outsideJudge == 0) {
		t.Fatalf("saw %d sound egress.Open calls inside the judge and %d outside it, want at least one of each, so this walk either read nothing or a command no longer opens its gate", insideJudge, outsideJudge)
	}
}

// writerUses reads how a command file uses the writer it is handed for its
// output. The only uses are writing a payload to it and passing it on to the
// method that does; a command that formatted to it itself would put out text
// no gate sealed. The second result counts the payloads written.
func writerUses(path string, fset *token.FileSet, file *ast.File) (found []site, payloads int) {
	allowed := map[*ast.Ident]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		for _, arg := range call.Args {
			identifier, ok := arg.(*ast.Ident)
			if !ok || identifier.Name != "stdout" {
				continue
			}
			switch selector.Sel.Name {
			case "WriteTo":
				allowed[identifier] = true
				payloads++
			case "run":
				allowed[identifier] = true
			}
		}
		return true
	})
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.Field:
			// A parameter's own declaration is not a use.
			for _, name := range node.Names {
				allowed[name] = true
			}
		case *ast.Ident:
			if node.Name == "stdout" && !allowed[node] {
				found = append(found, site{path: path, line: fset.Position(node.Pos()).Line, text: "stdout used other than to write a payload"})
			}
		}
		return true
	})
	return found, payloads
}

// commandFiles are the files whose commands write an agent-facing output. The
// walk is over these and not the whole command package, since the server and
// the help text have a standard output of their own to write.
var commandFiles = []string{"cmd/yomihon/adjudicate.go", "cmd/yomihon/leftopen.go"}

// unlistedCommand finds a file of the command package that opens a gate or
// runs a judge engine without being one of commandFiles. Its writer would go
// unread by the walk over those files, so the file has to be added there, where
// it is held to writing a payload and nothing else. The second result counts
// the gates opened and engines run, listed or not.
func unlistedCommand(path string, fset *token.FileSet, file *ast.File) (found []site, calls int) {
	if !strings.HasPrefix(path, "cmd/") {
		return nil, 0
	}
	names, _ := importedAs(path, fset, file)
	listed := slices.Contains(commandFiles, path)
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		switch names[qualifier.Name] + "." + selector.Sel.Name {
		case egressImport + ".Open", judgeImport + ".RunCheck", judgeImport + ".RunCoverage", judgeImport + ".RunExists":
			calls++
			if !listed {
				found = append(found, site{path: path, line: fset.Position(selector.Pos()).Line, text: "opens a gate or runs an engine, and is not in commandFiles"})
			}
		}
		return true
	})
	return found, calls
}

// TestAnAgentFacingCommandWritesNothingButASealedPayload closes the gap the
// standard-output walk leaves: main hands every command a writer, and a command
// that formatted its own text to that writer would reach standard output
// without passing a gate. In these files the writer is only ever written a
// payload, which cannot be made without the gate looking at the rules file
// again.
func TestAnAgentFacingCommandWritesNothingButASealedPayload(t *testing.T) {
	t.Parallel()

	var found []site
	payloads := map[string]int{}
	forEachProductionFile(t, func(path string, fset *token.FileSet, file *ast.File) {
		for _, command := range commandFiles {
			if path != command {
				continue
			}
			sites, written := writerUses(path, fset, file)
			found = append(found, sites...)
			payloads[path] = written
		}
		unlisted, _ := unlistedCommand(path, fset, file)
		found = append(found, unlisted...)
	})
	report(t, "an agent-facing command writes its output only as an egress.Payload, so the gate has looked at the rules file again before anything leaves", found)
	for _, command := range commandFiles {
		if payloads[command] == 0 && len(found) == 0 {
			t.Fatalf("%s writes no payload to its writer, so this walk either read nothing or the command found another way out, and the rule above guards a writer nobody uses", command)
		}
	}
}

// TestTheEgressGuardsCatchEveryShapeOfBypass runs each walk over a source that
// does what it exists to refuse, and over one that does what it permits, so a
// walk that stops matching is a red test here and not a quiet pass above.
func TestTheEgressGuardsCatchEveryShapeOfBypass(t *testing.T) {
	t.Parallel()

	type walk func(path string, fset *token.FileSet, file *ast.File) (found []site, counted int)
	asks := func(path string, fset *token.FileSet, file *ast.File) ([]site, int) {
		found, seen := withheldAsks(path, fset, file)
		return found, seen["EgressAllowed"] + seen["WithheldDirs"]
	}
	for _, tt := range []struct {
		name    string
		walk    walk
		path    string
		source  string
		refused int
		counted int
	}{
		{"asks whether a path is allowed", asks, "cmd/yomihon/a.go", `package main
func f(p policy) bool { return p.EgressAllowed("x") }`, 1, 1},
		{"asks for the withheld directories", asks, "internal/judge/a.go", `package judge
func f(p policy) []string { return p.WithheldDirs() }`, 1, 1},
		{"holds the question as a value", asks, "internal/judge/a.go", `package judge
func f(p policy) { ask := p.EgressAllowed; _ = ask }`, 1, 1},
		{"asks the gate instead", asks, "cmd/yomihon/a.go", `package main
func f(g gate) bool { return g.Allows("x") }`, 0, 0},

		{"writes to os.Stdout", standardOutputSpellings, "cmd/yomihon/a.go", `package main
import "os"
func f() { os.Stdout.WriteString("x") }`, 1, 1},
		{"writes to os.Stdout under another name", standardOutputSpellings, "cmd/yomihon/a.go", `package main
import operating "os"
func f() { operating.Stdout.WriteString("x") }`, 1, 1},
		{"takes descriptor one", standardOutputSpellings, "cmd/yomihon/a.go", `package main
import "os"
func f() { os.NewFile(1, "out").WriteString("x") }`, 1, 0},
		{"prints with fmt", standardOutputSpellings, "cmd/yomihon/a.go", `package main
import "fmt"
func f() { fmt.Println("x") }`, 1, 0},
		{"prints with a builtin", standardOutputSpellings, "cmd/yomihon/a.go", `package main
func f() { println("x") }`, 1, 0},
		{"dot-imports os", standardOutputSpellings, "cmd/yomihon/a.go", `package main
import . "os"
func f() { Stdout.WriteString("x") }`, 1, 0},
		{"writes to standard error", standardOutputSpellings, "cmd/yomihon/a.go", `package main
import ("fmt"; "os")
func f() { fmt.Fprintln(os.Stderr, "x") }`, 0, 0},

		{"opens with the judge's extractor from outside it", openCalls, "cmd/yomihon/a.go", `package main
import ("github.com/koopa0/yomihon/internal/egress"; "github.com/koopa0/yomihon/internal/judge")
func f() { egress.Open(ctx, r, judge.LinkTargets) }`, 0, 1},
		{"opens with the judge's extractor from inside it", openCalls, "internal/judge/a.go", `package judge
import "github.com/koopa0/yomihon/internal/egress"
func f() { egress.Open(ctx, r, LinkTargets) }`, 0, 1},
		{"opens with another extractor", openCalls, "cmd/yomihon/a.go", `package main
import ("github.com/koopa0/yomihon/internal/egress"; "github.com/koopa0/yomihon/internal/judge")
func f() { egress.Open(ctx, r, mine) ; _ = judge.LinkTargets }`, 1, 0},
		{"opens with an extractor of the same name elsewhere", openCalls, "cmd/yomihon/a.go", `package main
import ("github.com/koopa0/yomihon/internal/egress"; judge "example.com/other")
func f() { egress.Open(ctx, r, judge.LinkTargets) }`, 1, 0},
		{"names the extractor bare outside the judge", openCalls, "cmd/yomihon/a.go", `package main
import "github.com/koopa0/yomihon/internal/egress"
func f() { egress.Open(ctx, r, LinkTargets) }`, 1, 0},
		{"opens through an alias", openCalls, "cmd/yomihon/a.go", `package main
import (gate "github.com/koopa0/yomihon/internal/egress"; "github.com/koopa0/yomihon/internal/judge")
func f() { gate.Open(ctx, r, nil) ; _ = judge.LinkTargets }`, 1, 0},
		{"takes Open as a value", openCalls, "cmd/yomihon/a.go", `package main
import "github.com/koopa0/yomihon/internal/egress"
var open = egress.Open`, 1, 0},
		{"dot-imports egress", openCalls, "cmd/yomihon/a.go", `package main
import . "github.com/koopa0/yomihon/internal/egress"
func f() { Open(ctx, r, nil) }`, 1, 0},

		{"writes a payload", writerUses, "cmd/yomihon/a.go", `package main
func f(payload p, stdout io.Writer) { p.WriteTo(stdout) }`, 0, 1},
		{"passes the writer to the run method", writerUses, "cmd/yomihon/a.go", `package main
func f(r request, stdout io.Writer) { r.run(ctx, stdout, stderr) }`, 0, 0},
		{"formats to the writer", writerUses, "cmd/yomihon/a.go", `package main
func f(stdout io.Writer) { fmt.Fprintln(stdout, "x") }`, 1, 0},
		{"writes bytes to the writer", writerUses, "cmd/yomihon/a.go", `package main
func f(stdout io.Writer) { stdout.Write(b) }`, 1, 0},
		{"a listed command opens a gate", unlistedCommand, "cmd/yomihon/leftopen.go", `package main
import "github.com/koopa0/yomihon/internal/egress"
func f() { egress.Open(ctx, r, nil) }`, 0, 1},
		{"an unlisted file opens a gate", unlistedCommand, "cmd/yomihon/new.go", `package main
import "github.com/koopa0/yomihon/internal/egress"
func f() { egress.Open(ctx, r, nil) }`, 1, 1},
		{"an unlisted file runs an engine", unlistedCommand, "cmd/yomihon/new.go", `package main
import "github.com/koopa0/yomihon/internal/judge"
func f() { judge.RunExists(ctx, o) }`, 1, 1},
		{"a file outside the command package opens a gate", unlistedCommand, "internal/other/a.go", `package other
import "github.com/koopa0/yomihon/internal/egress"
func f() { egress.Open(ctx, r, nil) }`, 0, 0},
		{"hands the writer to a function", writerUses, "cmd/yomihon/a.go", `package main
func f(stdout io.Writer) { out(stdout) }`, 1, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, tt.path, tt.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			found, counted := tt.walk(tt.path, fset, file)
			if len(found) != tt.refused {
				t.Errorf("refused %d, want %d: %v", len(found), tt.refused, found)
			}
			if counted != tt.counted {
				t.Errorf("counted %d, want %d", counted, tt.counted)
			}
		})
	}
}
