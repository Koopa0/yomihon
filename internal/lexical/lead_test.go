package lexical

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func leadOf(t *testing.T, body, query string) Lead {
	t.Helper()
	doc := DocumentFromNote(&vault.Note{RelPath: "Notes/q.md", Body: body})
	idx := NewIndex([]Document{doc}, schema.ArtifactPolicy{})
	got, err := idx.Search(Parse(query), 10)
	if err != nil || len(got.Results) != 1 {
		t.Fatalf("search %q: %d results, err %v", query, len(got.Results), err)
	}
	return got.Results[0].Lead
}

func plain(runs []ExcerptRun) string {
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

func marked(runs []ExcerptRun) string {
	var b strings.Builder
	for _, r := range runs {
		if r.Hit {
			b.WriteString(r.Text)
		}
	}
	return b.String()
}

func TestALeadOpensAtTheSentenceWithTheHit(t *testing.T) {
	t.Parallel()

	lead := leadOf(t, "First we greet everyone here. Then cache invalidation is hard. After that more words follow.\n", "cache")
	if got := plain(lead.Runs); got != "…Then cache invalidation is hard. After that more words follow." {
		t.Errorf("lead = %q, want it to open at the sentence holding the hit", got)
	}
	if marked(lead.Runs) != "cache" {
		t.Errorf("marked = %q, want the hit", marked(lead.Runs))
	}
	zh := leadOf(t, "這是第一句。快取失效很難處理，所以要小心。第三句在這裡。\n", "快取")
	if got := plain(zh.Runs); !strings.HasPrefix(got, "…快取失效") {
		t.Errorf("zh lead = %q, want it to open at 。", got)
	}
}

func TestALeadIsNeverAHeadingAndIsOneParagraph(t *testing.T) {
	t.Parallel()

	body := "## About cache\n\nNothing relevant here at all.\n\nThe cache is in the second paragraph, which goes on.\n\nA third paragraph that must not be joined.\n"
	got := plain(leadOf(t, body, "cache").Runs)
	if got != "The cache is in the second paragraph, which goes on." {
		t.Errorf("lead = %q, want the one paragraph that holds the hit, not the heading or its neighbours", got)
	}
	if lead := leadOf(t, "## About cache\n\nNothing relevant here.\n", "cache"); len(lead.Runs) != 0 {
		t.Errorf("a hit only in a heading leaves the lead empty, got %q", plain(lead.Runs))
	}
}

func TestALeadFromAListIsOneItem(t *testing.T) {
	t.Parallel()

	got := plain(leadOf(t, "- first item\n- the cache item is here\n- third item\n", "cache").Runs)
	if got != "the cache item is here" {
		t.Errorf("lead = %q, want the one item", got)
	}
}

func TestACodeOnlyHitLeadsWithTheLineOfCode(t *testing.T) {
	t.Parallel()

	lead := leadOf(t, "prose with none\n\n```go\nfunc cache() {}\nx := 1\n```\n", "cache")
	if !lead.Code || !strings.HasPrefix(plain(lead.Runs), "func cache() {}") || strings.Contains(plain(lead.Runs), "x := 1") {
		t.Errorf("lead = %+v %q, want the one line of code", lead, plain(lead.Runs))
	}
}

func TestAHitFarIntoASentenceGivesTheNarrowLeadItsOwnOpening(t *testing.T) {
	t.Parallel()

	body := "Every sentence like this one wanders through a good many words before it finally reaches the cache, and then goes on a while longer too.\n"
	lead := leadOf(t, body, "cache")
	if len(lead.Near) == 0 {
		t.Fatalf("no narrow lead for a hit far in: %q", plain(lead.Runs))
	}
	if got := plain(lead.Near); !strings.Contains(got, "cache") || !strings.HasPrefix(got, "…") || len(got) >= len(plain(lead.Runs)) {
		t.Errorf("near = %q, want it to open near the hit and be shorter than %q", got, plain(lead.Runs))
	}
	short := leadOf(t, "A short cache sentence.\n", "cache")
	if len(short.Near) != 0 {
		t.Errorf("a hit at the start needs no narrow lead, got %q", plain(short.Near))
	}
}

func TestNearWordIsOneEditFromATitleWord(t *testing.T) {
	t.Parallel()

	docs := []Document{
		DocumentFromNote(&vault.Note{RelPath: "Notes/Cache design.md", Body: "# Cache design\n\nbody\n"}),
		DocumentFromNote(&vault.Note{RelPath: "Notes/Cache again.md", Body: "# Cache again\n\nbody\n"}),
		DocumentFromNote(&vault.Note{RelPath: "Notes/Caches.md", Body: "# Cachet\n\nbody\n"}),
	}
	idx := NewIndex(docs, schema.ArtifactPolicy{})
	for query, want := range map[string]string{
		"cahce":  "cache", // two neighbours swapped
		"cach":   "cache", // a letter left out; "cachet" is one edit too but used once
		"cachee": "cache", // a letter added
		"cacxe":  "cache", // a letter changed
		"cache":  "",      // already a word
		"zzzz":   "",
		"ca":     "",
		"快取":     "",
	} {
		if got := idx.NearWord(query); got != want {
			t.Errorf("NearWord(%q) = %q, want %q", query, got, want)
		}
	}
}

func TestALeadShowsALinksDisplayAndNotTheNameOfItsTarget(t *testing.T) {
	t.Parallel()

	lead := leadOf(t, "See [[lru-cache-implementation|手寫 LRU cache]] for the details.\n", "cache")
	if got := plain(lead.Runs); got != "See 手寫 LRU cache for the details." {
		t.Errorf("lead = %q, want the display only", got)
	}
}
