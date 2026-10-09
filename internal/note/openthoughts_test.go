package note_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
)

// Catches dropping a kept mark after a rebuild, retaining a dead fragment,
// treating a lost pair as an inadmissible addition, and hiding Home's fault row.
func TestOpenThoughtsRetainsLostUncertaintyMarks(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		lang     string
		noteGone bool
		missing  string
		remove   string
		count    string
	}{
		{name: "english note gone", lang: "en", noteGone: true, missing: "Note not found", remove: "Remove mark", count: "1 item"},
		{name: "chinese note gone", lang: "zh-Hant", noteGone: true, missing: "找不到這篇筆記", remove: "移除標記", count: "1 筆"},
		{name: "english place gone", lang: "en", missing: "Place not found", remove: "Remove mark", count: "1 item"},
		{name: "chinese place gone", lang: "zh-Hant", missing: "找不到這個位置", remove: "移除標記", count: "1 筆"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			const rel = "Lost <note>.md"
			file := filepath.Join(root, rel)
			if err := os.WriteFile(file, []byte("---\ntitle: Own title\n---\n# Own title\n\n## Chapter\n\nWords.\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			log := slog.New(slog.DiscardHandler)
			store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
			writer := openStatusWriter(t, source, nil, schema.Ungoverned())
			marks, err := mark.New(t.TempDir(), source.Name())
			if err != nil {
				t.Fatal(err)
			}
			current := store.Current()
			handler := note.New(&note.Sources{
				Source: source, Status: writer.Authority,
				Snapshot:         func() *snapshot.Generation { return current },
				RequestReconcile: func() {},
				ObservedStatus:   writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt,
				Continuation: noMark, Uncertainties: marks.Uncertainties,
				UncertaintyAddress: mark.UncertaintyAddress, Log: log,
			})
			mux := http.NewServeMux()
			handler.Register(mux)
			mark.NewUncertaintyHandler(marks, handler, log).Register(mux)
			form := url.Values{"path": {rel}, "anchor": {"chapter"}}.Encode()
			if diff := cmp.Diff("{\"marked\":true}\n", lostMarkRequest(t, mux, http.MethodPost, "/uncertainties", form, tt.lang)); diff != "" {
				t.Fatalf("add mark mismatch (-want +got):\n%s", diff)
			}
			assertLostMarkPair(t, mux, tt.lang, rel, "chapter")
			before := lostMarkRecords(t, mux, tt.lang)
			if tt.noteGone {
				if removeErr := os.Remove(file); removeErr != nil {
					t.Fatal(removeErr)
				}
			} else {
				if writeErr := os.WriteFile(file, []byte("---\ntitle: Own title\n---\n# Own title\n\n## Renamed\n\nWords.\n"), 0o600); writeErr != nil {
					t.Fatal(writeErr)
				}
			}
			rebuilt, err := snapshot.New(t.Context(), source, log, nil, schema.Ungoverned())
			if err != nil {
				t.Fatal(err)
			}
			current = rebuilt.Current()
			t.Log("YOMIHON_996_LOST_MARK_DRIVER_INVOKED")
			for _, address := range []string{"/", "/open-thoughts"} {
				page := lostMarkRequest(t, mux, http.MethodGet, address, "", tt.lang)
				assertLostMarksUnchanged(t, mux, tt.lang, before)
				marker, warning := "data-index-row", "y-row__measure--warn"
				if address == "/" {
					_, block, ok := strings.Cut(page, `data-home-block="open-thoughts"`)
					if !ok {
						t.Fatal("caught: Home omitted the lost mark block")
					}
					page, _, ok = strings.Cut(block, "</section>")
					if !ok {
						t.Fatal("Home block is not closed")
					}
					marker, warning = "data-desk-item", "ui-navitem__count--warn"
				}
				text := "Own title #chapter"
				if tt.noteGone {
					text = "Lost &lt;note&gt;.md"
				}
				buttonAt := strings.Index(page, "data-uncertainty-remove")
				button := ""
				outsideLink := false
				if buttonAt >= 0 {
					start := strings.LastIndex(page[:buttonAt], "<button")
					if start >= 0 {
						end := strings.Index(page[buttonAt:], ">")
						if end >= 0 {
							button = page[start : buttonAt+end+1]
						}
						outsideLink = strings.LastIndex(page[:start], "<a ") <= strings.LastIndex(page[:start], "</a>")
					}
				}
				_, classes, hasClasses := strings.Cut(button, `class="`)
				classes, _, closedClasses := strings.Cut(classes, `"`)
				gatedRemove := hasClasses && closedClasses && slices.Contains(strings.Fields(classes), "y-markset")
				got := struct {
					Rows                                                                                                  int
					Text, Missing, Fault, Count, Link, DeadFragment, Remove, Pair, NativeRemove, OutsideLink, GatedRemove bool
				}{
					Rows: strings.Count(page, marker), Text: strings.Contains(page, text), Missing: strings.Contains(page, tt.missing), Fault: strings.Contains(page, warning), Count: strings.Contains(page, tt.count),
					Link: strings.Contains(page, `href="/notes/Lost%20%3Cnote%3E.md"`), DeadFragment: strings.Contains(page, `href="/notes/Lost%20%3Cnote%3E.md#chapter"`),
					Remove:       strings.Contains(page, "data-uncertainty-remove") && strings.Contains(page, tt.remove),
					NativeRemove: strings.Contains(button, `type="button"`) && strings.Contains(button, "disabled"), OutsideLink: outsideLink, GatedRemove: gatedRemove,
					Pair: strings.Contains(page, `data-uncertainty-endpoint="/uncertainties"`) && strings.Contains(page, `data-uncertainty-path="Lost &lt;note&gt;.md"`) && strings.Contains(page, `data-uncertainty-anchor="chapter"`),
				}
				want := struct {
					Rows                                                                                                  int
					Text, Missing, Fault, Count, Link, DeadFragment, Remove, Pair, NativeRemove, OutsideLink, GatedRemove bool
				}{Rows: 1, Text: true, Missing: true, Fault: true, Count: true, Link: !tt.noteGone, Remove: true, Pair: true, NativeRemove: true, OutsideLink: true, GatedRemove: true}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("caught: GET %s lost mark mismatch (-want +got):\n%s", address, diff)
				}
			}
			// Reading must preserve every stored field, including the assigned time.
			assertLostMarksUnchanged(t, mux, tt.lang, before)
			if diff := cmp.Diff("{\"marked\":false}\n", lostMarkRequest(t, mux, http.MethodPost, "/uncertainties", form, tt.lang)); diff != "" {
				t.Errorf("caught: remove mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff("[]\n", lostMarkRequest(t, mux, http.MethodGet, "/uncertainties", "", tt.lang)); diff != "" {
				t.Errorf("caught: removed marks mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func lostMarkRequest(t *testing.T, mux http.Handler, method, address, body, lang string) string {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, address, strings.NewReader(body))
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.AddCookie(&http.Cookie{Name: "yomihon_lang", Value: lang}) // #nosec G124 -- synthetic incoming Cookie header; response-cookie security attributes are not transmitted
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("%s %s status = %d, want 200; body = %q", method, address, w.Code, w.Body.String())
	}
	return w.Body.String()
}

// Catches re-capturing the provider to classify a mark after the page already
// captured its row title. The second real generation has lost the heading.
func TestOpenThoughtsClassifiesInsideItsCapturedGeneration(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		advance bool
	}{
		{name: "one stable generation"},
		{name: "rebuild between provider reads", advance: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			file := filepath.Join(root, "Source.md")
			if err := os.WriteFile(file, []byte("---\ntitle: Captured title\n---\n## Chapter\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			log := slog.New(slog.DiscardHandler)
			first, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
			writer := openStatusWriter(t, source, nil, schema.Ungoverned())
			marks, err := mark.New(t.TempDir(), source.Name())
			if err != nil {
				t.Fatal(err)
			}
			reads := 0
			second := first
			handler := note.New(&note.Sources{
				Source: source, Status: writer.Authority,
				Snapshot: func() *snapshot.Generation {
					reads++
					if tt.advance && reads > 1 {
						return second.Current()
					}
					return first.Current()
				},
				RequestReconcile: func() {},
				ObservedStatus:   writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt,
				Continuation: noMark, Uncertainties: marks.Uncertainties,
				UncertaintyAddress: mark.UncertaintyAddress, Log: log,
			})
			marked, err := marks.ToggleUncertainty(&mark.Uncertainty{RelPath: "Source.md", Anchor: "chapter", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)}, func(u *mark.Uncertainty) bool { return handler.HasPlace(u.RelPath, u.Anchor) })
			if err != nil {
				t.Fatal(err)
			}
			if !marked {
				t.Fatal("initial real mark was not added")
			}
			initial := http.NewServeMux()
			mark.NewUncertaintyHandler(marks, handler, log).Register(initial)
			assertLostMarkPair(t, initial, "en", "Source.md", "chapter")
			if writeErr := os.WriteFile(file, []byte("---\ntitle: Later title\n---\n## Renamed\n"), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
			second, err = snapshot.New(t.Context(), source, log, nil, schema.Ungoverned())
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			handler.Register(mux)
			for _, address := range []string{"/", "/open-thoughts"} {
				reads = 0
				page := lostMarkRequest(t, mux, http.MethodGet, address, "", "en")
				got := struct {
					CapturedTitle, CapturedLink, LaterTitle, Missing bool
				}{
					CapturedTitle: strings.Contains(page, "Captured title #chapter"), CapturedLink: strings.Contains(page, `href="/notes/Source.md#chapter"`),
					LaterTitle: strings.Contains(page, "Later title"), Missing: strings.Contains(page, "Place not found"),
				}
				want := struct {
					CapturedTitle, CapturedLink, LaterTitle, Missing bool
				}{CapturedTitle: true, CapturedLink: true}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("caught: GET %s mixed generations (-want +got):\n%s", address, diff)
				}
			}
		})
	}
}

func assertLostMarkPair(t *testing.T, mux http.Handler, lang, rel, anchor string) {
	t.Helper()
	var held []mark.Uncertainty
	listed := lostMarkRequest(t, mux, http.MethodGet, "/uncertainties", "", lang)
	if err := json.Unmarshal([]byte(listed), &held); err != nil {
		t.Fatal(err)
	}
	if len(held) != 1 {
		t.Fatalf("stored marks = %d, want 1", len(held))
	}
	type pair struct {
		Path, Anchor string
	}
	if diff := cmp.Diff(pair{Path: rel, Anchor: anchor}, pair{Path: held[0].RelPath, Anchor: held[0].Anchor}); diff != "" {
		t.Fatalf("stored identity mismatch (-want +got):\n%s", diff)
	}
}

// Catches sorting after lost-row classification: the lost path must occupy its
// stored position before Home cuts to five, and the total still includes six.
func TestOpenThoughtsSortsStoredLocationsBeforeHomeNarrowing(t *testing.T) {
	root := t.TempDir()
	for _, fixture := range []struct {
		rel  string
		text string
	}{
		{rel: "A.md", text: "---\ntitle: Z title\n---\n## Alpha\n\n## Beta\n"},
		{rel: "B.md", text: "---\ntitle: A title\n---\nWords.\n"},
		{rel: "Missing.md", text: "Words.\n"},
		{rel: "Newest.md", text: "---\ntitle: Newest\n---\nWords.\n"},
		{rel: "Oldest.md", text: "---\ntitle: Oldest\n---\nWords.\n"},
	} {
		if err := os.WriteFile(filepath.Join(root, fixture.rel), []byte(fixture.text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	writer := openStatusWriter(t, source, nil, schema.Ungoverned())
	marks, err := mark.New(t.TempDir(), source.Name())
	if err != nil {
		t.Fatal(err)
	}
	current := store.Current()
	handler := note.New(&note.Sources{
		Source: source, Status: writer.Authority,
		Snapshot:         func() *snapshot.Generation { return current },
		RequestReconcile: func() {},
		ObservedStatus:   writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt,
		Continuation: noMark, Uncertainties: marks.Uncertainties,
		UncertaintyAddress: mark.UncertaintyAddress, Log: log,
	})
	for _, kept := range []mark.Uncertainty{
		{RelPath: "Oldest.md", At: time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)},
		{RelPath: "Missing.md", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
		{RelPath: "B.md", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
		{RelPath: "A.md", Anchor: "beta", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
		{RelPath: "A.md", Anchor: "alpha", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
		{RelPath: "Newest.md", At: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)},
	} {
		marked, markErr := marks.ToggleUncertainty(&kept, func(u *mark.Uncertainty) bool { return handler.HasPlace(u.RelPath, u.Anchor) })
		if markErr != nil {
			t.Fatal(markErr)
		}
		if !marked {
			t.Fatalf("initial mark %s #%s was not added", kept.RelPath, kept.Anchor)
		}
	}
	held, err := marks.Uncertainties()
	if err != nil {
		t.Fatal(err)
	}
	wantStored := []mark.Uncertainty{
		{RelPath: "Oldest.md", At: time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)},
		{RelPath: "Missing.md", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
		{RelPath: "B.md", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
		{RelPath: "A.md", Anchor: "beta", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
		{RelPath: "A.md", Anchor: "alpha", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
		{RelPath: "Newest.md", At: time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)},
	}
	if diff := cmp.Diff(wantStored, held); diff != "" {
		t.Fatalf("initial complete stored sort fixture mismatch (-want +got):\n%s", diff)
	}
	if removeErr := os.Remove(filepath.Join(root, "Missing.md")); removeErr != nil {
		t.Fatal(removeErr)
	}
	rebuilt, err := snapshot.New(t.Context(), source, log, nil, schema.Ungoverned())
	if err != nil {
		t.Fatal(err)
	}
	current = rebuilt.Current()
	mux := http.NewServeMux()
	handler.Register(mux)
	mark.NewUncertaintyHandler(marks, handler, log).Register(mux)
	for _, tt := range []struct {
		lang  string
		count string
	}{
		{lang: "en", count: "6 items"},
		{lang: "zh-Hant", count: "6 筆"},
	} {
		t.Run(tt.lang, func(t *testing.T) {
			page := lostMarkRequest(t, mux, http.MethodGet, "/", "", tt.lang)
			assertLostMarksUnchanged(t, mux, tt.lang, wantStored)
			_, block, ok := strings.Cut(page, `data-home-block="open-thoughts"`)
			if !ok {
				t.Fatal("caught: Home omitted stored candidates")
			}
			block, _, ok = strings.Cut(block, "</section>")
			if !ok {
				t.Fatal("Home block is not closed")
			}
			var titles []string
			for _, fragment := range strings.Split(block, "data-desk-item")[1:] {
				// The dated row leads with the date span, then the authored title.
				_, afterDate, ok := strings.Cut(fragment, "</span>")
				if !ok {
					t.Fatal("dated desk row has no date span")
				}
				_, titleElement, ok := strings.Cut(afterDate, "<span")
				if !ok {
					t.Fatal("dated desk row has no title span")
				}
				_, titleText, ok := strings.Cut(titleElement, ">")
				if !ok {
					t.Fatal("desk title span has no opening end")
				}
				title, _, ok := strings.Cut(titleText, "</span>")
				if !ok {
					t.Fatal("desk title span has no closing end")
				}
				titles = append(titles, title)
			}
			got := struct {
				Titles    []string
				FullCount bool
			}{Titles: titles, FullCount: strings.Contains(block, tt.count)}
			want := struct {
				Titles    []string
				FullCount bool
			}{Titles: []string{"Newest", "Z title #alpha", "Z title #beta", "A title", "Missing.md"}, FullCount: true}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("caught: stored-field Home ordering mismatch (-want +got):\n%s", diff)
			}
			all := lostMarkRequest(t, mux, http.MethodGet, "/open-thoughts", "", tt.lang)
			assertLostMarksUnchanged(t, mux, tt.lang, wantStored)
			if diff := cmp.Diff(6, strings.Count(all, "data-index-row")); diff != "" {
				t.Errorf("caught: complete stored shelf count mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func lostMarkRecords(t *testing.T, mux http.Handler, lang string) []mark.Uncertainty {
	t.Helper()
	listed := lostMarkRequest(t, mux, http.MethodGet, "/uncertainties", "", lang)
	var held []mark.Uncertainty
	if err := json.Unmarshal([]byte(listed), &held); err != nil {
		t.Fatal(err)
	}
	return held
}

func assertLostMarksUnchanged(t *testing.T, mux http.Handler, lang string, want []mark.Uncertainty) {
	t.Helper()
	if diff := cmp.Diff(want, lostMarkRecords(t, mux, lang)); diff != "" {
		t.Errorf("caught: reading changed complete stored marks (-want +got):\n%s", diff)
	}
}
