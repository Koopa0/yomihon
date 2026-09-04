package lesson

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadSidecarRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		unknown string
		yaml    string
	}{
		{
			name:    "top level",
			unknown: "unexpected_top",
			yaml: `lesson: L01
slug: jp-minna-l01
title: Example
patterns: []
unexpected_top: rejected
`,
		},
		{
			name:    "pattern",
			unknown: "unexpected_pattern",
			yaml: `lesson: L01
slug: jp-minna-l01
title: Example
patterns:
  - id: p1
    template: "{A}"
    gloss_zh: "{A}"
    note: ""
    slots: {}
    unexpected_pattern: rejected
`,
		},
		{
			name:    "position",
			unknown: "unexpected_position",
			yaml: `lesson: L01
slug: jp-minna-l01
title: Example
patterns:
  - id: p1
    template: "{A}"
    gloss_zh: "{A}"
    note: ""
    slots:
      A:
        label_zh: Subject
        color: topic
        fills: []
        unexpected_position: rejected
`,
		},
		{
			name:    "fill",
			unknown: "unexpected_fill",
			yaml: `lesson: L01
slug: jp-minna-l01
title: Example
patterns:
  - id: p1
    template: "{A}"
    gloss_zh: "{A}"
    note: ""
    slots:
      A:
        label_zh: Subject
        color: topic
        fills:
          - jp: watashi
            reading: watashi
            zh: I
            unexpected_fill: rejected
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSidecar("fixture.yaml", []byte(tt.yaml))
			if err == nil {
				t.Fatalf("parseSidecar() accepted unknown field %q", tt.unknown)
			}
			if !strings.Contains(err.Error(), tt.unknown) {
				t.Errorf("parseSidecar() error = %q, want unknown field %q", err, tt.unknown)
			}
		})
	}
}

func TestLoadSidecarRejectsAdditionalDocuments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "second populated document",
			yaml: `lesson: L01
slug: jp-minna-l01
title: Example
patterns: []
---
lesson: L02
`,
		},
		{
			name: "second empty document",
			yaml: `lesson: L01
slug: jp-minna-l01
title: Example
patterns: []
---
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseSidecar("fixture.yaml", []byte(tt.yaml))
			if err == nil {
				t.Fatal("parseSidecar() accepted more than one YAML document")
			}
			if !strings.Contains(err.Error(), "exactly one YAML document") {
				t.Errorf("parseSidecar() error = %q, want exactly-one-document diagnostic", err)
			}
		})
	}
}

func TestLoadSidecarAcceptsTrailingComments(t *testing.T) {
	t.Parallel()
	const source = `lesson: L01
slug: jp-minna-l01
title: Example
patterns: []
# A comment is still part of the first document, not a second document.
`
	if _, err := parseSidecar("fixture.yaml", []byte(source)); err != nil {
		t.Errorf("parseSidecar() with a trailing comment = %v, want success", err)
	}
}

func TestValidateChecksEveryDeclaredSlotInStableOrder(t *testing.T) {
	t.Parallel()
	positions := make(map[string]Position)
	positions["Zulu"] = Position{Color: "zulu"}
	positions["Beta"] = Position{Color: "topic"}
	positions["Alpha"] = Position{Color: "alpha", Fills: []Fill{{JP: "a"}}}

	sidecar := &Sidecar{Patterns: []Pattern{{
		ID:       "ordered",
		Template: "{Missing}",
		GlossZH:  "{Ghost}",
		Slots:    positions,
	}}}
	want := []string{
		`pattern "ordered": slot Alpha has unknown color "alpha"`,
		`pattern "ordered": slot Beta has no fills`,
		`pattern "ordered": slot Zulu has no fills`,
		`pattern "ordered": slot Zulu has unknown color "zulu"`,
		`pattern "ordered": template key {Missing} has no slot`,
		`pattern "ordered": gloss key {Ghost} is not in the template`,
	}
	if diff := cmp.Diff(want, sidecar.Validate()); diff != "" {
		t.Errorf("Validate() problems mismatch (-want +got):\n%s", diff)
	}
}

// The eight slot colour tokens are written four times: once here as the set the
// sidecar parser accepts, once in the component stylesheet as the class that
// assigns a colour, and once in each palette block that gives the colours their
// values. Nothing tied any copy to another, so a token added here and nowhere
// else parses and renders with no colour at all — a slot the reader sees as
// unmarked, with every test green.
//
// The stylesheets are read, never written. What is checked is the names, in
// both directions each time: a name here with no rule, and a rule with no name
// here, are both worth hearing about.
func TestTheSlotColourTokensAgreeAcrossGoAndTheStylesheets(t *testing.T) {
	t.Parallel()

	declared := slices.Sorted(maps.Keys(slotColors))
	if len(declared) == 0 {
		t.Fatal("no slot colour is declared, so nothing below compares anything")
	}

	components := readStylesheet(t, "components.css")
	assigning := regexp.MustCompile(`\.y-slot-([a-z]+)\s*\{\s*--c:\s*var\(--slot-([a-z]+)\)`)
	classes := make([]string, 0, len(declared))
	for _, match := range assigning.FindAllStringSubmatch(components, -1) {
		if match[1] != match[2] {
			t.Errorf("the class .y-slot-%s reads the colour named --slot-%s", match[1], match[2])
		}
		classes = append(classes, match[1])
	}
	slices.Sort(classes)
	if diff := cmp.Diff(declared, classes); diff != "" {
		t.Errorf("the classes that assign a slot colour differ from the tokens declared here (-declared +stylesheet):\n%s", diff)
	}

	// Every block that names one slot colour has to name all of them: the light
	// palette and each dark one are separate lists, and a token added to one
	// leaves the others rendering the reader's own theme without it.
	blocks := 0
	naming := regexp.MustCompile(`--slot-([a-z]+)\s*:`)
	for block := range strings.SplitSeq(readStylesheet(t, "tokens.css"), "}") {
		found := naming.FindAllStringSubmatch(block, -1)
		if len(found) == 0 {
			continue
		}
		blocks++
		named := make([]string, 0, len(found))
		for _, match := range found {
			named = append(named, match[1])
		}
		slices.Sort(named)
		if diff := cmp.Diff(declared, named); diff != "" {
			t.Errorf("a palette block names a different set of slot colours (-declared +palette):\n%s", diff)
		}
	}
	// The palette is written three times: the light one, and a dark one for
	// each of the two ways a reader can be in the dark. A floor rather than a
	// count would let one of them be deleted with the remaining two agreeing
	// among themselves, which is the drift this exists to catch.
	if blocks != 3 {
		t.Errorf("found %d palette blocks naming slot colours, want 3: the light palette, the chosen dark one and the system dark one", blocks)
	}
}

// readStylesheet reads one of the served stylesheets as a fixture. They are the
// other half of what a slot colour is, and they are read only.
func readStylesheet(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "assets", "css", name)) // #nosec G304 -- a fixed basename under this repository's own assets directory
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}
