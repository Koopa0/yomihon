package layouts

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestSystemDarkPreferenceGetsTheWholeDarkPalette holds the stylesheet's two
// dark entrances together. A reader who chose dark enters through the
// [data-theme="dark"] attribute the server stamps; a reader who chose nothing
// on a dark system enters through the prefers-color-scheme media block. Both
// must land in the same room: the media block is a second copy of the dark
// declarations, and a value edited in one and not the other would give the
// unchosen dark a subtly different palette nobody designed. The comparison is
// declaration-for-declaration so any drift names the exact token.
//
// The media rule excludes only an explicit light choice, because a stored
// choice must keep beating the system either way — dark-on-dark repeats the
// same values, light-on-dark escapes the block entirely.
func TestSystemDarkPreferenceGetsTheWholeDarkPalette(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/tokens.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := string(source)

	chosen := cssDeclarations(t, ruleBody(t, css, `[data-theme="dark"] {`))

	mediaAt := strings.Index(css, "@media (prefers-color-scheme: dark)")
	if mediaAt < 0 {
		t.Fatalf("%s has no prefers-color-scheme dark block; a reader who chose nothing gets light on a dark system", path)
	}
	media := css[mediaAt:]
	const guard = `:root:not([data-theme="light"]) {`
	if !strings.Contains(media[:strings.Index(media, "{")+200], guard) {
		t.Fatalf("the media block does not guard on %q; an explicit light choice must escape it", guard)
	}
	system := cssDeclarations(t, ruleBody(t, media, guard))

	if len(chosen) < 10 || chosen["--bg"] == "" || chosen["color-scheme"] != "dark" {
		t.Fatalf("the chosen-dark block parsed implausibly (%d declarations); the comparison below would prove nothing", len(chosen))
	}
	if diff := cmp.Diff(chosen, system); diff != "" {
		t.Errorf("the two dark entrances disagree (-chosen +system):\n%s", diff)
	}
}

// ruleBody returns the text between the named rule opener and its closing
// brace. The token blocks under test nest nothing, so the first unmatched
// close brace ends the body.
func ruleBody(t *testing.T, css, opener string) string {
	t.Helper()
	at := strings.Index(css, opener)
	if at < 0 {
		t.Fatalf("stylesheet has no rule %q", opener)
	}
	body := css[at+len(opener):]
	end := strings.IndexByte(body, '}')
	if end < 0 {
		t.Fatalf("rule %q is not closed", opener)
	}
	return body[:end]
}

// cssDeclarations parses one rule body into property -> value, with comments
// stripped and whitespace folded so formatting differences cannot register as
// palette drift.
func cssDeclarations(t *testing.T, body string) map[string]string {
	t.Helper()
	body = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(body, "")
	out := map[string]string{}
	for decl := range strings.SplitSeq(body, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		property, value, ok := strings.Cut(decl, ":")
		if !ok {
			t.Fatalf("unparsable declaration %q", decl)
		}
		out[strings.TrimSpace(property)] = strings.Join(strings.Fields(value), " ")
	}
	return out
}
