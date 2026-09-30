package schema

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nested repeats opener n times around a leaf and closes it with closer the
// same number of times.
func nested(opener, leaf, closer string, n int) string {
	return strings.Repeat(opener, n) + leaf + strings.Repeat(closer, n)
}

// TestDecodeRefusesAContractNestedPastTheBound holds the depth guard ahead of
// the decoder, whose cost grows with the square of the depth. Each shape
// nests one step past the bound, and none would be refused for anything else
// first: without the guard the decoder reads every one of them and answers
// about the unknown table instead. The exceptions are the non-ASCII key and
// the key after a bare carriage return, which this decoder refuses on its
// own; they are here so that a decoder which accepts them cannot bring the
// cost back. The cases past the plain ones
// are the ways a string could hide the structure around it if the walk misread
// where it ends.
func TestDecodeRefusesAContractNestedPastTheBound(t *testing.T) {
	t.Parallel()
	past := maxContractDepth + 1
	for _, tc := range []struct {
		name  string
		extra string
	}{
		{"inline tables", "[extra]\nx = " + nested("{a = ", "1", "}", past) + "\n"},
		{"arrays", "[extra]\nx = " + nested("[", "1", "]", past) + "\n"},
		{"a dotted key", "[extra]\n" + strings.Repeat("a.", past) + "a = 1\n"},
		{"a dotted key with spaces", "[extra]\n" + strings.Repeat("a . ", past) + "a = 1\n"},
		{"a quoted dotted key", "[extra]\n" + strings.Repeat(`"a".`, past) + `"a" = 1` + "\n"},
		{"a non-ASCII dotted key", "[extra]\n" + strings.Repeat("é.", past) + "é = 1\n"},
		{"a table header", "[" + strings.Repeat("a.", past) + "a]\nx = 1\n"},
		{"closers inside basic strings", "[extra]\nx = " + nested(`{s = "]}", a = `, "1", "}", past) + "\n"},
		{"closers inside multi-line strings", "[extra]\nx = " + nested(`{s = """]}""", a = `, "1", "}", past) + "\n"},
		{"a quote run closing a multi-line string", "[extra]\nx = " + nested(`{s = """]}""""", a = `, "1", "}", past) + "\n"},
		{"a quote run closing a multi-line literal", "[extra]\nx = " + nested(`{s = ''']}''''', a = `, "1", "}", past) + "\n"},
		{"four quotes closing a multi-line string", "[extra]\nx = " + nested(`{s = """]}"""", a = `, "1", "}", past) + "\n"},
		{"four quotes closing a multi-line literal", "[extra]\nx = " + nested(`{s = ''']}'''', a = `, "1", "}", past) + "\n"},
		{"a backslash ending a literal string", "[extra]\nx = " + nested(`{s = '\', a = `, "1", "}", past) + "\n"},
		{"an escaped quote inside a basic string", "[extra]\nx = " + nested(`{s = "\"]}", a = `, "1", "}", past) + "\n"},
		{"closers inside comments", "[extra]\nx = " + nested("[ # ]]]]\n", "1", "]", past) + "\n"},
		// Depth and key length each within their bound, multiplied past it:
		// the decoder's cost follows the whole path, not either part alone.
		{"a long key opening each inline table", "[extra]\nx = " + nested("{"+strings.Repeat("k.", 4)+"k = ", "1", "}", 8) + "\n"},
		{"a long header above a long key", "[" + strings.Repeat("h.", 19) + "h]\n" + strings.Repeat("k.", 19) + "k = 1\n"},
		{"inline tables one past the path", "[extra]\ny = " + nested("{a = ", "1", "}", maxContractPath-1) + "\n"},
		{"an array of inline tables with long keys", "[extra]\nx = [" + nested("{"+strings.Repeat("k.", 4)+"k = ", "1", "}", 8) + "]\n"},
		// A path of few parts can still be long in bytes, and the decoder copies
		// every byte of it for each key read beneath it.
		{"a long header part", "[extra." + strings.Repeat("p", maxContractPathBytes) + "]\nk = 1\n"},
		{"a long quoted header part", `[extra."` + strings.Repeat("p", maxContractPathBytes) + `"]` + "\nk = 1\n"},
		{"long keys opening each inline table", "[extra]\nx = " + nested("{"+strings.Repeat("k", 40)+" = ", "1", "}", 7) + "\n"},
		{"a long key opening an inline table", "[extra]\nx = { " + strings.Repeat("k", maxContractPathBytes) + " = { a = 1 } }\n"},
		{"a long quoted key", "[extra]\n\"" + strings.Repeat("k", maxContractPathBytes) + "\" = 1\n"},
		{"a key after a bare carriage return", "[extra]\nx = 1\r" + strings.Repeat("k", maxContractPathBytes) + " = { b = 1 }\n"},
		{"long parts within the part bound", "[extra]\n" + strings.Repeat(strings.Repeat("k", 32)+".", 8) + "k = 1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := decodeContract([]byte(validContractV1+"\n"+tc.extra), policySource{})
			if err == nil {
				t.Fatal("decodeContract() error = nil, want the depth refusal")
			}
			if got := classifyDecodeError(err); got != "too-deep" {
				t.Errorf("classifyDecodeError() = %q, want %q (error was %v)", got, "too-deep", err)
			}
		})
	}
}

// TestDecodeKeepsAContractWithinTheBound is the guard's other side. Brackets
// and dots written inside a string or a comment count for nothing, a contract
// may nest right up to the bound, and every contract this repository keeps
// decodes past the guard.
func TestDecodeKeepsAContractWithinTheBound(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		contract string
	}{
		{"brackets in a string", strings.Replace(validContractV1,
			`aligned_with = "Note-Contract.md"`,
			`aligned_with = "`+nested("[{", "x", "}]", 200)+`"`, 1)},
		{"dots in a string", strings.Replace(validContractV1,
			`aligned_with = "Note-Contract.md"`,
			`aligned_with = "`+strings.Repeat("a.", 200)+`md"`, 1)},
		{"brackets and dots in a comment", "# " + nested("[{", strings.Repeat("a.", 200), "}]", 200) + "\n" + validContractV1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := checkContractDepth([]byte(tc.contract)); err != nil {
				t.Errorf("checkContractDepth() = %v, want nil", err)
			}
			if _, err := decodeContract([]byte(tc.contract), policySource{}); err != nil {
				t.Errorf("decodeContract() = %v, want nil", err)
			}
		})
	}

	// At the bounds exactly: arrays as deep as allowed, a key whose parts or
	// bytes and its header's fill the path, and inline tables whose innermost
	// key does.
	for name, atBound := range map[string]string{
		"arrays":        "[extra]\nx = " + nested("[", "1", "]", maxContractDepth) + "\n",
		"a dotted key":  "[extra]\n" + strings.Repeat("a.", maxContractPath-2) + "a = 1\n",
		"inline tables": "[extra]\ny = " + nested("{a = ", "1", "}", maxContractPath-2) + "\n",
		"floats":        "[extra]\nx = " + nested("[", "1.5, 2.5", "]", maxContractDepth) + "\n",
		"a long key":    "[extra]\n" + strings.Repeat("k", maxContractPathBytes-len("extra")) + "=1\n",
		// A colon or a sign inside a value does not end it, so the dot after
		// one is still a fraction, not a key part.
		"a time and a signed float": "[extra]\ny = " + nested("{a = ", "07:32:00.5, b = +1.5", "}", maxContractPath-2) + "\n",
	} {
		if err := checkContractDepth([]byte(atBound)); err != nil {
			t.Errorf("checkContractDepth() with %s at the bound = %v, want nil", name, err)
		}
	}

	// Every TOML file in the repository is a contract, so the set is found by
	// walking the tree rather than listed by pattern: a pattern misses the
	// contract kept one directory deeper than it looks.
	var contracts []string
	err := filepath.WalkDir(filepath.Join("..", ".."), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !entry.IsDir() && filepath.Ext(path) == ".toml" {
			contracts = append(contracts, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(contracts) < 10 {
		t.Fatalf("found %d contracts in the repository, want the whole set", len(contracts))
	}
	for _, path := range contracts {
		data, err := os.ReadFile(path) // #nosec G304 -- the repository's own tracked contracts
		if err != nil {
			t.Fatal(err)
		}
		if err := checkContractDepth(data); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}

// TestDecodeRefusesAContractPastTheSizeBound holds the second bound. The path
// bound keeps each key the decoder reads cheap; this one keeps the number of
// keys finite, whatever shape a future decoder turns out to be slow on.
func TestDecodeRefusesAContractPastTheSizeBound(t *testing.T) {
	t.Parallel()
	padding := strings.Repeat("# a comment line that only takes up room\n", maxContractBytes/40)
	_, err := decodeContract([]byte(validContractV1+padding), policySource{})
	if err == nil {
		t.Fatal("decodeContract() error = nil, want the size refusal")
	}
	if got := classifyDecodeError(err); got != "too-large" {
		t.Errorf("classifyDecodeError() = %q, want %q (error was %v)", got, "too-large", err)
	}
}
