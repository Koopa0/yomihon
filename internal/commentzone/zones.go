// Package commentzone is the one reading of Obsidian %%...%% comment spans.
// An unpaired mark runs to the end of the body, as Obsidian hides it too.
// Sequence and the judge consume this scan so a parked row cannot be a
// lesson on the shelf and invisible on the page.
package commentzone

import "strings"

// Span is a half-open byte range [Start, Stop) into a body.
type Span struct{ Start, Stop int }

// Contains reports whether off falls in the range.
func (s Span) Contains(off int) bool {
	return off >= s.Start && off < s.Stop
}

// In reports whether off falls in any of the ranges.
func In(zones []Span, off int) bool {
	for _, z := range zones {
		if z.Contains(off) {
			return true
		}
	}
	return false
}

// Zones are the Obsidian %%...%% spans in body. A mark whose start sits in
// code is ignored so it cannot shift the pairing; an unpaired trailing mark
// runs to the end of the body, as Obsidian hides everything after it.
func Zones(body string, code []Span) []Span {
	var marks []int
	for off := 0; ; {
		rel := strings.Index(body[off:], "%%")
		if rel < 0 {
			break
		}
		at := off + rel
		if !In(code, at) {
			marks = append(marks, at)
		}
		off = at + 2
	}
	var zones []Span
	for k := 0; k < len(marks); k += 2 {
		start := marks[k]
		if k+1 < len(marks) {
			zones = append(zones, Span{Start: start, Stop: marks[k+1] + 2})
			continue
		}
		zones = append(zones, Span{Start: start, Stop: len(body)})
	}
	return zones
}
