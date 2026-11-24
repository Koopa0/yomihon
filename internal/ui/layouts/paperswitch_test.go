package layouts

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// paperForm is what one option of the header's paper switch posts, read back
// out of the rendered markup.
type paperForm struct {
	Option  string
	Action  string
	Method  string
	Fields  map[string]string
	Pressed string
}

var (
	paperFormPattern = regexp.MustCompile(`(?s)<form class="y-paperswitch__form"([^>]*)>(.*?)</form>`)
	attrPattern      = regexp.MustCompile(`([a-z-]+)="([^"]*)"`)
	inputPattern     = regexp.MustCompile(`<input type="hidden" name="([a-z]+)" value="([^"]*)">`)
)

func attrsOf(tag string) map[string]string {
	out := map[string]string{}
	for _, m := range attrPattern.FindAllStringSubmatch(tag, -1) {
		out[m[1]] = m[2]
	}
	return out
}

func renderPaperForms(t *testing.T, c *Chrome) []paperForm {
	t.Helper()
	var buf bytes.Buffer
	if err := header(*c).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	var out []paperForm
	for _, m := range paperFormPattern.FindAllStringSubmatch(buf.String(), -1) {
		form := attrsOf(m[1])
		fields := map[string]string{}
		for _, in := range inputPattern.FindAllStringSubmatch(m[2], -1) {
			fields[in[1]] = in[2]
		}
		_, button, _ := strings.Cut(m[2], "<button")
		button, _, _ = strings.Cut(button, ">")
		b := attrsOf(button)
		out = append(out, paperForm{
			Option: b["data-paper-option"], Action: form["action"], Method: form["method"],
			Fields: fields, Pressed: b["aria-pressed"],
		})
	}
	return out
}

// TestThePaperSwitchPostsWhatEachGroundSets holds the switch to working with no
// script: each option is a form posting to the page that stores reading
// choices, a paper sets the light theme with it, and the night lamp sets dark
// without touching the paper kept for the day. Every one returns the reader to
// the page they pressed it on.
func TestThePaperSwitchPostsWhatEachGroundSets(t *testing.T) {
	t.Parallel()
	got := renderPaperForms(t, &Chrome{Theme: "light", Paper: "washi", ReturnTo: "/notes/A.md?x=1"})
	post := func(option string, fields map[string]string, pressed string) paperForm {
		return paperForm{Option: option, Action: "/preferences", Method: "post", Fields: fields, Pressed: pressed}
	}
	want := []paperForm{
		post("stone", map[string]string{"theme": "light", "paper": "stone", "next": "/notes/A.md?x=1"}, "false"),
		post("washi", map[string]string{"theme": "light", "paper": "washi", "next": "/notes/A.md?x=1"}, "true"),
		post("white", map[string]string{"theme": "light", "paper": "white", "next": "/notes/A.md?x=1"}, "false"),
		post("dark", map[string]string{"theme": "dark", "next": "/notes/A.md?x=1"}, "false"),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("header paper switch (-want +got):\n%s", diff)
	}
}

// TestThePaperSwitchPressesWhatTheServerKnows pins aria-pressed as rendered. A
// stored dark theme is the night lamp, a stored light theme is the stored
// paper; a reader following the system is answered with none pressed, because
// only the browser knows the system's answer and its script sets it before a
// press can happen.
func TestThePaperSwitchPressesWhatTheServerKnows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		theme string
		paper string
		want  []string
	}{
		{name: "stored dark", theme: "dark", paper: "washi", want: []string{"false", "false", "false", "true"}},
		{name: "stored light, stone", theme: "light", paper: "stone", want: []string{"true", "false", "false", "false"}},
		{name: "stored light, white", theme: "light", paper: "white", want: []string{"false", "false", "true", "false"}},
		{name: "following the system", theme: "", paper: "stone", want: []string{"false", "false", "false", "false"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, form := range renderPaperForms(t, &Chrome{Theme: tt.theme, Paper: tt.paper}) {
				got = append(got, form.Pressed)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("aria-pressed for stone, washi, white, dark (-want +got):\n%s", diff)
			}
		})
	}
}
