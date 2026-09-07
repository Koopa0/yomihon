package search

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// topicServer answers over notes whose topics are the only place a term
// appears, so a hit proves the topic was indexed and nothing else could have
// produced it.
func topicServer(t *testing.T) *httptest.Server {
	t.Helper()
	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Sources/reading/Stoner.md", Title: "Stoner", NoteType: "writing", Status: "draft",
			Topics: []string{"kindness"}, PlainText: "a novel about a quiet life"},
		{RelPath: "Sources/reading/Stoner-zh.md", Title: "史托納", NoteType: "writing", Status: "draft",
			Topics: []string{"善良"}, PlainText: "一本安靜的小說"},
		{RelPath: "Concepts/Several.md", Title: "Several", NoteType: "concept", Status: "draft",
			Topics: []string{"first-subject", "second-subject"}, PlainText: "unrelated"},
		{RelPath: "Concepts/Plain.md", Title: "Plain", NoteType: "concept", Status: "draft",
			PlainText: "no topic at all"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func getSearch(t *testing.T, srv *httptest.Server, query string, lang wording.Lang) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/search?q="+query, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if lang == wording.En {
		req.Header.Set("Cookie", wording.CookieName+"="+string(wording.En))
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// TestATopicHitSaysWhichSubjectAnsweredTheQuery is the silence the alias
// label already ended: a topic-only row shows a title that does not contain
// the query, and without the subject on the row the reader sees no reason
// for the hit. Both languages name the subject, because a label written in
// one would leave the other reader looking at a title that does not match.
func TestATopicHitSaysWhichSubjectAnsweredTheQuery(t *testing.T) {
	t.Parallel()
	srv := topicServer(t)

	_, zh := getBody(t, srv.Client(), srv.URL+"/search?q=kindness")
	row := resultRow(t, zh, "Sources/reading/Stoner.md")
	named := elementText(t, row, "y-result__topic")
	if !strings.Contains(named, wording.ResultTopicLabel.In(wording.ZhHant)) {
		t.Errorf("the default page does not name the subject in Traditional Chinese: %q", named)
	}
	if !strings.Contains(named, "kindness") {
		t.Errorf("the row does not show the topic that answered: %q", named)
	}
	if !strings.Contains(named, "<mark>") {
		t.Errorf("the row shows the topic but never says which part the query met: %q", named)
	}

	en := getSearch(t, srv, "kindness", wording.En)
	enRow := resultRow(t, en, "Sources/reading/Stoner.md")
	enNamed := elementText(t, enRow, "y-result__topic")
	if !strings.Contains(enNamed, wording.ResultTopicLabel.In(wording.En)) {
		t.Errorf("the English page does not name the subject in English: %q", enNamed)
	}
	if strings.Contains(enNamed, wording.ResultTopicLabel.In(wording.ZhHant)) {
		t.Errorf("the English page still carries the Traditional Chinese label: %q", enNamed)
	}

	_, han := getBody(t, srv.Client(), srv.URL+"/search?q="+("%E5%96%84%E8%89%AF"))
	hanNamed := elementText(t, resultRow(t, han, "Sources/reading/Stoner-zh.md"), "y-result__topic")
	if !strings.Contains(hanNamed, "善良") {
		t.Errorf("the Han topic is missing from the row: %q", hanNamed)
	}

	_, several := getBody(t, srv.Client(), srv.URL+"/search?q=second-subject")
	answering := elementText(t, resultRow(t, several, "Concepts/Several.md"), "y-result__topic")
	if !strings.Contains(answering, "second-subject") {
		t.Errorf("the row does not name the topic that matched: %q", answering)
	}
	if strings.Contains(answering, "first-subject") {
		t.Errorf("the row names a topic the query never touched: %q", answering)
	}

	_, plain := getBody(t, srv.Client(), srv.URL+"/search?q=Plain")
	if strings.Contains(resultRow(t, plain, "Concepts/Plain.md"), "y-result__topic") {
		t.Errorf("a note found by its own title was given a topic attribution:\n%s", plain)
	}
}
