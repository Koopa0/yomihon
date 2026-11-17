package graph

import (
	"slices"
	"testing"
)

func TestGitHubAnchorsFollowGitHub(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		names []string
		want  []string
	}{
		{"punctuation is deleted", []string{"What's new?"}, []string{"whats-new"}},
		{"each space is one hyphen", []string{"a  b - c"}, []string{"a--b---c"}},
		{"CJK letters stay and full-width marks go", []string{"4. 主題清單（129 項）"}, []string{"4-主題清單129-項"}},
		{"underscore and hyphen stay", []string{"snake_case-name"}, []string{"snake_case-name"}},
		{"case folds", []string{"Go 語言 FAQ"}, []string{"go-語言-faq"}},
		{"an emoji is dropped with its space kept", []string{"🚀 Launch"}, []string{"-launch"}},
		{"a repeat counts from 1", []string{"Intro", "Intro", "Intro"}, []string{"intro", "intro-1", "intro-2"}},
		{"a repeat that lands on a name that exists", []string{"Intro", "Intro 1", "Intro"}, []string{"intro", "intro-1", "intro-2"}},
		{"all punctuation leaves nothing", []string{"???", "!!!"}, []string{"", "-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := GitHubAnchors(tt.names); !slices.Equal(got, tt.want) {
				t.Errorf("GitHubAnchors(%q) = %q, want %q", tt.names, got, tt.want)
			}
		})
	}
}

func TestGitHubAnchorsDifferFromSectionIDWhereTheyShould(t *testing.T) {
	t.Parallel()
	const name = "What's new?"
	if got := SectionID(name); got != "what-s-new" {
		t.Fatalf("SectionID(%q) = %q: the id a page stamps must not change", name, got)
	}
	if got := GitHubAnchors([]string{name})[0]; got != "whats-new" {
		t.Errorf("GitHubAnchors(%q) = %q, want whats-new", name, got)
	}
}
