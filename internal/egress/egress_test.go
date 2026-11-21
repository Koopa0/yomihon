package egress_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// A rules file for the gate to read: the loader's own fixture with a privacy
// section appended, so what the gate withholds is the only thing a test varies.
func rulesFile(t *testing.T, withheld string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read the rules fixture: %v", err)
	}
	return string(data) + "\n[privacy]\nnever_egress_dirs = [" + withheld + "]\n"
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("make the folder of %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil { // #nosec G703 -- test fixture paths are owned by each caller's temporary root
		t.Fatalf("write %s: %v", rel, err)
	}
}

// openFolder reads a folder and closes it when the test ends.
func openFolder(t *testing.T, root string) *vault.Reader {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("open the folder: %v", err)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Errorf("close the folder: %v", err)
		}
	})
	return reader
}

// open opens a gate on a folder whose rules file, if any, is the one given. The
// extractor is a plain pattern: what these tests hold is how the gate uses it.
func open(t *testing.T, rules string) (gate *egress.Gate, root string) {
	t.Helper()
	root = t.TempDir()
	writeFile(t, root, "Notes/Plain.md", "# Plain\n")
	if rules != "" {
		writeFile(t, root, schema.ContractRelPath, rules)
	}
	gate, err := egress.Open(t.Context(), openFolder(t, root), wikilinks)
	if err != nil {
		t.Fatalf("egress.Open() error = %v", err)
	}
	return gate, root
}

var wikilinkPattern = regexp.MustCompile(`\[\[([^\]|#^]+)`)

func wikilinks(text string) []string {
	var targets []string
	for _, match := range wikilinkPattern.FindAllStringSubmatch(text, -1) {
		targets = append(targets, match[1])
	}
	return targets
}

func TestAFolderWithNoRulesFileWithholdsNothing(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, "")
	if !gate.WithoutRules() {
		t.Error("WithoutRules() = false for a folder with no rules file")
	}
	if !gate.Allows("Notes/Plain.md") {
		t.Error("Allows(Notes/Plain.md) = false, want true: nothing was declared, so nothing is withheld")
	}
	for _, rel := range []string{"", "../Outside.md", "/abs.md", `Back\slash.md`} {
		if gate.Allows(rel) {
			t.Errorf("Allows(%q) = true, want false: it is not a path inside the vault", rel)
		}
	}
}

func TestTheRulesFileDecidesWhatMayBeSaid(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, rulesFile(t, `"Private"`))
	if gate.WithoutRules() {
		t.Error("WithoutRules() = true for a folder with a rules file")
	}
	for rel, want := range map[string]bool{
		"Notes/Plain.md":      true,
		"Private/Secret.md":   false,
		"private/secret.md":   false,
		"Private":             false,
		"PrivateNotes/A.md":   true,
		"Notes/Private/A.md":  true,
		"Private/Deep/B.md":   false,
		"Private/../Notes.md": true,
	} {
		if got := gate.Allows(rel); got != want {
			t.Errorf("Allows(%q) = %v, want %v", rel, got, want)
		}
	}
}

// Allowed is Allows for a caller with no gate: the same answer for a folder that
// has a rules file or none, and no for one whose rules file cannot be honoured.
func TestAllowedAnswersAsTheGateDoesAndNoWhenTheRulesFileCannotBeHonoured(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		rules string
		rel   string
		want  bool
	}{
		{"no rules file", "", "README.md", true},
		{"a rules file that withholds another folder", rulesFile(t, `"Private"`), "README.md", true},
		{"a rules file that withholds the file", rulesFile(t, `"README.md"`), "README.md", false},
		{"a rules file that withholds a folder above it", rulesFile(t, `"Private"`), "Private/README.md", false},
		{"a rules file that declares no privacy", strings.TrimSuffix(rulesFile(t, `"Private"`), "\n[privacy]\nnever_egress_dirs = [\"Private\"]\n"), "README.md", false},
		{"a rules file that still declares a version", "schema_version = \"1\"\n" + rulesFile(t, `"Private"`), "README.md", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeFile(t, root, "README.md", "# Title\n")
			if test.rules != "" {
				writeFile(t, root, schema.ContractRelPath, test.rules)
			}
			if got := egress.Allowed(t.Context(), openFolder(t, root), test.rel); got != test.want {
				t.Errorf("Allowed(%q) = %v, want %v", test.rel, got, test.want)
			}
		})
	}
}

// TestSealRefusesOnceTheRulesFileIsNotTheOneItWasOpenedOn is the promise: an
// output is assembled under the rules file the command began with, so it may
// hold what the rules file as it now stands withholds, and the seal is where
// that is found.
func TestSealRefusesOnceTheRulesFileIsNotTheOneItWasOpenedOn(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		rules  string
		change func(t *testing.T, root string)
	}{
		{"rules file rewritten", rulesFile(t, `"Private"`), func(t *testing.T, root string) {
			t.Helper()
			writeFile(t, root, schema.ContractRelPath, rulesFile(t, ""))
		}},
		{"rules file removed", rulesFile(t, `"Private"`), func(t *testing.T, root string) {
			t.Helper()
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))); err != nil {
				t.Fatalf("remove the rules file: %v", err)
			}
		}},
		{"rules file appears", "", func(t *testing.T, root string) {
			t.Helper()
			writeFile(t, root, schema.ContractRelPath, rulesFile(t, `"Private"`))
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gate, root := open(t, tt.rules)
			if _, err := gate.Seal([]byte("before")); err != nil {
				t.Fatalf("Seal() before the change error = %v", err)
			}
			tt.change(t, root)
			payload, err := gate.Seal([]byte("Private/Secret.md"))
			if !errors.Is(err, egress.ErrUnavailable) {
				t.Fatalf("Seal() after the change error = %v, want a refusal", err)
			}
			var out bytes.Buffer
			if _, err := payload.WriteTo(&out); err != nil || out.Len() != 0 {
				t.Errorf("the refused payload wrote %q, %v; want nothing", out.String(), err)
			}
		})
	}
}

func TestAPayloadWritesWhatWasSealedAndNothingElse(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, rulesFile(t, `"Private"`))
	data := []byte("the output")
	payload, err := gate.Seal(data)
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	copy(data, "CHANGED!!!")
	var out bytes.Buffer
	if n, err := payload.WriteTo(&out); err != nil || n != int64(len("the output")) {
		t.Fatalf("WriteTo() = %d, %v", n, err)
	}
	if got := out.String(); got != "the output" {
		t.Errorf("payload wrote %q after the caller changed its slice, want the sealed bytes", got)
	}

	var empty egress.Payload
	out.Reset()
	if n, err := empty.WriteTo(&out); err != nil || n != 0 || out.Len() != 0 {
		t.Errorf("the zero Payload wrote %d bytes, %v, %q; want nothing", n, err, out.String())
	}
}

func TestOpenRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()

	t.Run("a rules file that still declares a version", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeFile(t, root, schema.ContractRelPath, "schema_version = \"1\"\n"+rulesFile(t, `"Private"`))
		reader := openFolder(t, root)
		_, err := egress.Open(t.Context(), reader, wikilinks)
		if !errors.Is(err, egress.ErrUnavailable) {
			t.Fatalf("Open() error = %v, want ErrUnavailable", err)
		}
		if strings.Contains(err.Error(), "schema_version") {
			t.Errorf("the refusal quotes the rules file: %v", err)
		}
	})

	t.Run("a rules file that does not decode", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeFile(t, root, schema.ContractRelPath, "[malformed\n")
		reader := openFolder(t, root)
		_, err := egress.Open(t.Context(), reader, wikilinks)
		if !errors.Is(err, egress.ErrUnavailable) {
			t.Fatalf("Open() error = %v, want ErrUnavailable", err)
		}
		if strings.Contains(err.Error(), "malformed") {
			t.Errorf("the refusal quotes the rules file: %v", err)
		}
	})

	t.Run("a rules file without a privacy section", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, root, schema.ContractRelPath, string(data))
		reader := openFolder(t, root)
		_, err = egress.Open(t.Context(), reader, wikilinks)
		if !errors.Is(err, egress.ErrUndeclared) || !errors.Is(err, egress.ErrUnavailable) {
			t.Fatalf("Open() error = %v, want ErrUndeclared, which is a case of ErrUnavailable", err)
		}
	})

	t.Run("no extractor", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if recover() == nil {
				t.Error("Open() with no link extractor did not panic")
			}
		}()
		if _, err := egress.Open(t.Context(), nil, nil); err != nil {
			t.Errorf("Open() error = %v", err)
		}
	})
}

// names is a vault of titled notes, for the gate to match a spelled link name
// against: a name reaches the note whose title it is. Notes in outside are
// ones the walk left out by its own rules.
type names struct {
	byName  map[string]string
	outside map[string]bool
}

func (n names) OutsideScan(rel string) bool { return n.outside[rel] }

func (n names) NameWithin(target string, allowed func(rel string) bool) bool {
	rel, known := n.byName[target]
	return !known || allowed(rel)
}

func TestSpellsFindsAWithheldNameInTheWordsOfATitle(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, rulesFile(t, `"Private"`))
	vaultNames := names{byName: map[string]string{
		"Secret Plan": "Private/Secret Plan.md",
		"Plain":       "Notes/Plain.md",
	}}
	for _, tt := range []struct {
		name string
		text string
		body string
		want bool
	}{
		{"a wikilink to a withheld note", "Reading [[Secret Plan]]", "", true},
		{"a wikilink to a readable note", "Reading [[Plain]]", "", false},
		{"a wikilink with an alias", "Reading [[Secret Plan|the plan]]", "", true},
		{"a path into a withheld folder", "see Private/Secret Plan.md", "", true},
		{"a path into a withheld folder in another case", "see private/x", "", true},
		{"a folder that only starts the same", "see PrivateNotes", "", false},
		{"a link the page would not render, in a code span", "`[[Secret Plan]]`", "", true},
		{"an inline link to a withheld note", "see [it](Private/Secret Plan.md)", "", true},
		{"words that cite a withheld name the body links", "Secret Plan", "[[Secret Plan]]", true},
		{"words that cite a readable name the body links", "Plain", "[[Plain]]", false},
		{"nothing cited", "A heading", "no links", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := gate.Spells(tt.text, tt.body, vaultNames); got != tt.want {
				t.Errorf("Spells(%q, %q) = %v, want %v", tt.text, tt.body, got, tt.want)
			}
		})
	}
}

func TestSpellsReadsLinksWithTheExtractorTheGateWasOpenedWith(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, schema.ContractRelPath, rulesFile(t, `"Private"`))
	reader := openFolder(t, root)
	// An extractor that reads nothing leaves the raw reader alone to find the
	// link; one that reads a title the raw reader would not shows the body
	// citation is found by the extractor and not by the text.
	called := 0
	gate, err := egress.Open(t.Context(), reader, func(text string) []string {
		called++
		return []string{"Secret Plan"}
	})
	if err != nil {
		t.Fatal(err)
	}
	vaultNames := names{byName: map[string]string{"Secret Plan": "Private/Secret Plan.md"}}
	if !gate.Spells("Secret Plan", "any body at all", vaultNames) {
		t.Error("Spells() = false, want true: the body's extractor names the withheld note and the words cite it")
	}
	if called == 0 {
		t.Error("the extractor the gate was opened with was never asked")
	}

	// A title whose only link the extractor reads and the plain-text reader
	// does not: the words themselves carry no link syntax.
	if !gate.Spells("no link syntax in these words", "", vaultNames) {
		t.Error("Spells() = false, want true: the extractor reads a link out of the title and it reaches withheld ground")
	}
}

func TestReadableAsksWhatTheScanLeftOutAsWellAsWhatIsWithheld(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, rulesFile(t, `"Private"`))
	readable := gate.Readable(names{outside: map[string]bool{"Notes/Ignored.md": true}})
	for rel, want := range map[string]bool{
		"Notes/Plain.md":   true,
		"Notes/Ignored.md": false,
		"Private/A.md":     false,
	} {
		if got := readable(rel); got != want {
			t.Errorf("Readable()(%q) = %v, want %v", rel, got, want)
		}
	}
}
