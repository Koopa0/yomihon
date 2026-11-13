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
			got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), source, "A thought", "Chapter one")
			const want = "---\ntitle: \"A thought\"\ntype: \"writing\"\ndraft: true\ndomain: \"japanese\"\nbased_on: \"[[Sources/Book#Chapter one]]\"\n---\n"
			if got != want {
				t.Errorf("thoughtMarkdown() = %q, want %q", got, want)
			}
		})
	}
}

// TestThoughtMarkdownStartsUnfinishedOnlyWhereTheContractKnowsTheFlag pins
// the stub's one claim about the thought's progress: it starts as a draft,
// which is what puts it on the shelf of what the reader left open, and it
// writes the flag only for a contract that lists the field. No status line is
// written either way.
func TestThoughtMarkdownStartsUnfinishedOnlyWhereTheContractKnowsTheFlag(t *testing.T) {
	t.Parallel()
	known := thoughtTestContract(t, "", "")
	unknown := thoughtTestContractWith(t, func(text string) string { return text })
	for _, tc := range []struct {
		name      string
		contract  *schema.Contract
		wantDraft bool
	}{
		{"a contract that knows draft", known, true},
		{"a contract that does not", unknown, false},
	} {
		got := thoughtMarkdown(tc.contract, "writing", &snapshot.Reading{RelPath: "Source.md"}, "", "")
		if strings.Contains(got, "\ndraft: true\n") != tc.wantDraft {
			t.Errorf("%s: thoughtMarkdown() = %q, want draft: true written = %t", tc.name, got, tc.wantDraft)
		}
		if strings.Contains(got, "status:") {
			t.Errorf("%s: thoughtMarkdown() = %q, want no status line", tc.name, got)
		}
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
		got := thoughtMarkdown(contract, "writing", &snapshot.Reading{RelPath: "Source.md"}, "", tc.heading)
		if !strings.Contains(got, `based_on: "`+tc.want+`"`) {
			t.Errorf("heading %q: thoughtMarkdown() = %q, want based_on %s", tc.heading, got, tc.want)
		}
	}
}

func TestThoughtMarkdownOmitsUndeterminedRequiredFields(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t, "", "")
	// This existing fixture already requires title, created and updated. The
	// door is given no title here, and the dates are not the source's to lend.
	got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), &snapshot.Reading{RelPath: "Source.md", Domain: "not-declared"}, "", "")
	const want = "---\ntype: \"writing\"\ndraft: true\nbased_on: \"[[Source]]\"\n---\n"
	if got != want {
		t.Errorf("thoughtMarkdown() = %q, want no invented metadata", got)
	}
}

func TestThoughtMarkdownWritesTheTitleItIsGivenQuoted(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t, "", "")
	got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), &snapshot.Reading{RelPath: "Source.md"}, `On: "quotes" 的想法`, "")
	const want = "---\ntitle: \"On: \\\"quotes\\\" 的想法\"\ntype: \"writing\"\ndraft: true\nbased_on: \"[[Source]]\"\n---\n"
	if got != want {
		t.Errorf("thoughtMarkdown() = %q, want %q", got, want)
	}
}

func TestThoughtMarkdownOmitsTitleTheContractDoesNotKnow(t *testing.T) {
	t.Parallel()
	// Removing the name from every list keeps required a subset of known, so
	// the contract still loads and simply has no title field.
	contract := thoughtTestContractWith(t, func(text string) string {
		if !strings.Contains(text, `"title", `) {
			t.Fatal("contract fixture no longer lists title")
		}
		return strings.ReplaceAll(text, `"title", `, "")
	})
	got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), &snapshot.Reading{RelPath: "Source.md"}, "A thought", "")
	if strings.Contains(got, "title:") {
		t.Errorf("thoughtMarkdown() = %q, want no title for a contract without the field", got)
	}
}

func TestThoughtMarkdownOmitsExemptDomain(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t,
		`domain_exempt_types = ["system", "guide", "template"]`,
		`domain_exempt_types = ["system", "guide", "template", "writing"]`)
	got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), &snapshot.Reading{RelPath: "Source.md", Domain: "japanese"}, "", "")
	if strings.Contains(got, "domain:") {
		t.Errorf("thoughtMarkdown() invented an exempt domain: %q", got)
	}
}

func TestThoughtMarkdownQuotesSourceWithoutInjectingFrontmatter(t *testing.T) {
	t.Parallel()
	contract := thoughtTestContract(t, "", "")
	source := &snapshot.Reading{RelPath: "Sources/quote\" and & plus+.md"}
	got := thoughtMarkdown(contract, contract.NavigationRoles().AnswerType(), source, "", "")
	const want = "---\ntype: \"writing\"\ndraft: true\nbased_on: \"[[Sources/quote\\\" and & plus+]]\"\n---\n"
	if got != want {
		t.Errorf("thoughtMarkdown() = %q, want %q", got, want)
	}
}

// thoughtTestContract is the shared contract with the reader's thoughts filed
// as writing and the draft flag known, edited once more by before and after.
func thoughtTestContract(t *testing.T, before, after string) *schema.Contract {
	t.Helper()
	return thoughtTestContractWith(t, func(text string) string {
		text = strings.Replace(text, "known = [", "known = [\"draft\", ", 1)
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
