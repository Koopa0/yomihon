package note

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
)

func TestAnnotateMarkAnchorsPreservesRenderedBytes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, body, want string }{
		{"quotes and attributes", `<P class='a' ID = 'safe' title="kept">text</P>`, `<P data-mark-anchor class='a' ID = 'safe' title="kept">text</P>`},
		{"attribute newlines", "<p id=\"safe\" title=\"first\r\nsecond\">", "<p data-mark-anchor id=\"safe\" title=\"first\r\nsecond\">"},
		{"unquoted self-closing", `<img id=safe />`, `<img data-mark-anchor id=safe />`},
		{"entity decoding accepts ampersand", `<p id="a&amp;b">`, `<p data-mark-anchor id="a&amp;b">`},
		{"quote entities refused", `<p id="a&quot;b"><p id='a&#34;b'>`, `<p id="a&quot;b"><p id='a&#34;b'>`},
		{"delimiter entities refused", `<p id="a&#47;b"><p id="a&#63;b">`, `<p id="a&#47;b"><p id="a&#63;b">`},
		{"missing and empty", `<p><p id=""><p id>`, `<p><p id=""><p id>`},
		{"duplicate ID uses first", `<p id="safe" id="a/b"><p id="a/b" id="safe">`, `<p data-mark-anchor id="safe" id="a/b"><p id="a/b" id="safe">`},
		{"raw-text and comment decoys", `<script type="application/json">{"html":"<p id='safe'>"}</script><!-- <p id="safe"> --><textarea><p id="safe"></textarea>&lt;p id="safe"&gt;`, `<script type="application/json">{"html":"<p id='safe'>"}</script><!-- <p id="safe"> --><textarea><p id="safe"></textarea>&lt;p id="safe"&gt;`},
		{"truncated token", `<p id="safe`, `<p id="safe`},
		{"invalid UTF-8", "<p id='a" + string([]byte{0xff}) + "b'>", "<p id='a" + string([]byte{0xff}) + "b'>"},
		{"already annotated", `<p id="safe" data-mark-anchor>`, `<p id="safe" data-mark-anchor>`},
		{"unicode", `<p id="漢字">`, `<p data-mark-anchor id="漢字">`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, annotateMarkAnchors(tt.body)); diff != "" {
				t.Errorf("annotated HTML mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMarkAnchorsUseQualifiedByteLength(t *testing.T) {
	t.Parallel()
	id := strings.Repeat("a", 256)
	result := render.Result{HTML: `<h2 id="` + id + `">heading</h2>`}
	render.Qualify("right-", &result)
	want := `<h2 id="right-` + id + `">heading</h2>`
	if diff := cmp.Diff(want, annotateMarkAnchors(result.HTML)); diff != "" {
		t.Errorf("qualified anchor must remain unmarked (-want +got):\n%s", diff)
	}
}
