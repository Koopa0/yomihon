package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lesson"
)

func TestSlotMachineRendersPatternData(t *testing.T) {
	t.Parallel()
	view := &lesson.Sidecar{Patterns: []lesson.Pattern{{
		ID:       "plain",
		Template: "{A}です",
		GlossZH:  "{A}。",
		Slots: map[string]lesson.Position{
			"A": {
				Color: "topic",
				Fills: []lesson.Fill{{JP: "私", Reading: "わたし", ZH: "我"}},
			},
		},
	}}}
	data := `{"template":"{A}です","gloss":"{A}。","keys":["A"],"slots":{"A":{"color":"topic","fills":[{"jp":"私","reading":"わたし","zh":"我"}]}}}`
	tests := []struct {
		name      string
		nonce     string
		wantNonce string
	}{
		{name: "response nonce", nonce: "response-nonce", wantNonce: "response-nonce"},
		{name: "attribute delimiter", nonce: `bad" data-injected="true`, wantNonce: `bad&#34; data-injected=&#34;true`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := SlotMachine(view, tt.nonce).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render slot machine: %v", err)
			}
			want := `<script nonce="` + tt.wantNonce + `" type="application/json" class="y-slotdata">` + data + `</script>`
			if html := buf.String(); !strings.Contains(html, want) {
				t.Errorf("SlotMachine() = %q, want slot data %q", html, want)
			}
		})
	}
}
