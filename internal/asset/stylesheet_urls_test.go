package asset

import (
	"testing"
)

func TestStylesheetReferencesUseRegisteredByteIdentities(t *testing.T) {
	t.Parallel()
	reg := map[string]entry{"fonts/example.woff2": fixed(woff2ContentType, []byte("first"))}
	for _, tt := range []struct {
		name   string
		source string
		want   string
	}{
		{name: "single quoted", source: "src: url('/static/fonts/example.woff2');", want: "src: url('/static/fonts/example.woff2?v=a7937b64b8ca');"},
		{name: "double quoted", source: `src: url("/static/fonts/example.woff2");`, want: `src: url("/static/fonts/example.woff2?v=a7937b64b8ca");`},
		{name: "unquoted", source: "src: url(/static/fonts/example.woff2);", want: "src: url(/static/fonts/example.woff2?v=a7937b64b8ca);"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := string(rewriteStaticURLs(reg, []byte(tt.source))); got != tt.want {
				t.Errorf("caught: stylesheet references = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStylesheetRefusesAnUnregisteredStaticReference(t *testing.T) {
	t.Parallel()
	defer func() {
		if got := recover(); got != "asset: unknown stylesheet URL name: fonts/missing.woff2" {
			t.Errorf("caught: unknown stylesheet reference panic = %v, want the missing registered name", got)
		}
	}()
	rewriteStaticURLs(map[string]entry{}, []byte("src: url('/static/fonts/missing.woff2');"))
}
