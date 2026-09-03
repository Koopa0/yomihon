package lesson

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// placeholder matches a {Key} slot reference in a template or gloss string.
var placeholder = regexp.MustCompile(`\{([A-Za-z0-9]+)\}`)

// Segment is one piece of a parsed template: literal text (Key == "") or a
// slot reference (Key == the slot name).
type Segment struct {
	Text string
	Key  string
}

// ParseTemplate splits a template like "{A}は {B}です" into ordered segments:
// [{Key:"A"}, {Text:"は "}, {Key:"B"}, {Text:"です"}].
func ParseTemplate(tmpl string) []Segment {
	var segs []Segment
	last := 0
	for _, loc := range placeholder.FindAllStringSubmatchIndex(tmpl, -1) {
		if loc[0] > last {
			segs = append(segs, Segment{Text: tmpl[last:loc[0]]})
		}
		segs = append(segs, Segment{Key: tmpl[loc[2]:loc[3]]})
		last = loc[1]
	}
	if last < len(tmpl) {
		segs = append(segs, Segment{Text: tmpl[last:]})
	}
	return segs
}

// TemplateKeys returns the slot keys in first-appearance order, deduplicated.
func TemplateKeys(tmpl string) []string {
	var keys []string
	seen := map[string]bool{}
	for _, m := range placeholder.FindAllStringSubmatch(tmpl, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			keys = append(keys, m[1])
		}
	}
	return keys
}

// AbstractTemplate renders the template with each {Key} shown as its bare key,
// e.g. "{A}は {B}です" -> "Aは Bです" — the card's heading label.
func AbstractTemplate(tmpl string) string {
	return placeholder.ReplaceAllString(tmpl, "$1")
}

// FirstFill returns a slot's first candidate — the server-rendered default —
// or the zero Fill when the slot has none.
func FirstFill(p Position) Fill {
	if len(p.Fills) > 0 {
		return p.Fills[0]
	}
	return Fill{}
}

// GlossInitial renders the gloss with each {Key} replaced by its first fill's
// Chinese, the default the page shows before any interaction. Only template
// keys are substituted, so this default and PatternJSON agree.
func GlossInitial(p Pattern) string {
	inTemplate := make(map[string]bool)
	for _, k := range TemplateKeys(p.Template) {
		inTemplate[k] = true
	}
	return placeholder.ReplaceAllStringFunc(p.GlossZH, func(tok string) string {
		key := tok[1 : len(tok)-1]
		if pos, ok := p.Slots[key]; ok && inTemplate[key] && len(pos.Fills) > 0 {
			return pos.Fills[0].ZH
		}
		return tok
	})
}

// jsSlot and jsPattern are the compact wire shape embedded in each card's
// slot-data blob, kept separate from the on-disk structs.
type jsSlot struct {
	Color string `json:"color"`
	Fills []Fill `json:"fills"`
}

type jsPattern struct {
	Template string            `json:"template"`
	Gloss    string            `json:"gloss"`
	Keys     []string          `json:"keys"`
	Slots    map[string]jsSlot `json:"slots"`
}

// PatternJSON serialises a pattern into the slot-data blob. json.Marshal
// escapes < > &, so the blob cannot break out of its <script> element even if
// a fill holds markup — the sole reason this is not string assembly.
func PatternJSON(p Pattern) string {
	keys := TemplateKeys(p.Template)
	js := jsPattern{
		Template: p.Template,
		Gloss:    p.GlossZH,
		Keys:     keys,
		Slots:    map[string]jsSlot{},
	}
	for _, k := range keys {
		pos := p.Slots[k]
		js.Slots[k] = jsSlot{Color: pos.Color, Fills: pos.Fills}
	}
	b, err := json.Marshal(js)
	if err != nil {
		// The wire struct holds only strings, slices of them and a
		// string-keyed map, so nothing in it can refuse to encode.
		panic(fmt.Sprintf("lesson: pattern %q cannot be encoded: %v", p.ID, err))
	}
	return string(b)
}
