package note

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
)

func TestThoughtMarkdownContainsOnlyDeterminedFrontmatterAndSource(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t, "", "")
	for _, sourceType := range []string{"", "lesson"} {
		t.Run(sourceType, func(t *testing.T) {
			t.Parallel()
			source := &snapshot.Reading{
				RelPath: "Sources/Book.md", Type: sourceType, Domain: "japanese",
				Title: "Do not copy this title", Body: "Do not copy this prompt or answer",
			}
			got := thoughtMarkdown(contract, contract.AnswerType(), source, "chapter-one")
			const want = "---\ntype: \"writing\"\nstatus: \"draft\"\ndomain: \"japanese\"\nbased_on: \"[[Sources/Book#chapter-one]]\"\n---\n"
			if got != want {
				t.Errorf("thoughtMarkdown() = %q, want %q", got, want)
			}
		})
	}
}

func TestThoughtMarkdownOmitsMultipleInitialStates(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t,
		"status = \"ready\"\napplies_to = [\"writing\", \"lesson\"]\ninitial = false",
		"status = \"ready\"\napplies_to = [\"writing\", \"lesson\"]\ninitial = true")
	got := thoughtMarkdown(contract, contract.AnswerType(), &snapshot.Reading{RelPath: "Source.md"}, "")
	const want = "---\ntype: \"writing\"\nbased_on: \"[[Source]]\"\n---\n"
	if got != want {
		t.Errorf("thoughtMarkdown() = %q, want status omitted with multiple initial states", got)
	}
}

func TestThoughtMarkdownOmitsUndeterminedRequiredFields(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t, "", "")
	// This existing fixture already requires title, created and updated. None
	// can be taken from the source as the new thought's own metadata.
	got := thoughtMarkdown(contract, contract.AnswerType(), &snapshot.Reading{RelPath: "Source.md", Domain: "not-declared"}, "")
	const want = "---\ntype: \"writing\"\nstatus: \"draft\"\nbased_on: \"[[Source]]\"\n---\n"
	if got != want {
		t.Errorf("thoughtMarkdown() = %q, want no invented metadata", got)
	}
}

func TestThoughtMarkdownOmitsExemptDomain(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t,
		`domain_exempt_types = ["system", "guide", "template"]`,
		`domain_exempt_types = ["system", "guide", "template", "writing"]`)
	got := thoughtMarkdown(contract, contract.AnswerType(), &snapshot.Reading{RelPath: "Source.md", Domain: "japanese"}, "")
	if strings.Contains(got, "domain:") {
		t.Errorf("thoughtMarkdown() invented an exempt domain: %q", got)
	}
}

func TestThoughtMarkdownQuotesSourceWithoutInjectingFrontmatter(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t, "", "")
	source := &snapshot.Reading{RelPath: "Sources/quote\" and & plus+.md"}
	got := thoughtMarkdown(contract, contract.AnswerType(), source, "")
	const want = "---\ntype: \"writing\"\nstatus: \"draft\"\nbased_on: \"[[Sources/quote\\\" and & plus+]]\"\n---\n"
	if got != want {
		t.Errorf("thoughtMarkdown() = %q, want %q", got, want)
	}
}

func thoughtTestContract(t *testing.T, before, after string) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), "[navigation]\n", "[navigation]\nanswer_type = \"writing\"\n", 1)
	if before != "" {
		if strings.Count(text, before) != 1 {
			t.Fatalf("contract fixture needs exactly one %q replacement", before)
		}
		text = strings.Replace(text, before, after, 1)
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(contractPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	return contract
}
