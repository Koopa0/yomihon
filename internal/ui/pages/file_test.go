package pages

import (
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestHumanSize pins what the information page tells a reader about a file it
// cannot show. The exact byte count always travels with the rounded figure,
// because the page exists to be exact, and a lone file's byte is one byte.
func TestHumanSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		n    int64
		zh   string
		en   string
	}{
		{name: "empty", n: 0, zh: "0 位元組", en: "0 bytes"},
		{name: "one byte is singular", n: 1, zh: "1 位元組", en: "1 byte"},
		{name: "two bytes", n: 2, zh: "2 位元組", en: "2 bytes"},
		{name: "just under a kibibyte", n: 1023, zh: "1,023 位元組", en: "1,023 bytes"},
		{name: "a kibibyte", n: 1024, zh: "1.0 KiB（1,024 位元組）", en: "1.0 KiB (1,024 bytes)"},
		{name: "a mebibyte", n: 1 << 20, zh: "1.0 MiB（1,048,576 位元組）", en: "1.0 MiB (1,048,576 bytes)"},
		{name: "a gibibyte", n: 1 << 30, zh: "1.0 GiB（1,073,741,824 位元組）", en: "1.0 GiB (1,073,741,824 bytes)"},
		{name: "one byte over the reading limit", n: 16777217, zh: "16.0 MiB（16,777,217 位元組）", en: "16.0 MiB (16,777,217 bytes)"},
		{name: "twice the reading limit", n: 33554432, zh: "32.0 MiB（33,554,432 位元組）", en: "32.0 MiB (33,554,432 bytes)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, language := range []struct {
				lang wording.Lang
				want string
			}{
				{lang: wording.ZhHant, want: tt.zh},
				{lang: wording.En, want: tt.en},
			} {
				if got := humanSize(tt.n, language.lang); got != language.want {
					t.Errorf("humanSize(%d, %v) = %q, want %q", tt.n, language.lang, got, language.want)
				}
			}
		})
	}
}
