package schema

import (
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
// about the unknown table instead. The one exception is the non-ASCII key,
// which this decoder refuses on its own; it is here so that a decoder which
// accepts such keys cannot bring the cost back. The cases past the plain ones
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
		{"a backslash ending a literal string", "[extra]\nx = " + nested(`{s = '\', a = `, "1", "}", past) + "\n"},
		{"an escaped quote inside a basic string", "[extra]\nx = " + nested(`{s = "\"]}", a = `, "1", "}", past) + "\n"},
		{"closers inside comments", "[extra]\nx = " + nested("[ # ]]]]\n", "1", "]", past) + "\n"},
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

	atBound := "[extra]\nx = " + nested("[", "1", "]", maxContractDepth) + "\n" +
		strings.Repeat("a.", maxContractDepth) + "a = 1\n"
	if err := checkContractDepth([]byte(atBound)); err != nil {
		t.Errorf("checkContractDepth() at the bound = %v, want nil", err)
	}

	contracts, err := filepath.Glob(filepath.Join("..", "..", "*", "*", "System", "schemas", "vault-schema.toml"))
	if err != nil {
		t.Fatal(err)
	}
	more, err := filepath.Glob(filepath.Join("..", "..", "internal", "*", "testdata", "*", "System", "schemas", "vault-schema.toml"))
	if err != nil {
		t.Fatal(err)
	}
	contracts = append(contracts, more...)
	tomls, err := filepath.Glob(filepath.Join("..", "..", "internal", "*", "testdata", "*.toml"))
	if err != nil {
		t.Fatal(err)
	}
	contracts = append(contracts, tomls...)
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
