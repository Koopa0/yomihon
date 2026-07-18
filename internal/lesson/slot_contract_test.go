package lesson

import (
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
