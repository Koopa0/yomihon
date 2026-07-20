package render

import "testing"

func TestStripObsidianCommentsPreservesFenceOpeningLine(t *testing.T) {
	t.Parallel()

	body := "```text %%literal info%%\n%%literal body%%\n```\nafter %%hidden%%"
	want := "```text %%literal info%%\n%%literal body%%\n```\nafter "
	if got := stripObsidianComments(body); got != want {
		t.Errorf("stripObsidianComments() = %q, want %q", got, want)
	}
}

func FuzzStripObsidianComments(f *testing.F) {
	for _, seed := range []string{
		"plain text",
		"before %%hidden%% after",
		"%%unclosed",
		"one%%first%%%%second%%two",
		"```text\n%%literal%%\n```",
		"```text %%literal info%%\n%%literal%%\n```",
		"%%hidden%% ```text\n%%literal%%\n```",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body string) {
		got := stripObsidianComments(body)
		if second := stripObsidianComments(body); second != got {
			t.Fatalf("stripObsidianComments() is not deterministic: first %q, second %q", got, second)
		}
		if len(got) > len(body) {
			t.Fatalf("stripObsidianComments() length = %d, want at most input length %d", len(got), len(body))
		}
		if again := stripObsidianComments(got); again != got {
			t.Fatalf("stripObsidianComments() is not idempotent: first %q, second %q", got, again)
		}
	})
}
