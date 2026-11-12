package figure

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// MaxBytes is the largest figure served. It is the ceiling every file read as
// characters has (render.MaxSourceBytes, which imports this package and so
// cannot be named here); a test holds the two equal.
const MaxBytes = 1 << 20 // 1 MiB

// Refusal says why a figure is not served. The zero value is a figure that is.
type Refusal uint8

const (
	// Served is a figure whose markup asks the browser for nothing outside it.
	Served Refusal = iota
	// TooLarge is a figure over MaxBytes.
	TooLarge
	// NotText is a figure whose bytes are not UTF-8 text: invalid UTF-8, or a
	// NUL byte, which no text yomihon reads carries and which the browser and
	// this parse would each replace in their own way.
	NotText
	// UTF16 is a figure that opens with a UTF-16 byte order mark. A browser
	// honours that mark over the declared charset, so the document it builds
	// is not the one parsed here.
	UTF16
	// NestedFrame is an element that frames another document. Measured, a
	// nested frame opens a connection to its address when the figure opens,
	// before the frame's content policy refuses the request. Detail names it.
	NestedFrame
	// LoadsElsewhere is markup that makes the browser connect ahead, re-base
	// addresses, move to another page, or send a request of its own, with no
	// reader action. Detail names it.
	LoadsElsewhere
	// ScriptOutsideText is script the approval cannot pin by its text: a
	// script file, an event-handler attribute, a javascript: address, or a
	// script element of a kind other than a classic or module script. Detail
	// names it.
	ScriptOutsideText
)

// Inspection is what one read of a figure's bytes says about it. Everything
// in it describes those bytes and nothing else, so the digest, the verdict
// and the scripts can never come from two different reads.
type Inspection struct {
	// Sum is the SHA-256 of the bytes inspected, which is what an approval pins.
	Sum [sha256.Size]byte
	// Refusal is why the figure is not served, or Served.
	Refusal Refusal
	// Detail names the markup a refusal found, as the author would recognise
	// it: "<iframe>", "ping", "onclick". Empty for the other refusals.
	Detail string
	// Scripts are the text of each inline script, in document order, as the
	// browser builds them with scripting on. Nil for a refused figure.
	Scripts []string
}

// Scripted reports whether the figure carries inline script, and so needs an
// approval to run it.
func (in *Inspection) Scripted() bool {
	return in.Refusal == Served && len(in.Scripts) > 0
}

// ScriptHashes are the content-policy source expressions that admit exactly
// this figure's inline scripts: one 'sha256-…' per distinct script text, in
// order. A browser compares each script it is about to run against these, so
// a script that was not in the bytes inspected — injected later, built by
// another script, or carried in by a nested document — does not run.
func (in *Inspection) ScriptHashes() []string {
	hashes := make([]string, 0, len(in.Scripts))
	for _, script := range in.Scripts {
		sum := sha256.Sum256([]byte(script))
		hash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
		if !slices.Contains(hashes, hash) {
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

// Inspect reads one figure's bytes and says whether they may be served and
// which inline scripts they carry.
//
// The bytes are parsed twice with the standard tree builder, once with
// scripting off and once with it on, because the two build different
// documents and a figure is shown under both: unapproved, its frame has no
// script, and a browser with no script parses the inside of a noscript element
// as markup, so a nested frame written there is a real one. Every node of both
// trees is visited, including what a template element and foreign content
// (svg, math) hold.
func Inspect(data []byte) Inspection {
	in := Inspection{Sum: sha256.Sum256(data)}
	switch {
	case len(data) > MaxBytes:
		in.Refusal = TooLarge
		return in
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}) || bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		in.Refusal = UTF16
		return in
	case !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0:
		in.Refusal = NotText
		return in
	}
	for _, scripting := range []bool{false, true} {
		// A bytes.Reader never fails, so the parse cannot either: the HTML
		// tree builder accepts every input and errors only on a failed read.
		doc, err := html.ParseWithOptions(bytes.NewReader(data), html.ParseOptionEnableScripting(scripting))
		if err != nil {
			in.Refusal = NotText
			return in
		}
		if refusal, detail := refuse(doc); refusal != Served {
			in.Refusal, in.Detail = refusal, detail
			return in
		}
		if scripting {
			in.Scripts = inlineScripts(doc)
		}
	}
	return in
}

// nestedFrames are elements that frame another document, and fetching are
// elements that ask the browser to fetch, preconnect, or re-base on their
// own. Both are matched in any namespace: an element of the same name in svg
// or math is not one the browser acts on, and refusing it costs an author
// nothing.
var (
	nestedFrames = []string{"iframe", "frame", "frameset", "embed", "object", "fencedframe", "portal"}
	fetching     = []string{"link", "base"}
)

// harmlessEquivalents are the http-equiv values a figure may carry. Each
// either restates what the response already says or can only narrow it.
// Every other value is refused: refresh navigates on its own, and a value
// this list does not know is not one to find out about in a reader's browser.
var harmlessEquivalents = []string{"content-type", "content-language", "content-security-policy"}

// scriptTypes are the type values that make a script element a classic or a
// module script, the two kinds whose inline text a content-policy hash pins.
// An import map or speculation rules reach for addresses of their own, and a
// data block is not script at all; neither is something to approve as script.
var scriptTypes = []string{
	"", "module",
	"text/javascript", "application/javascript", "application/ecmascript",
	"application/x-ecmascript", "application/x-javascript", "text/ecmascript",
	"text/javascript1.0", "text/javascript1.1", "text/javascript1.2",
	"text/javascript1.3", "text/javascript1.4", "text/javascript1.5",
	"text/jscript", "text/livescript", "text/x-ecmascript", "text/x-javascript",
}

// refuse walks every node of one parse and names the first markup a figure
// may not carry.
func refuse(doc *html.Node) (Refusal, string) {
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		name := strings.ToLower(n.Data)
		if slices.Contains(nestedFrames, name) {
			return NestedFrame, "<" + name + ">"
		}
		if slices.Contains(fetching, name) {
			return LoadsElsewhere, "<" + name + ">"
		}
		switch name {
		case "meta":
			if equiv, ok := attribute(n, "http-equiv"); ok {
				value := strings.ToLower(strings.TrimSpace(equiv))
				if !slices.Contains(harmlessEquivalents, value) {
					return LoadsElsewhere, `<meta http-equiv="` + value + `">`
				}
			}
		case "script":
			for _, key := range []string{"src", "href", "xlink:href"} {
				if _, ok := attribute(n, key); ok {
					return ScriptOutsideText, "<script " + key + ">"
				}
			}
			if kind, ok := attribute(n, "type"); ok {
				kind = strings.ToLower(strings.TrimSpace(kind))
				if !slices.Contains(scriptTypes, kind) {
					return ScriptOutsideText, `<script type="` + kind + `">`
				}
			}
		}
		for _, attr := range n.Attr {
			key := strings.ToLower(attr.Key)
			switch {
			case key == "ping" || key == "attributionsrc":
				// Both send a request of their own beside the one the element
				// makes, which is the request a content policy governs.
				return LoadsElsewhere, key
			case strings.HasPrefix(key, "on"):
				return ScriptOutsideText, key
			case scriptAddress(attr.Val):
				return ScriptOutsideText, "javascript:"
			}
		}
	}
	return Served, ""
}

// scriptAddress reports whether an attribute value could be read as a
// javascript: address anywhere in it. A browser drops tabs and line breaks
// from an address and matches the scheme in any case, so those are removed
// before the look; matching anywhere rather than at the start also covers an
// animation's list of values and over-refuses only prose about the scheme.
func scriptAddress(value string) bool {
	folded := strings.Map(func(r rune) rune {
		switch r {
		case '\t', '\n', '\r':
			return -1
		}
		return r
	}, value)
	return strings.Contains(strings.ToLower(folded), "javascript:")
}

// inlineScripts are the text of every script element in one parse, in
// document order: the concatenated text children, which is the text a
// browser hashes. A refused parse never reaches here, so none has a source.
func inlineScripts(doc *html.Node) []string {
	var scripts []string
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode || strings.ToLower(n.Data) != "script" {
			continue
		}
		var text strings.Builder
		for child := range n.ChildNodes() {
			if child.Type == html.TextNode {
				text.WriteString(child.Data)
			}
		}
		scripts = append(scripts, text.String())
	}
	return scripts
}

// attribute returns the value of the attribute named key on n, matched in any
// case and by its qualified name, so xlink:href is found however foreign
// content spelled it.
func attribute(n *html.Node, key string) (string, bool) {
	for _, attr := range n.Attr {
		name := attr.Key
		if attr.Namespace != "" {
			name = attr.Namespace + ":" + attr.Key
		}
		if strings.EqualFold(name, key) || (attr.Namespace == "" && strings.EqualFold(attr.Key, key)) {
			return attr.Val, true
		}
	}
	return "", false
}
