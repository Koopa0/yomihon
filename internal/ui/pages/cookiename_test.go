package pages

import (
	"os"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestClientWritesTheCookieTheServerReads holds the two halves of one name
// together. The server reads the language cookie by a constant and the client
// writes it by a literal, and nothing until now compared them: they agreed
// because both were typed the same day, which is not a mechanism. Renaming one
// leaves the other reading a cookie nobody writes, and the symptom is a control
// that appears to do nothing — the click stores a choice, the reload asks for
// the page, and the page comes back in the language it was already in.
func TestClientWritesTheCookieTheServerReads(t *testing.T) {
	t.Parallel()

	const path = "../../../assets/js/preferences.js"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	script := string(source)
	if !strings.Contains(script, "data-lang-toggle") {
		t.Fatalf("%s no longer carries the language control, so this test asserts nothing about it", path)
	}
	if !strings.Contains(script, wording.CookieName+"=") {
		t.Errorf("%s writes no cookie named %q; the server reads that name and would never see the choice", path, wording.CookieName)
	}
}
