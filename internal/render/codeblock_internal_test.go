package render

import (
	"testing"

	"github.com/alecthomas/chroma/v3"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestASecondUnknownLanguageLookupDoesNotReachChroma is the lock on the
// registry memo: a miss is an answer, and the second time the same name is
// asked, chroma is not asked again. The count is the seam, not a duration,
// because a timing assertion is a test you cannot watch fail reliably.
func TestASecondUnknownLanguageLookupDoesNotReachChroma(t *testing.T) {
	t.Parallel()

	cache := newLexerCache(8)
	var calls int
	get := func(string) chroma.Lexer {
		calls++
		return nil
	}

	if got := cache.lookup("d2", get); got != nil {
		t.Fatalf("lookup(d2) = %v, want nil", got)
	}
	if got := cache.lookup("d2", get); got != nil {
		t.Fatalf("second lookup(d2) = %v, want nil", got)
	}
	if calls != 1 {
		t.Errorf("chroma was reached %d times, want 1 (the miss is a cacheable answer)", calls)
	}
}

// TestALexerCacheDoesNotGrowPastItsBound holds the other half of the
// ruling: an author's invented info-strings cannot grow the map without
// limit, and they cannot push out a name that already fit.
func TestALexerCacheDoesNotGrowPastItsBound(t *testing.T) {
	t.Parallel()

	cache := newLexerCache(2)
	var calls int
	get := func(string) chroma.Lexer {
		calls++
		return nil
	}

	_ = cache.lookup("a", get)
	_ = cache.lookup("a", get)
	if calls != 1 {
		t.Fatalf("first key: chroma was reached %d times, want 1", calls)
	}
	_ = cache.lookup("b", get)
	_ = cache.lookup("b", get)
	if calls != 2 {
		t.Fatalf("second key: chroma was reached %d times, want 2", calls)
	}

	beforeOverflow := calls
	_ = cache.lookup("c", get)
	_ = cache.lookup("c", get)
	if calls != beforeOverflow+2 {
		t.Errorf("a key past the bound was remembered: chroma was reached %d times after the overflow pair, want %d", calls, beforeOverflow+2)
	}

	beforeKept := calls
	_ = cache.lookup("a", get)
	if calls != beforeKept {
		t.Errorf("a key that fit was evicted by a later typo: chroma was reached again for %q", "a")
	}
}

// TestProductionLexerLookupsReachTheMemo is the wiring lock the type-level
// tests cannot be. Those drive a fresh cache directly, so they stay green if
// renderCodeBlock and lexerFor go back to asking chroma themselves. This one
// calls the production entry points with names nothing else uses, and the
// process-wide memo holding that name is the proof the call site went through
// it. Unwiring any of the three lookups makes the matching assertion fail.
func TestProductionLexerLookupsReachTheMemo(t *testing.T) {
	const (
		file = "koo76-wiring-lock.zzq"
		lang = "koo76-wiring-lock-fence"
	)

	_ = SourceHTML(file, "x")
	if !lexerMemoHas(lexerFiles, file) {
		t.Fatal("SourceHTML never reached the memo")
	}

	New(graph.BuildFromNotes(nil, nil), noBodies{}, anyTitle{}, holdsEverything{}).HTML(
		"note.md", "", "```"+lang+"\nx\n```\n", wording.ZhHant,
	)
	if !lexerMemoHas(lexerNames, lang) {
		t.Fatal("renderCodeBlock never reached the memo")
	}

	_ = SourceHTML("koo76-wiring-lock.canvas", "{}")
	if !lexerMemoHas(lexerNames, "JSON") {
		t.Fatal("SourceHTML alias lookup never reached the memo")
	}
}

func lexerMemoHas(c *lexerCache, key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.m[key]
	return ok
}
