package status_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestRecoveryPageHoldsItsWayBackOnlyWhenItKnowsTheVersion pins the second place
// a page invites a reader to reload. A refusal that got as far as binding a
// version can hold its own invitation until the reading generation holds that
// version too; a refusal that never had one has nothing to hold against and
// keeps the plain link it always carried.
func TestRecoveryPageHoldsItsWayBackOnlyWhenItKnowsTheVersion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	writeNote(t, root, lessonContent("draft"))
	srv := newHandlerServer(t, writer)

	identity := formIdentity(lessonContent("draft"))
	// The page claims a status the file does not carry, so the write is refused
	// after it has bound itself to these bytes.
	_, _, refused := postStatus(t, srv, url.Values{
		"path": {testRel}, "from": {"imported"}, "to": {schema.SealStatus},
		"content_identity": {identity},
	})
	if !strings.Contains(refused, `data-freshness-path="`+testRel+`"`) {
		t.Errorf("a refusal that bound a version does not name the note to watch; body = %q", refused)
	}
	if !strings.Contains(refused, `data-freshness-identity="`+identity+`"`) {
		t.Errorf("a refusal that bound a version does not carry it; body = %q", refused)
	}

	// This one is refused before any version is read, so there is no version the
	// invitation could wait for.
	_, _, unbound := postStatus(t, srv, url.Values{
		"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus},
	})
	if strings.Contains(unbound, "data-freshness-") {
		t.Errorf("a refusal with no version of its own still marks the page as watchable; body = %q", unbound)
	}
}
