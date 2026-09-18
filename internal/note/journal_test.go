package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestJournalMonthComesFromTheAddress keeps the calendar answering the reader
// rather than the clock. The month a reader names in the address is the month
// they are shown, whatever month it happens to be today; an address that names
// something which is not a month falls back to the month they are in, which is
// a page rather than a refusal.
//
// The whole page is built from that month, so this is also what lets every
// recording and every probe of the journal name one outright and go on saying
// the same thing next month.
func TestJournalMonthComesFromTheAddress(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for rel, body := range map[string]string{
		"Diary/2026-07-31.md": "the last day of July\n",
		"Diary/2026-11-02.md": "a day in November\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	// The month this test is being run in, which is the answer an address that
	// names no readable month has to give.
	today := time.Now().Format("2006-01")

	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "a named month", url: "/journal?month=2026-07", want: "2026-07"},
		{name: "another named month", url: "/journal?month=2026-11", want: "2026-11"},
		{name: "no month named", url: "/journal", want: today},
		{name: "a month that is not one", url: "/journal?month=banana", want: today},
		{name: "a whole day where a month belongs", url: "/journal?month=2026-07-31", want: today},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := get(t, srv.Client(), srv.URL+tt.url)
			if code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", tt.url, code)
			}
			// Every square of the month carries its own day, so the month drawn
			// is read off the squares rather than off the words above them,
			// which change with the reader's language.
			day := `data-journal-day="` + tt.want + `-`
			if !strings.Contains(body, day) {
				t.Errorf("GET %s drew no day of %s", tt.url, tt.want)
			}
			for _, other := range []string{"2026-07", "2026-11", today} {
				if other == tt.want {
					continue
				}
				if strings.Contains(body, `data-journal-day="`+other+`-`) {
					t.Errorf("GET %s drew a day of %s as well as of %s", tt.url, other, tt.want)
				}
			}
		})
	}
}
