package status_test

import (
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestHandlerWritesOnlyTheNoteTheFormNamed holds the submitted path to the
// note's name rather than to a field worth tidying.
//
// Every case puts a second note beside the target carrying byte-identical
// content, which is what makes the failure silent: a request that resolves to
// the neighbour also satisfies the content check the form binds itself to, so
// the wrong file is rewritten and reported as a success. The assertion is
// therefore on the bytes of every file in the vault, not only on the response.
//
// The notes sit inside the knowledge layer the fixture contract declares, so
// the layer lets each request through to the question under test. The one
// request that misspells the layer's own folder is kept, and pins where its
// answer comes from instead.
func TestHandlerWritesOnlyTheNoteTheFormNamed(t *testing.T) {
	t.Parallel()

	const neighbour = "Writing/Space.md"
	body := lessonContent("draft")

	for _, tc := range []struct {
		name     string
		onDisk   map[string]string
		postPath string
		wantCode int
	}{
		{
			name:     "a leading space names a different note",
			onDisk:   map[string]string{neighbour: body},
			postPath: "Writing/ Space.md",
			wantCode: http.StatusNotFound,
		},
		{
			// A name ending in a space does not end in ".md", so the scan's
			// own definition of a note answers this one before the
			// filesystem is asked.
			name:     "a trailing space names something that is not a note",
			onDisk:   map[string]string{neighbour: body},
			postPath: neighbour + " ",
			wantCode: http.StatusUnprocessableEntity,
		},
		{
			name:     "an ideographic space names a different note",
			onDisk:   map[string]string{neighbour: body},
			postPath: "Writing/　Space.md",
			wantCode: http.StatusNotFound,
		},
		{
			// The first folder is what the knowledge layer reads, and the
			// contract named "Writing", not " Writing": a tree the layer never
			// named is refused before the filesystem is asked about it.
			name:     "a space on the first folder names a tree outside the layer",
			onDisk:   map[string]string{"Writing/lessons/japanese/Space.md": body},
			postPath: " Writing/lessons/japanese/Space.md",
			wantCode: http.StatusUnprocessableEntity,
		},
		{
			// Deeper down, the spelling is the filesystem's to answer.
			name:     "a space on a later folder names a different tree",
			onDisk:   map[string]string{"Writing/lessons/japanese/Space.md": body},
			postPath: "Writing/ lessons/japanese/Space.md",
			wantCode: http.StatusNotFound,
		},
		{
			// Room alone is a name too, and it is not a note's.
			name:     "a path of nothing but space names nothing",
			onDisk:   map[string]string{neighbour: body},
			postPath: "  ",
			wantCode: http.StatusUnprocessableEntity,
		},
		{
			// The same answer on every volume: a case-insensitive one would
			// open the neighbour for this spelling, and must not.
			name:     "a spelling the folder does not hold names no note",
			onDisk:   map[string]string{neighbour: body},
			postPath: "Writing/SPACE.md",
			wantCode: http.StatusNotFound,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writer := newWriter(t, root, loadContract(t))
			for rel, content := range tc.onDisk {
				writeVaultFile(t, root, rel, content)
			}
			srv := newHandlerServer(t, writer)

			code, _, _ := postStatus(t, srv, url.Values{
				"path":             {tc.postPath},
				"from":             {"draft"},
				"to":               {schema.SealStatus},
				"content_identity": {formIdentity(body)},
			})
			if code != tc.wantCode {
				t.Errorf("status = %d, want %d", code, tc.wantCode)
			}
			for rel, want := range tc.onDisk {
				assertVaultFileUnchanged(t, root, rel, want)
			}
		})
	}
}

// TestHandlerWritesANoteWhoseNameHoldsASpace is the other side of the same
// rule: a note really named with a leading space is reachable, and the note
// beside it is not touched.
func TestHandlerWritesANoteWhoseNameHoldsASpace(t *testing.T) {
	t.Parallel()

	const spaced = "Writing/ Space.md"
	const neighbour = "Writing/Space.md"
	body := lessonContent("draft")

	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	writeVaultFile(t, root, spaced, body)
	writeVaultFile(t, root, neighbour, body)
	srv := newHandlerServer(t, writer)

	code, _, _ := postStatus(t, srv, url.Values{
		"path":             {spaced},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {formIdentity(body)},
	})
	if code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", code, http.StatusSeeOther)
	}
	got := readVaultFile(t, root, spaced)
	if !strings.Contains(got, "status: "+schema.SealStatus) {
		t.Errorf("the note named with a space was not the one written:\n%s", got)
	}
	assertVaultFileUnchanged(t, root, neighbour, body)
}

// TestHandlerReadsTheOtherFormFieldsAsSubmitted covers the remaining values
// the form carries. A status the contract does not spell, and an identity
// with room around it, are values the page never renders; taking them as
// written keeps what the form said and what the writer acted on the same
// bytes.
func TestHandlerReadsTheOtherFormFieldsAsSubmitted(t *testing.T) {
	t.Parallel()

	body := lessonContent("draft")
	for _, tc := range []struct {
		name     string
		form     url.Values
		wantCode int
	}{
		{
			name:     "a padded from is not the status the note carries",
			form:     url.Values{"from": {" draft"}, "to": {schema.SealStatus}, "content_identity": {formIdentity(body)}},
			wantCode: http.StatusConflict,
		},
		{
			name:     "a padded to is not a status the contract spells",
			form:     url.Values{"from": {"draft"}, "to": {schema.SealStatus + " "}, "content_identity": {formIdentity(body)}},
			wantCode: http.StatusUnprocessableEntity,
		},
		{
			name:     "a padded identity is not an identity",
			form:     url.Values{"from": {"draft"}, "to": {schema.SealStatus}, "content_identity": {" " + formIdentity(body)}},
			wantCode: http.StatusUnprocessableEntity,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writer := newWriter(t, root, loadContract(t))
			writeNote(t, root, body)
			srv := newHandlerServer(t, writer)

			form := url.Values{"path": {testRel}}
			maps.Copy(form, tc.form)
			code, _, _ := postStatus(t, srv, form)
			if code != tc.wantCode {
				t.Errorf("status = %d, want %d", code, tc.wantCode)
			}
			assertVaultFileUnchanged(t, root, testRel, body)
		})
	}
}

func readVaultFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a fixed in-test path under this test's TempDir
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func assertVaultFileUnchanged(t *testing.T, root, rel, want string) {
	t.Helper()
	if diff := cmp.Diff(want, readVaultFile(t, root, rel)); diff != "" {
		t.Errorf("%s was rewritten (-want +got):\n%s", rel, diff)
	}
}
