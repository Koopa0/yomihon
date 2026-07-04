// Package lesson loads the hand-authored sidecar data that enriches a
// Japanese lesson's reading page. Today that is the slot-machine sentence
// patterns kept at System/slots/*.yaml; the concept-note index the
// lessons link to joins this package when that interaction lands.
//
// Everything here is read-only: the vault owns the data, this package only
// parses and validates it. The slot sidecar's shape — patterns, slots, fills,
// and the closed colour set — is kurodo's own vocabulary expressed as a Go
// struct and checked here, NOT the note frontmatter contract: vault-schema.toml
// is the single source of schema truth for notes, but this machine-owned
// sidecar sits outside that schema.
package lesson

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// slotColors is the closed set of slot colour tokens; each maps to a
// light/dark pair in the stylesheet. Validate rejects any other value so an
// authoring typo surfaces at load rather than shipping a miscoloured card.
// This is kurodo's own vocabulary for the slot sidecar, deliberately not a
// vault-schema.toml enum — that schema governs note frontmatter, not this
// machine-owned file.
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
// placeholder name (e.g. "A"). It carries only yaml tags: the compact JSON the
// slot machine consumes is assembled separately by PatternJSON, so the wire
// shape stays decoupled from the on-disk one.
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

// Sidecar is the slot-machine data for one lesson — the contents of one
// System/slots/Lxx.yaml file. It joins to a lesson note by Slug, never by
// filename: the note carries slug "jp-minna-l01", this file's Slug field
// matches it, and the two filenames (the note's Japanese title, the sidecar's
// Lxx.yaml) deliberately do not.
type Sidecar struct {
	Lesson   string    `yaml:"lesson"`
	Slug     string    `yaml:"slug"`
	Title    string    `yaml:"title"`
	Patterns []Pattern `yaml:"patterns"`
}

// Load reads and parses one slot sidecar file.
func Load(path string) (*Sidecar, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is a System/slots/*.yaml entry under the operator's own vault
	if err != nil {
		return nil, fmt.Errorf("read slot sidecar %s: %w", path, err)
	}
	var s Sidecar
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse slot sidecar %s: %w", path, err)
	}
	return &s, nil
}

// Validate reports every contract problem in the sidecar, one message per
// problem; an empty result means valid. It is the load-time guard against
// freezing wrong content as truth — a template key with no slot, a slot with no
// fills, an unknown colour, or a gloss key absent from the template. It reports
// only, never repairs: kurodo surfaces problems, a human edits the file.
func (s *Sidecar) Validate() []string {
	var problems []string
	for _, p := range s.Patterns {
		keys := TemplateKeys(p.Template)
		inTemplate := make(map[string]bool, len(keys))
		for _, k := range keys {
			inTemplate[k] = true
			pos, ok := p.Slots[k]
			if !ok {
				problems = append(problems, fmt.Sprintf("pattern %q: template key {%s} has no slot", p.ID, k))
				continue
			}
			if len(pos.Fills) == 0 {
				problems = append(problems, fmt.Sprintf("pattern %q: slot %s has no fills", p.ID, k))
			}
			if pos.Color != "" && !slotColors[pos.Color] {
				problems = append(problems, fmt.Sprintf("pattern %q: slot %s has unknown color %q", p.ID, k, pos.Color))
			}
		}
		for _, gk := range TemplateKeys(p.GlossZH) {
			if !inTemplate[gk] {
				problems = append(problems, fmt.Sprintf("pattern %q: gloss key {%s} is not in the template", p.ID, gk))
			}
		}
	}
	return problems
}
