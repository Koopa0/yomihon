// Package lesson loads the hand-authored data that enriches lesson reading
// pages: Japanese slot-machine sentence patterns from System/slots/*.yaml and
// the cross-domain concept-note index used by in-page sheets. Everything here
// is read-only, and the sidecar's shape is yomihon's own vocabulary rather
// than the note frontmatter contract vault-schema.toml governs.
package lesson

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"

	"gopkg.in/yaml.v3"
)

// slotColors is the closed set of slot colour tokens, each mapping to a
// light/dark pair in the stylesheet. It is this sidecar's own vocabulary, not
// a vault-schema.toml enum: that schema governs note frontmatter.
var slotColors = map[string]bool{
	"topic": true, "pred": true, "poss": true, "obj": true,
	"place": true, "time": true, "verb": true, "num": true,
}

// Fill is one selectable value for a slot: the Japanese surface form, its kana
// reading (copied verbatim from the lesson's ruby), and the Chinese gloss.
type Fill struct {
	JP      string `yaml:"jp" json:"jp"`
	Reading string `yaml:"reading" json:"reading"`
	ZH      string `yaml:"zh" json:"zh"`
}

// Position is one replaceable slot in a pattern template, keyed by its
// placeholder name. It carries only yaml tags; PatternJSON assembles the
// compact JSON the slot machine consumes.
type Position struct {
	LabelZH string `yaml:"label_zh"`
	Color   string `yaml:"color"` // one of slotColors, or empty for a neutral (uncoloured) slot
	Fills   []Fill `yaml:"fills"`
}

// Pattern is one sentence frame, e.g. "{A}は {B}です", with the fills that may
// drop into each of its {Key} positions.
type Pattern struct {
	ID       string              `yaml:"id"`
	Template string              `yaml:"template"` // {Key} placeholders
	GlossZH  string              `yaml:"gloss_zh"`
	Note     string              `yaml:"note"`
	Slots    map[string]Position `yaml:"slots"`
}

// Sidecar is the slot-machine data for one lesson. It joins to a lesson note
// by Slug, never by filename: the note's slug and this file's Slug field
// match, while their filenames deliberately do not.
type Sidecar struct {
	Lesson string `yaml:"lesson"`
	Slug   string `yaml:"slug"`
	Title  string `yaml:"title"`
	// Note is what this drill does not cover, in the author's own words — a
	// rule holding across the lesson has nowhere to go among the patterns.
	Note     string    `yaml:"note"`
	Patterns []Pattern `yaml:"patterns"`
}

func parseSidecar(source string, data []byte) (*Sidecar, error) {
	var s Sidecar
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&s); err != nil {
		return nil, fmt.Errorf("parse slot sidecar %s: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("parse slot sidecar %s: %w", source, err)
		}
		return nil, fmt.Errorf("parse slot sidecar %s: exactly one YAML document is allowed", source)
	}
	return &s, nil
}

// Validate reports every contract problem in the sidecar, one message per
// problem; an empty result means valid. It reports only and never repairs: a
// human edits the file.
func (s *Sidecar) Validate() []string {
	var problems []string
	for _, p := range s.Patterns {
		problems = append(problems, p.validate()...)
	}
	return problems
}

func (p Pattern) validate() []string {
	problems := p.validateSlots()
	keys := TemplateKeys(p.Template)
	inTemplate := make(map[string]bool, len(keys))
	for _, k := range keys {
		inTemplate[k] = true
		if _, ok := p.Slots[k]; !ok {
			problems = append(problems, fmt.Sprintf("pattern %q: template key {%s} has no slot", p.ID, k))
		}
	}
	for _, k := range TemplateKeys(p.GlossZH) {
		if !inTemplate[k] {
			problems = append(problems, fmt.Sprintf("pattern %q: gloss key {%s} is not in the template", p.ID, k))
		}
	}
	return problems
}

func (p Pattern) validateSlots() []string {
	var problems []string
	for _, k := range slices.Sorted(maps.Keys(p.Slots)) {
		pos := p.Slots[k]
		if len(pos.Fills) == 0 {
			problems = append(problems, fmt.Sprintf("pattern %q: slot %s has no fills", p.ID, k))
		}
		if pos.Color != "" && !slotColors[pos.Color] {
			problems = append(problems, fmt.Sprintf("pattern %q: slot %s has unknown color %q", p.ID, k, pos.Color))
		}
	}
	return problems
}
