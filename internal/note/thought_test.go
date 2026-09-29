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
			got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), source, "Chapter one")
			const want = "---\ntype: \"writing\"\nstatus: \"draft\"\ndomain: \"japanese\"\nbased_on: \"[[Sources/Book#Chapter one]]\"\n---\n"
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
	got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), &snapshot.Reading{RelPath: "Source.md"}, "")
	const want = "---\ntype: \"writing\"\nbased_on: \"[[Source]]\"\n---\n"
	if got != want {
		t.Errorf("thoughtMarkdown() = %q, want status omitted with multiple initial states", got)
	}
}

func TestThoughtMarkdownWritesHeadingTextNotItsID(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t, "", "")
	for _, tc := range []struct{ heading, want string }{
		{"Chapter one", `[[Source#Chapter one]]`},
		{"日本語の 章", `[[Source#日本語の 章]]`},
		{"A | B # C [x] ^d", `[[Source#A B C x d]]`},
		{"", `[[Source]]`},
	} {
		got := thoughtMarkdown(contract, "writing", &snapshot.Reading{RelPath: "Source.md"}, tc.heading)
		if !strings.Contains(got, `based_on: "`+tc.want+`"`) {
			t.Errorf("heading %q: thoughtMarkdown() = %q, want based_on %s", tc.heading, got, tc.want)
		}
	}
}

func TestThoughtMarkdownOmitsStatusWithoutLiteralInitial(t *testing.T) {
	t.Parallel()
	// With every initial key removed the lifecycle still lets a reader infer a
	// first status from rows that name no predecessor, but nothing declares one.
	contract := thoughtTestContractWith(t, func(text string) string {
		var kept []string
		for line := range strings.SplitSeq(text, "\n") {
			if !strings.HasPrefix(line, "initial = ") {
				kept = append(kept, line)
			}
		}
		return strings.Join(kept, "\n")
	})
	got := thoughtMarkdown(contract, "writing", &snapshot.Reading{RelPath: "Source.md"}, "")
	if strings.Contains(got, "status:") {
		t.Errorf("thoughtMarkdown() = %q, want no status line when no initial is declared", got)
	}
}

func TestThoughtMarkdownOmitsUndeterminedRequiredFields(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t, "", "")
	// This existing fixture already requires title, created and updated. None
	// can be taken from the source as the new thought's own metadata.
	got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), &snapshot.Reading{RelPath: "Source.md", Domain: "not-declared"}, "")
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
	got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), &snapshot.Reading{RelPath: "Source.md", Domain: "japanese"}, "")
	if strings.Contains(got, "domain:") {
		t.Errorf("thoughtMarkdown() invented an exempt domain: %q", got)
	}
}

func TestThoughtMarkdownQuotesSourceWithoutInjectingFrontmatter(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t, "", "")
	source := &snapshot.Reading{RelPath: "Sources/quote\" and & plus+.md"}
	got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), source, "")
	const want = "---\ntype: \"writing\"\nstatus: \"draft\"\nbased_on: \"[[Sources/quote\\\" and & plus+]]\"\n---\n"
	if got != want {
		t.Errorf("thoughtMarkdown() = %q, want %q", got, want)
	}
}

func thoughtTestContract(t *testing.T, before, after string) *schema.Contract {
	t.Helper()
	return thoughtTestContractWith(t, func(text string) string {
		if before == "" {
			return text
		}
		if strings.Count(text, before) != 1 {
			t.Fatalf("contract fixture needs exactly one %q replacement", before)
		}
		return strings.Replace(text, before, after, 1)
	})
}

func thoughtTestContractWith(t *testing.T, edit func(string) string) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), "[navigation]\n", "[navigation]\nanswer_type = \"writing\"\n", 1)
	text = edit(text)
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(contractPath, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	return contract
}
