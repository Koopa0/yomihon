package search

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

// governedStatusView opens a real writer over the shared test contract. It
// is a real contract rather than a stand-in because the question under test —
// whether a value belongs to a type's declared list — is the contract's answer
// and nothing else's.
func governedStatusView(t *testing.T) status.Authority {
	t.Helper()
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}
	return openStatusView(t, contract, contract.Governance())
}

// outOfEnumIndex holds two concept notes the query "needle" finds: one whose
// status the default group declares, one whose status it does not.
func outOfEnumIndex(t *testing.T) *lexical.Index {
	t.Helper()
	return lexical.NewIndex([]lexical.Document{
		{RelPath: "Concepts/legal.md", Title: "Legal", NoteType: "concept", Status: "draft", PlainText: "needle body"},
		{RelPath: "Concepts/outside.md", Title: "Outside", NoteType: "concept", Status: "reviewing", PlainText: "needle body"},
	}, validArtifactPolicy(t))
}

func searchResultsBody(t *testing.T, snapshot func() RequestSnapshot, query string) string {
	t.Helper()
	h := NewHandler(snapshot, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search/results?q="+query, http.NoBody)
	rr := httptest.NewRecorder()
	h.results(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	return rr.Body.String()
}

// linksToNote reports whether html holds a result link to relPath. The note's
// address is where such an href starts rather than the whole of it: a row whose
// excerpt marked something carries the words the query found after it, so the
// browser opens the note where they are.
func linksToNote(html, relPath string) bool {
	address := `href="/notes/` + relPath
	return strings.Contains(html, address+`"`) || strings.Contains(html, address+`#`)
}

// resultRow cuts one hit out of a rendered results fragment by the note it
// links to, so an assertion about one row cannot be answered by another row.
func resultRow(t *testing.T, body, relPath string) string {
	t.Helper()
	for _, chunk := range strings.Split(body, `<li>`)[1:] {
		row, _, terminated := strings.Cut(chunk, "</li>")
		if !terminated {
			t.Fatalf("unterminated result row: %q", chunk)
		}
		if linksToNote(row, relPath) {
			return row
		}
	}
	t.Fatalf("no result row links to %q; body = %q", relPath, body)
	return ""
}

// A search result row carried its note's status as bare text, so a value the
// schema does not allow was indistinguishable from one it does. The reader
// who could act on it was the one least likely to open every hit. The row now
// says so in words a reader sees without hovering and a listener hears in the
// link's own name — never colour alone, and never text carried out of sight.
func TestSearchRowNamesAStatusOutsideItsTypesEnum(t *testing.T) {
	t.Parallel()
	idx := outOfEnumIndex(t)
	view := governedStatusView(t)
	body := searchResultsBody(t, func() RequestSnapshot {
		return RequestSnapshot{
			Index:  idx,
			Shell:  nav.Shell{Nav: &nav.Model{}, Governed: true},
			Status: view,
		}
	}, "needle")

	outside := resultRow(t, body, "Concepts/outside.md")
	if !strings.Contains(outside, "不在 schema 允許清單中") {
		t.Errorf("the row says nothing about the value the schema does not allow; row = %q", outside)
	}
	// The words must sit in the row's own text, not in a title attribute and
	// not behind the offscreen class: a reader looking straight at the list is
	// the one this exists for.
	visible := outside
	if _, hidden, found := strings.Cut(outside, "y-offscreen"); found {
		visible = strings.Replace(outside, hidden, "", 1)
	}
	if !strings.Contains(visible, "不在 schema 允許清單中") {
		t.Errorf("the warning is carried only out of sight; row = %q", outside)
	}
	if strings.Contains(outside, "title=") {
		t.Errorf("the row explains itself only on hover; row = %q", outside)
	}

	legal := resultRow(t, body, "Concepts/legal.md")
	if strings.Contains(legal, "不在 schema 允許清單中") {
		t.Errorf("a declared status is flagged; row = %q", legal)
	}
	if !strings.Contains(legal, "draft") {
		t.Errorf("the declared row lost its status; row = %q", legal)
	}
}

// closedStatusView opens a writer over a folder whose contract exists and
// could not be read. It is governed and shut: the state that answers "not
// declared" to every value while a contract is genuinely in force, which is
// the one that would paint the warning on every row in the vault.
func closedStatusView(t *testing.T) status.Authority {
	t.Helper()
	return openStatusView(t, nil, schema.Unreadable(errors.New("contract unreadable")))
}

// openStatusView opens one writer over an empty folder and hands back its
// read-only view. The three cases differ only in what authority they were
// opened under, so they differ only in these two arguments.
func openStatusView(t *testing.T, contract *schema.Contract, governance schema.Governance) status.Authority {
	t.Helper()
	reader, err := vaultfs.Open(t.TempDir())
	if err != nil {
		t.Fatalf("vaultfs.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	writer, err := status.Open(reader, contract, governance, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("status.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := writer.Close(); closeErr != nil {
			t.Errorf("Writer.Close() error = %v", closeErr)
		}
	})
	return writer.Authority()
}

// ungovernedStatusView opens a writer over a folder that declared no
// contract. It is the ordinary shape of any directory yomihon is pointed at,
// and it governs nothing — so it has no list to find a value missing from.
func ungovernedStatusView(t *testing.T) status.Authority {
	t.Helper()
	return openStatusView(t, nil, schema.Ungoverned())
}

// A write face that knows no vocabulary can accuse nothing. Both shapes of not
// knowing answer false to every question about a declared value, so ruling
// with either would flag every governed row in the vault — and the suite would
// not notice, because a row that gained a warning still carries everything the
// other tests look for. They are separate branches in the row's own guard, so
// they are separate rows here.
func TestSearchRowAccusesNothingWhenTheWriteFaceIsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		authority func(*testing.T) status.Authority
	}{
		{name: "no status vocabulary was supplied at all"},
		{name: "a contract is in force and could not be read", authority: closedStatusView},
		{name: "the folder carries no contract at all", authority: ungovernedStatusView},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := outOfEnumIndex(t)
			snap := RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
			if tt.authority != nil {
				snap.Status = tt.authority(t)
			}
			body := searchResultsBody(t, func() RequestSnapshot { return snap }, "needle")

			if strings.Contains(body, "不在 schema 允許清單中") {
				t.Errorf("a status vocabulary that declares nothing still ruled on a status value; body = %q", body)
			}
			// The control: the rows themselves are present, so the assertion
			// above is not passing over an empty answer.
			if got := strings.Count(body, "<li>"); got != 2 {
				t.Errorf("rendered result rows = %d, want 2; body = %q", got, body)
			}
		})
	}
}
