package figure_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/figure"
	"github.com/koopa0/yomihon/internal/render"
)

func TestTheFigureCeilingIsTheSourceCeiling(t *testing.T) {
	t.Parallel()
	if figure.MaxBytes != render.MaxSourceBytes {
		t.Fatalf("figure.MaxBytes = %d, want render.MaxSourceBytes %d", figure.MaxBytes, render.MaxSourceBytes)
	}
}

func TestIsNamesOnlyHTMLInAnyCase(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]bool{
		"Notes/x.html": true, "X.HTML": true, "a/b.Html": true,
		"x.htm": false, "x.html.md": false, "x.svg": false, "html": false, "x.xhtml": false,
	} {
		if got := figure.Is(path); got != want {
			t.Errorf("Is(%q) = %v, want %v", path, got, want)
		}
	}
}

// refusal is one row's expected verdict: the refusal and what it names.
type refusal struct {
	Refusal figure.Refusal
	Detail  string
}

// TestInspectRefusesWhatReachesOutsideTheFrame is the table of markup a figure
// may not carry, spelled the ways a parser and a pattern disagree about. Each
// row is markup a browser acts on; a regular expression or a bare tokenizer
// misses several of them, which is the reason the inspection builds the tree.
func TestInspectRefusesWhatReachesOutsideTheFrame(t *testing.T) {
	t.Parallel()
	frame := func(detail string) refusal { return refusal{figure.NestedFrame, detail} }
	loads := func(detail string) refusal { return refusal{figure.LoadsElsewhere, detail} }
	script := func(detail string) refusal { return refusal{figure.ScriptOutsideText, detail} }
	for _, tc := range []struct {
		name string
		html string
		want refusal
	}{
		{"plain iframe", `<iframe src="http://x.test/"></iframe>`, frame("<iframe>")},
		{"upper case", `<IFRAME SRC=http://x.test/>`, frame("<iframe>")},
		{"mixed case and odd attribute spacing", "<iFrAmE\tsrc\n=\n'http://x.test/'/>", frame("<iframe>")},
		{"unclosed at end of file", `<p>text<iframe src=//x.test/`, refusal{figure.Served, ""}},
		{"unclosed tag with attributes closed by end", `<p>text<iframe src=//x.test/ >`, frame("<iframe>")},
		{"frame", `<frameset><frame src=//x.test/></frameset>`, frame("<frameset>")},
		{"embed", `<embed src=//x.test/>`, frame("<embed>")},
		{"object", `<object data=//x.test/></object>`, frame("<object>")},
		{"fenced frame", `<fencedframe></fencedframe>`, frame("<fencedframe>")},
		{"portal", `<portal src=//x.test/></portal>`, frame("<portal>")},
		{"link preconnect", `<link rel=preconnect href=//x.test/>`, loads("<link>")},
		{"link in body", `<body><p><link rel=prefetch href=//x.test/>`, loads("<link>")},
		{"base", `<base href=//x.test/>`, loads("<base>")},
		{"meta refresh", `<meta http-equiv=refresh content="0;url=//x.test/">`, loads(`<meta http-equiv="refresh">`)},
		{"meta refresh upper case padded", `<meta http-equiv=" REFRESH " content=0>`, loads(`<meta http-equiv="refresh">`)},
		{"meta dns prefetch", `<meta http-equiv=x-dns-prefetch-control content=on>`, loads(`<meta http-equiv="x-dns-prefetch-control">`)},
		{"a ping", `<a href=/ ping=//x.test/>x</a>`, loads("ping")},
		{"ping on area", `<map><area href=/ ping=//x.test/></map>`, loads("ping")},
		{"attributionsrc", `<img src=x attributionsrc>`, loads("attributionsrc")},
		{"script src", `<script src=//x.test/a.js></script>`, script("<script src>")},
		{"svg script href", `<svg><script href="//x.test/a.js"></script></svg>`, script("<script href>")},
		{"svg script xlink href", `<svg><script xlink:href="//x.test/a.js"></script></svg>`, script("<script xlink:href>")},
		// Scripting off, a browser parses noscript's inside as markup; a
		// tokenizer reading noscript as text would let this frame through.
		{"noscript iframe", `<noscript><iframe src=//x.test/></iframe></noscript>`, frame("<iframe>")},
		{"noscript in head", `<head><noscript><link rel=preconnect href=//x.test/></noscript></head>`, loads("<link>")},
		{"svg foreign object", `<svg><foreignObject><iframe src=//x.test/></iframe></foreignObject></svg>`, frame("<iframe>")},
		{"math annotation xml", `<math><annotation-xml encoding="text/html"><iframe src=//x.test/></iframe></annotation-xml></math>`, frame("<iframe>")},
		{"template content", `<template><iframe src=//x.test/></iframe></template>`, frame("<iframe>")},
		{"declarative shadow root", `<div><template shadowrootmode=open><link rel=preconnect href=//x.test/></template></div>`, loads("<link>")},
		{"nested template", `<template><div><template><embed src=//x.test/></template></div></template>`, frame("<embed>")},
		{"in a table", `<table><tr><td><object data=//x.test/></object></td></tr></table>`, frame("<object>")},
		{"misnested formatting", `<b><i></b><iframe src=//x.test/></i>`, frame("<iframe>")},
		{"after a stray end tag", `</p></iframe><iframe src=//x.test/>`, frame("<iframe>")},
		{"event handler", `<img src=x onerror=alert(1)>`, script("onerror")},
		{"event handler upper case", `<body ONLOAD="x()">`, script("onload")},
		{"svg event handler", `<svg><circle onclick="x()"/></svg>`, script("onclick")},
		{"javascript address", `<a href="javascript:alert(1)">x</a>`, script("javascript:")},
		{"javascript address with entities and breaks", "<a href=\"jav&#x09;ascript&colon;x\">x</a>", script("javascript:")},
		{"javascript address split by line breaks", "<a href=\"java\nscript:x\">x</a>", script("javascript:")},
		{"svg animation to javascript", `<svg><a><animate attributeName=href values="#;javascript:x()"/></a></svg>`, script("javascript:")},
		{"speculation rules", `<script type=speculationrules>{}</script>`, script(`<script type="speculationrules">`)},
		{"import map", `<script type="importmap">{}</script>`, script(`<script type="importmap">`)},
		{"data block", `<script type="application/json">{}</script>`, script(`<script type="application/json">`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := figure.Inspect([]byte(tc.html))
			got := refusal{in.Refusal, in.Detail}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Inspect(%q) mismatch (-want +got):\n%s", tc.html, diff)
			}
			if in.Refusal != figure.Served && in.Scripts != nil {
				t.Errorf("Inspect(%q) refused and still listed scripts %q", tc.html, in.Scripts)
			}
		})
	}
}

func TestInspectRefusesBytesThatAreNotUTF8Text(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		data []byte
		want figure.Refusal
	}{
		{"over the ceiling", []byte(strings.Repeat("a", figure.MaxBytes+1)), figure.TooLarge},
		{"at the ceiling", []byte(strings.Repeat("a", figure.MaxBytes)), figure.Served},
		{"UTF-16 big endian mark", []byte{0xFE, 0xFF, 0, '<'}, figure.UTF16},
		{"UTF-16 little endian mark", []byte{0xFF, 0xFE, '<', 0}, figure.UTF16},
		{"invalid UTF-8", []byte("<p>\xC3\x28</p>"), figure.NotText},
		{"NUL byte", []byte("<ifr\x00ame src=//x.test/>"), figure.NotText},
		{"UTF-8 byte order mark", []byte("\xEF\xBB\xBF<p>x</p>"), figure.Served},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := figure.Inspect(tc.data).Refusal; got != tc.want {
				t.Errorf("Inspect(%s).Refusal = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

// TestInspectServesWhatStaysInsideTheFrame holds the other side: markup the
// rulings keep. A link is not refused; neither is a picture, a stylesheet
// address, or a form, each of which the frame's policy closes on its own.
func TestInspectServesWhatStaysInsideTheFrame(t *testing.T) {
	t.Parallel()
	for _, markup := range []string{
		`<a href="https://example.test/">a link</a>`,
		`<a href=https://example.test/ target=_top>a link out</a>`,
		`<img src="https://example.test/x.png" alt="">`,
		`<style>@import url(https://example.test/a.css); p { background: url(//example.test/b.png) }</style>`,
		`<form action=https://example.test/><button>go</button></form>`,
		`<svg><use href="#a"/><symbol id=a></symbol></svg>`,
		`<meta charset=utf-8><meta http-equiv="Content-Type" content="text/html; charset=utf-8">`,
		`<meta http-equiv=content-security-policy content="img-src 'none'">`,
		`<details><summary>more</summary>text</details>`,
		`<p>the word javascript without a colon</p>`,
		`<noscript><p>no script here</p></noscript>`,
	} {
		in := figure.Inspect([]byte(markup))
		if in.Refusal != figure.Served {
			t.Errorf("Inspect(%q) refused with %d %q, want served", markup, in.Refusal, in.Detail)
		}
		if in.Scripted() {
			t.Errorf("Inspect(%q).Scripted() = true for a figure with no script", markup)
		}
	}
}

// TestInspectHashesTheScriptTheBrowserRuns pins the content-policy hashes to
// literals computed outside this code (openssl dgst -sha256 -binary | base64),
// so the expected value is not the function under test. The browser hashes
// the script's text after its own newline normalization, and the second row
// is where a hash computed over the raw bytes would differ.
func TestInspectHashesTheScriptTheBrowserRuns(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		html    string
		scripts []string
		hashes  []string
	}{
		{
			name:    "one script",
			html:    `<p>x</p><script>parent.postMessage(1,'*')</script>`,
			scripts: []string{"parent.postMessage(1,'*')"},
			hashes:  []string{"'sha256-q0WzbdtafWUuNCZHUF6Fci75N1nqGycz70QmvkjZPdE='"},
		},
		{
			name:    "carriage return line feed",
			html:    "<script>a\r\nb</script>",
			scripts: []string{"a\nb"},
			hashes:  []string{"'sha256-fhj3NzEbLcOy8mndeDlrA1HxT7Zu+oefdoyyMYGIPHg='"},
		},
		{
			name:    "an empty script and a repeat",
			html:    "<script></script><script>a\nb</script><script>a\nb</script>",
			scripts: []string{"", "a\nb", "a\nb"},
			hashes: []string{
				"'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='",
				"'sha256-fhj3NzEbLcOy8mndeDlrA1HxT7Zu+oefdoyyMYGIPHg='",
			},
		},
		{
			name:    "script inside noscript is text when scripting is on",
			html:    "<noscript><script>a\nb</script></noscript><script></script>",
			scripts: []string{""},
			hashes:  []string{"'sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU='"},
		},
		{
			name:    "svg script decodes its entities",
			html:    "<svg><script>a&#10;b</script></svg>",
			scripts: []string{"a\nb"},
			hashes:  []string{"'sha256-fhj3NzEbLcOy8mndeDlrA1HxT7Zu+oefdoyyMYGIPHg='"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := figure.Inspect([]byte(tc.html))
			if in.Refusal != figure.Served {
				t.Fatalf("Inspect(%q) refused with %d %q", tc.html, in.Refusal, in.Detail)
			}
			if diff := cmp.Diff(tc.scripts, in.Scripts); diff != "" {
				t.Errorf("Inspect(%q).Scripts mismatch (-want +got):\n%s", tc.html, diff)
			}
			if diff := cmp.Diff(tc.hashes, in.ScriptHashes()); diff != "" {
				t.Errorf("Inspect(%q).ScriptHashes() mismatch (-want +got):\n%s", tc.html, diff)
			}
		})
	}
}

// TestInspectDigestsTheExactBytes is the approval's anchor: one byte changed
// anywhere is a different digest, so an approval of the old bytes no longer
// names the file.
func TestInspectDigestsTheExactBytes(t *testing.T) {
	t.Parallel()
	a := figure.Inspect([]byte("<script>x()</script>"))
	b := figure.Inspect([]byte("<script>x() </script>"))
	if a.Sum == b.Sum {
		t.Fatal("two different byte strings share a digest")
	}
	// sha256 of "<p>" computed with openssl, as an independent literal.
	got := figure.Inspect([]byte("<p>")).Sum
	const want = "33f2799467177287a29260780a107ac98ea63dd6165f67fcc0d74767d0a82090"
	if hex.EncodeToString(got[:]) != want {
		t.Errorf("Inspect(\"<p>\").Sum = %x, want %s", got, want)
	}
}

func FuzzInspect(f *testing.F) {
	for _, seed := range []string{
		"<iframe src=x>", "<noscript><iframe></noscript>", "<svg><script>x</script></svg>",
		"<template><link></template>", "<script>a</script>", "<a ping=x>", "\xFF\xFE",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		in := figure.Inspect(data)
		if in.Refusal != figure.Served && len(in.Scripts) > 0 {
			t.Fatalf("Inspect refused (%d) and still listed %d scripts", in.Refusal, len(in.Scripts))
		}
		if in.Refusal == figure.Served && len(in.ScriptHashes()) > len(in.Scripts) {
			t.Fatal("more hashes than scripts")
		}
	})
}
