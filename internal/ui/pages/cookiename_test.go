package pages

import (
	"os"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestClientReadsTheCookieTheServerWrites holds the two halves of one name
// together. The server stores the language choice under a constant and the
// client's cache-restore check reads it by a literal, and nothing else
// compares them: they agree because both were typed the same day, which is not
// a mechanism. Renaming one leaves the restore check reading a cookie nobody
// writes, and the symptom is quiet — every revived page compares the document
// against the default language and a reader who chose English gets silently
// returned to a page the cookie says they left.
func TestClientReadsTheCookieTheServerWrites(t *testing.T) {
	t.Parallel()

	const path = "../../../assets/js/preferences.js"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	script := string(source)
	if !strings.Contains(script, "pageshow") {
		t.Fatalf("%s no longer watches cache restores, so this test asserts nothing about them", path)
	}
	if !strings.Contains(script, `readCookie('`+wording.CookieName+`')`) {
		t.Errorf("%s does not read the cookie named %q; the restore check would compare against a value nobody stores", path, wording.CookieName)
	}
	// The choice itself is stored by the server now: a client-side write here
	// would be a second author for the same fact, and the two could disagree
	// about the value's shape.
	if strings.Contains(script, wording.CookieName+"=") {
		t.Errorf("%s still writes the %q cookie; the language endpoint owns that write", path, wording.CookieName)
	}
}
