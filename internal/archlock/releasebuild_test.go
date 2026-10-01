package archlock

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// releaseWorkflow is the workflow that builds the binaries a reader downloads.
const releaseWorkflow = ".github/workflows/release.yml"

// TestTheReleaseBuildTrimsPathsAndStripsSymbols holds the published binaries to
// the two flags that make them what a reader can check and carry.
//
// Without -trimpath every binary records the absolute directory it was built in,
// so two clones of one commit in different directories produce different
// SHA-256 values, and nobody can rebuild a release from a checkout of their own
// to the digest that was published. Without -ldflags="-s -w" the binary carries
// the symbol table and DWARF data, about a quarter of its size, which a reader
// never uses; a panic still prints its file and line and `go version -m` still
// reads the build information without them.
//
// The check reads every `go build` the workflow runs and asks each for both
// flags, rather than looking for the one line that ships today: a second build
// added to the workflow without them is the same mistake. A workflow in which it
// finds no `go build` fails too, because then the check would be reading nothing
// and would pass whatever the release did.
//
// It reads the file as text, one shell command at a time, and sees a build only
// when `go` and `build` are two words of their own. A build spelled any other
// way (inside `$(...)`, through a variable standing for go, as `go -C dir
// build`, as `go install`, or as a whole command quoted into a one-line YAML
// string) is not seen, and fails the check when it is the only one. Prose
// outside a comment that spells the two words, a step name for instance, is read
// as a build and fails it the other way.
func TestTheReleaseBuildTrimsPathsAndStripsSymbols(t *testing.T) {
	t.Parallel()

	builds := releaseBuilds(t)
	if len(builds) == 0 {
		t.Fatalf("%s holds no `go build` this check can read, so there is no release build for it to hold; if the build moved or is spelled another way, move this check with it", releaseWorkflow)
	}
	for _, build := range builds {
		if missing := missingReleaseFlags(build.args); len(missing) > 0 {
			t.Errorf("%s:%d: this `go build` lacks %s (-trimpath keeps the runner's checkout path out of the binary; -s -w drops symbol data nobody reads)\n\tbuild: go build %q",
				releaseWorkflow, build.line, strings.Join(missing, " and "), build.args)
		}
	}
}

// TestTheReleaseBuildCheckReadsTheCommandNotItsSpelling pins how the check above
// reads a script, so a build that is spelled differently is neither missed nor
// mistaken for one that carries the flags.
func TestTheReleaseBuildCheckReadsTheCommandNotItsSpelling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		script      string
		wantBuilds  int
		wantMissing []string // what the last build found lacks; a row with one build has one answer
	}{
		{
			name:       "both flags on the line",
			script:     `CGO_ENABLED=0 GOOS="$goos" go build -trimpath -ldflags="-s -w" -o "$out" ./cmd/yomihon`,
			wantBuilds: 1,
		},
		{
			name:       "the flags in either order, the ldflags value as a separate word",
			script:     `go build -ldflags '-w -s' -trimpath -o out ./cmd/yomihon`,
			wantBuilds: 1,
		},
		{
			name:       "more linker flags beside the two",
			script:     `go build -trimpath -ldflags="-s -w -X main.version=v1" ./cmd/yomihon`,
			wantBuilds: 1,
		},
		{
			name:       "the flags on a continued line",
			script:     "go build \\\n  -trimpath \\\n  -ldflags=\"-s -w\" \\\n  ./cmd/yomihon",
			wantBuilds: 1,
		},
		{
			name:       "CRLF line endings, the flags on a continued line",
			script:     "go build \\\r\n  -trimpath \\\r\n  -ldflags=\"-s -w\" \\\r\n  ./cmd/yomihon\r\n",
			wantBuilds: 1,
		},
		{
			name:        "no flags",
			script:      `go build -o "$out" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:        "only -trimpath",
			script:      `go build -trimpath -o "$out" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{`-ldflags="-s -w"`},
		},
		{
			name:        "only -ldflags",
			script:      `go build -ldflags="-s -w" -o "$out" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "-ldflags that strips only one of the two",
			script:      `go build -trimpath -ldflags="-s" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{`-ldflags="-s -w"`},
		},
		{
			name:        "the last -ldflags wins, as it does for go",
			script:      `go build -trimpath -ldflags="-s -w" -ldflags="-X main.version=v1" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{`-ldflags="-s -w"`},
		},
		{
			name:        "-trimpath switched off",
			script:      `go build -trimpath=false -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "-trimpath given a value go cannot read",
			script:      `go build -trimpath=maybe -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "a later -trimpath=false switches -trimpath off",
			script:      `go build -trimpath -trimpath=false -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "a later -trimpath=0 switches -trimpath off",
			script:      `go build -trimpath -trimpath=0 -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "a later -trimpath=f switches -trimpath off",
			script:      `go build -trimpath -trimpath=f -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "a later -trimpath=F switches -trimpath off",
			script:      `go build -trimpath -trimpath=F -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "a later -trimpath=FALSE switches -trimpath off",
			script:      `go build -trimpath -trimpath=FALSE -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "a later -trimpath=False switches -trimpath off",
			script:      `go build -trimpath -trimpath=False -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:       "a later -trimpath switches it back on",
			script:     `go build -trimpath=false -trimpath -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds: 1,
		},
		{
			name:       "-trimpath given any spelling of true",
			script:     `go build -trimpath=TRUE -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds: 1,
		},
		{
			name:        "flags that belong to the next command",
			script:      `go build ./cmd/yomihon && go vet -trimpath -ldflags="-s -w" ./...`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:        "flags that belong to the next command, after a semicolon with no space",
			script:      `go build ./cmd/yomihon;go vet -trimpath -ldflags="-s -w" ./...`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:        "flags that belong to the next command, after && with no space",
			script:      `go build ./cmd/yomihon&&go vet -trimpath -ldflags="-s -w" ./...`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:        "flags that belong to the next command in a pipe, with no space",
			script:      `go build ./cmd/yomihon|tee -trimpath -ldflags="-s -w"`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:        "flags that belong to the next command, after a background &",
			script:      `go build ./cmd/yomihon& go vet -trimpath -ldflags="-s -w" ./...`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:       "a redirection is not the end of the command",
			script:     `go build -o out ./cmd/yomihon 2>&1 -trimpath -ldflags="-s -w"`,
			wantBuilds: 1,
		},
		{
			name:        "flags that sit in a trailing comment",
			script:      `go build -o out ./cmd/yomihon # -trimpath -ldflags="-s -w"`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:        "flags that sit in a comment after a semicolon with no space",
			script:      `go build -o out ./cmd/yomihon;# -trimpath -ldflags="-s -w"`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:       "both flags, then a comment",
			script:     `go build -trimpath -ldflags="-s -w" ./cmd/yomihon # the release binary`,
			wantBuilds: 1,
		},
		{
			name:       "a # inside a word or a quoted string is not a comment",
			script:     `go build -trimpath -o out#1 -ldflags="-s -w -X main.tag=a #b" ./cmd/yomihon`,
			wantBuilds: 1,
		},
		{
			name:        "a build on the line after a comment that ends in a backslash",
			script:      "# the release build \\\ngo build -o out ./cmd/yomihon",
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:        "a trailing comment that ends in a backslash does not join the next line",
			script:      "go build -trimpath -ldflags=\"-s -w\" ./cmd/yomihon # published binary \\\ngo build -o dist/extra ./cmd/yomihon",
			wantBuilds:  2,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:        "an escaped backslash at the end of a line does not join the next line",
			script:      "echo a\\\\\ngo build -o out ./cmd/yomihon",
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:        "-ldflags takes the next word as its value, so -trimpath there is not the flag",
			script:      `go build -ldflags -trimpath -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "-ldflags given -s as its value does not also get -w",
			script:      `go build -trimpath -ldflags -s -w ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{`-ldflags="-s -w"`},
		},
		{
			name:        "-o takes the next word as its value, so -trimpath there is not the flag",
			script:      `go build -o -trimpath -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:       "the output named before the flags, the ldflags value as a separate word",
			script:     `go build -o out -trimpath -ldflags "-s -w" ./cmd/yomihon`,
			wantBuilds: 1,
		},
		{
			name:       "words that only mention a build",
			script:     "# go build -o out ./cmd/yomihon\necho \"go build -o out\"\ngo test ./...",
			wantBuilds: 0,
		},
		{
			name:       "no build at all",
			script:     "go vet ./...",
			wantBuilds: 0,
		},
		{
			name:       "a build the reader does not see: command substitution",
			script:     `bin=$(go build -o out ./cmd/yomihon)`,
			wantBuilds: 0,
		},
		{
			name:       "a build the reader does not see: go held in a variable",
			script:     `"$GO" build -o out ./cmd/yomihon`,
			wantBuilds: 0,
		},
		{
			name:       "a build the reader does not see: go -C",
			script:     `go -C cmd/yomihon build -o out`,
			wantBuilds: 0,
		},
		{
			name:       "a build the reader does not see: go install",
			script:     `go install ./cmd/yomihon`,
			wantBuilds: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builds := goBuilds(tt.script)
			if len(builds) != tt.wantBuilds {
				t.Fatalf("goBuilds(%q) found %d build(s), want %d", tt.script, len(builds), tt.wantBuilds)
			}
			if len(builds) == 0 {
				return
			}
			got := missingReleaseFlags(builds[len(builds)-1].args)
			if !slices.Equal(got, tt.wantMissing) {
				t.Errorf("missingReleaseFlags(%q) = %q, want %q", tt.script, got, tt.wantMissing)
			}
		})
	}
}

// TestTheReleaseBuildCheckNamesTheLineItReads pins the line a failure points at:
// the one a command starts on, counted in the file as written, whatever the line
// endings and however many lines a continuation spans.
func TestTheReleaseBuildCheckNamesTheLineItReads(t *testing.T) {
	t.Parallel()

	script := "name: Build\r\n# go build ./not-this\r\nrun: |\r\n  set -e\r\n  go build \\\r\n    -o out ./cmd/yomihon\r\n  go build -trimpath ./cmd/other\r\n"
	var got []int
	for _, build := range goBuilds(script) {
		got = append(got, build.line)
	}
	if want := []int{5, 7}; !slices.Equal(got, want) {
		t.Errorf("goBuilds reported builds on lines %v, want %v", got, want)
	}
}

// releaseBuild is one `go build` in a file, as the check reads it.
type releaseBuild struct {
	line int // the line the command starts on, counted from 1
	args []string
}

// releaseBuilds reads every `go build` out of the release workflow.
func releaseBuilds(t *testing.T) []releaseBuild {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(releaseWorkflow))) // #nosec G304 -- a fixed path under the repository root
	if err != nil {
		t.Fatalf("read %s: %v", releaseWorkflow, err)
	}
	return goBuilds(string(data))
}

// goBuilds returns each `go build` a file runs, with the arguments up to the
// operator that ends the command. A carriage return is not part of a line, a
// comment is not a command, a line whose last character is an unquoted,
// uncommented backslash is one command with the line after it, and a quoted
// string is one word, so text that only mentions a build is not read as one.
// Whether a line continues is the tokenizer's answer and not a look at the raw
// text, because a backslash that ends a comment, or that an earlier backslash
// escaped, does not join anything.
func goBuilds(text string) []releaseBuild {
	var builds []releaseBuild
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	for i := 0; i < len(lines); i++ {
		start, command := i+1, lines[i]
		words, continued := shellWords(command)
		for continued && i+1 < len(lines) {
			i++
			command = strings.TrimSuffix(command, `\`) + " " + lines[i]
			words, continued = shellWords(command)
		}
		for w := 0; w+1 < len(words); w++ {
			if words[w] != "go" || words[w+1] != "build" {
				continue
			}
			args := words[w+2:]
			if end := slices.IndexFunc(args, isShellOperator); end >= 0 {
				args = args[:end]
			}
			builds = append(builds, releaseBuild{line: start, args: args})
		}
	}
	return builds
}

func isShellOperator(word string) bool {
	switch word {
	case ";", "&&", "||", "|", "&":
		return true
	}
	return false
}

// shellWords splits a line the way a shell does for the purposes above:
// whitespace separates words, single and double quotes group and are removed, a
// backslash outside single quotes escapes the next character, an unquoted ;, &
// or | (or && or ||) is a word of its own whether or not a space surrounds it,
// and an unquoted word that begins with # ends the line. A & that follows a >
// or < is a redirection and stays in its word.
//
// continued reports that the line ended on a backslash that was still waiting
// for the character it escapes, which is the newline the shell then removes. A
// comment ends the line before any such backslash is read, and a backslash an
// earlier one escaped is a character of its own, so neither continues the line.
func shellWords(line string) (words []string, continued bool) {
	var (
		word   strings.Builder
		inWord bool
		quote  rune
		escape bool
	)
	flush := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case escape:
			word.WriteRune(r)
			escape = false
		case r == '\\' && quote != '\'':
			escape, inWord = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			flush()
		case r == '#' && !inWord:
			return words, false
		case r == '&' && (strings.HasSuffix(word.String(), ">") || strings.HasSuffix(word.String(), "<")):
			word.WriteRune(r)
		case r == ';' || r == '&' || r == '|':
			flush()
			operator := string(r)
			if r != ';' && i+1 < len(runes) && runes[i+1] == r {
				operator += string(r)
				i++
			}
			words = append(words, operator)
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	flush()
	return words, escape
}

// buildFlagsWithValue are the flags `go build` reads a value for, as `go help
// build` lists them and with -o, which it documents beside them. The boolean
// ones (-a, -race, -trimpath and the rest) take no value and are not here.
var buildFlagsWithValue = []string{
	"-C", "-asmflags", "-buildmode", "-compiler", "-covermode", "-coverpkg",
	"-gccgoflags", "-gcflags", "-installsuffix", "-ldflags", "-mod", "-modfile",
	"-o", "-overlay", "-p", "-pgo", "-pkgdir", "-tags", "-toolexec",
}

// missingReleaseFlags names, in the spelling the workflow uses, each flag the
// release build is required to carry and does not. -trimpath counts when the
// last one given is on, and go reads its value the way strconv.ParseBool does,
// so 0, f, F, FALSE and False switch it off as false does; a value go cannot
// read stops the build and earns no credit. -ldflags counts only when its value
// strips both the symbol table (-s) and the DWARF data (-w); when the flag is
// given twice the last one is the one go uses. A flag that takes a value and is
// not given one with `=` takes the next word, even one that begins with a dash,
// so that word is a value and never a flag of its own.
func missingReleaseFlags(args []string) []string {
	var (
		trimmed bool
		ldflags []string
	)
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "-trimpath":
			trimmed = true
		case strings.HasPrefix(arg, "-trimpath="):
			on, err := strconv.ParseBool(strings.TrimPrefix(arg, "-trimpath="))
			trimmed = err == nil && on
		case strings.HasPrefix(arg, "-ldflags="):
			ldflags = strings.Fields(strings.TrimPrefix(arg, "-ldflags="))
		case slices.Contains(buildFlagsWithValue, arg) && i+1 < len(args):
			i++
			if arg == "-ldflags" {
				ldflags = strings.Fields(args[i])
			}
		}
	}

	var missing []string
	if !trimmed {
		missing = append(missing, "-trimpath")
	}
	if !slices.Contains(ldflags, "-s") || !slices.Contains(ldflags, "-w") {
		missing = append(missing, `-ldflags="-s -w"`)
	}
	return missing
}
