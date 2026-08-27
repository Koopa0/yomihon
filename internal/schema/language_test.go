package schema

import "testing"

func TestArticleLanguageRequiresContractAuthority(t *testing.T) {
	t.Parallel()
	resolver := (&Contract{}).ArticleLanguage()
	got, err := resolver.Resolve(map[string]any{"lang": "ja"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != "" {
		t.Errorf("Resolve() = %q, want no tag without a contract declaration", got)
	}
}

func TestArticleLanguageRejectsLessonOnlyAuthority(t *testing.T) {
	t.Parallel()
	contract := &Contract{definition: Definition{Fields: Fields{LessonOnly: []string{"lang"}}}}
	got, err := contract.ArticleLanguage().Resolve(map[string]any{"lang": "ja"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != "" {
		t.Errorf("Resolve() = %q, want no tag when only lesson fields declare lang", got)
	}
}

func TestArticleLanguageResolve(t *testing.T) {
	t.Parallel()
	contract := &Contract{definition: Definition{Fields: Fields{Known: []string{"title", "lang"}}}}
	resolver := contract.ArticleLanguage()
	tests := []struct {
		name        string
		frontmatter map[string]any
		want        string
		wantErr     bool
	}{
		{name: "missing", frontmatter: nil, want: ""},
		{name: "Japanese", frontmatter: map[string]any{"lang": "ja"}, want: "ja"},
		{name: "canonicalized", frontmatter: map[string]any{"lang": "zh-hant"}, want: "zh-Hant"},
		// An author who writes the undetermined tag by hand gets it back: it
		// is a value the grammar accepts and the contract gave authority to,
		// and refusing to carry one the author chose would be the resolver
		// overruling the note. It is the resolver's own silence that no longer
		// spells itself that way.
		{name: "explicit undetermined", frontmatter: map[string]any{"lang": "und"}, want: "und"},
		{name: "empty", frontmatter: map[string]any{"lang": ""}, want: "", wantErr: true},
		{name: "wrong type", frontmatter: map[string]any{"lang": []string{"ja"}}, want: "", wantErr: true},
		{name: "invalid", frontmatter: map[string]any{"lang": "not_a_tag"}, want: "", wantErr: true},
		{name: "leading separator", frontmatter: map[string]any{"lang": "-ja"}, want: "", wantErr: true},
		{name: "trailing separator", frontmatter: map[string]any{"lang": "ja-"}, want: "", wantErr: true},
		{name: "repeated separator", frontmatter: map[string]any{"lang": "ja--JP"}, want: "", wantErr: true},
		{name: "non ASCII", frontmatter: map[string]any{"lang": "日本語"}, want: "", wantErr: true},
		{name: "control", frontmatter: map[string]any{"lang": "ja\nJP"}, want: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolver.Resolve(tt.frontmatter)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}

func FuzzParseLanguageTag(f *testing.F) {
	for _, seed := range []string{
		"",
		"ja",
		"zh-hant",
		"en-US",
		"x-private",
		"ja-JP-u-ca-japanese",
		"-ja",
		"ja-",
		"ja--JP",
		"not_a_tag",
		"日本語",
		"ja\nJP",
		string([]byte{'j', 'a', 0}),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		got, err := ParseLanguageTag(raw)
		if err != nil {
			return
		}
		if got == "" {
			t.Fatal("ParseLanguageTag() accepted an empty canonical tag")
		}
		roundTrip, roundTripErr := ParseLanguageTag(got)
		if roundTripErr != nil {
			t.Fatalf("ParseLanguageTag(%q) accepted %q, which it then rejected: %v", raw, got, roundTripErr)
		}
		if roundTrip != got {
			t.Errorf("ParseLanguageTag(%q) = %q; canonical round trip = %q", raw, got, roundTrip)
		}
	})
}
