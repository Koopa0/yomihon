package note_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The pair the side-by-side tests are driven against: one note and its
// translation, in one folder, the translation naming the original. Between
// them they carry every way a note names a place inside itself — a heading
// whose words are the same in both, a footnote numbered from one in each, an
// address the author wrote at one of their own sections — plus a frontmatter
// key the contract does not know, which puts a block of schema findings in
// each column, and a slug that joins a practice card whose own parts are
// numbered from one per card.
func comparePairVault() map[string]string {
	lesson := func(title, lang, extra, section, body string) string {
		return "---\ntitle: " + title + "\ntype: lesson\ndomain: japanese\nstatus: draft\n" +
			"created: 2026-06-01\nupdated: 2026-06-01\nlang: " + lang + "\nslug: " + strings.ToLower(title) + "\n" +
			"unknown_to_the_contract: yes\n" + extra + "---\n\n" +
			body + "[^one]\n\nGo to [that part](#" + section + ").\n\n" +
			"## Ledger\n\nThe heading both halves keep.\n\n" +
			"## " + strings.ToUpper(section[:1]) + section[1:] + "\n\nThe other one.\n\n" +
			"[^one]: The note's own footnote.\n"
	}
	return map[string]string{
		"Writing/Cutover.md": lesson("Cutover", "en", "", "afterward", "The original half"),
		"Writing/Cutoverzh.md": lesson("Cutoverzh", "zh-Hant",
			"based_on: \"[[Cutover]]\"\n", "afterward", "The translated half"),
		"System/slots/cutover.yaml": "lesson: Cutover\nslug: cutover\ntitle: Cutover\npatterns:\n" +
			"  - id: p1\n    template: \"{A}\"\n    gloss_zh: \"{A}\"\n    slots:\n" +
			"      A: {label_zh: \"甲\", color: topic, fills: [{jp: 一, reading: いち, zh: 一}, {jp: 二, reading: に, zh: 二}]}\n",
		"System/slots/cutoverzh.yaml": "lesson: Cutoverzh\nslug: cutoverzh\ntitle: Cutoverzh\npatterns:\n" +
			"  - id: p1\n    template: \"{A}\"\n    gloss_zh: \"{A}\"\n    slots:\n" +
			"      A: {label_zh: \"甲\", color: topic, fills: [{jp: 一, reading: いち, zh: 一}, {jp: 二, reading: に, zh: 二}]}\n",
	}
}

const (
	compareAddress = "/compare/Writing/Cutover.md?with=Writing%2FCutoverzh.md"
	columnAMark    = `id="compare-a"`
	columnBMark    = `id="compare-b"`
)

var (
	// The three shapes one element reaches another by, as this tree writes
	// them. The reference list is every attribute HTML and WAI-ARIA define for
	// the purpose, not the ones written today: an attribute this test does not
	// know about is one it would report as reaching nothing, which is the
	// answer that gets looked at.
	elementName    = regexp.MustCompile(` id="([^"]*)"`)
	elementAddress = regexp.MustCompile(` href="#([^"]*)"`)
	elementRef     = regexp.MustCompile(
		` (?:form|for|list|headers|itemref|popovertarget|` +
			`aria-activedescendant|aria-controls|aria-describedby|aria-details|` +
			`aria-errormessage|aria-flowto|aria-labelledby|aria-owns)="([^"]*)"`)
)

// TestNoAddressInsideAColumnLeavesIt is the whole reason a column is rendered
// under a name of its own. Two notes each number their headings, footnotes and
// practice cards from their own text, so on one page the same name lands on two
// elements — and a reader following the right column's contents entry, footnote
// marker or an address its author wrote at one of their own sections arrives in
// the left column's copy of it. That is worse than a duplicate name, because
// nothing about it looks wrong.
func TestNoAddressInsideAColumnLeavesIt(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, writeNotes(t, comparePairVault()), loadHomeContract(t))
	code, body := get(t, srv.Client(), srv.URL+compareAddress)
	if code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", compareAddress, code)
	}

	columns := map[string]string{"the opened note": columnOf(t, body, columnAMark, columnBMark), "the note beside it": columnOf(t, body, columnBMark, "</main>")}
	for where, column := range columns {
		names := captured(elementName, column)
		addresses := captured(elementAddress, column)
		if len(names) < 2 || len(addresses) == 0 {
			t.Fatalf("%s carries %d names and %d addresses, so this test asks almost nothing of it", where, len(names), len(addresses))
		}
		for _, address := range addresses {
			target, err := url.PathUnescape(address)
			if err != nil {
				t.Errorf("%s has an address that is not a readable name: %q", where, address)
				continue
			}
			if !slices.Contains(names, target) {
				t.Errorf("%s addresses %q, which is not an element in it — the reader lands in the other note", where, target)
			}
		}
		for _, value := range captured(elementRef, column) {
			for named := range strings.FieldsSeq(value) {
				if !slices.Contains(names, named) {
					t.Errorf("%s describes an element by %q, which is not in it", where, named)
				}
			}
		}
	}

	// Two elements answering to one name is the other half of the same fault,
	// and it reaches the parts of a column no address points at.
	all := captured(elementName, body)
	seen := make(map[string]bool, len(all))
	for _, name := range all {
		if seen[name] {
			t.Errorf("two elements on the page answer to %q", name)
		}
		seen[name] = true
	}
	// A practice card numbers its own parts from one, so a page carrying two
	// cards is the case a renamed body alone would not have covered.
	if !strings.Contains(body, "y-slotcard") {
		t.Fatalf("no practice card reached the page, so nothing here exercises a name written after the body was rendered")
	}
}

// columnOf cuts one column out of the page. The two are marked by the names
// their own strip of links addresses them by, and the cut is by those names
// rather than by element nesting: a column holds sections of its own, so
// matching a closing tag would stop inside one.
func columnOf(t *testing.T, body, from, to string) string {
	t.Helper()
	start := strings.Index(body, from)
	if start < 0 {
		t.Fatalf("the page has no column marked %s", from)
	}
	rest := body[start:]
	end := strings.Index(rest, to)
	if end < 0 {
		t.Fatalf("the column marked %s does not end at %s", from, to)
	}
	return rest[:end]
}

func captured(re *regexp.Regexp, in string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(in, -1) {
		out = append(out, m[1])
	}
	return out
}

// TestACompareColumnLinksBackAndShowsItsStatus holds the two things the page
// adds around a note it otherwise reproduces: the way to the note's own page,
// where it can be adjudicated, and the status word it carries now. No control
// that could change that word appears here.
func TestACompareColumnLinksBackAndShowsItsStatus(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, writeNotes(t, comparePairVault()), loadHomeContract(t))
	code, body := get(t, srv.Client(), srv.URL+compareAddress)
	if code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", compareAddress, code)
	}
	for _, back := range []string{`href="/notes/Writing/Cutover.md"`, `href="/notes/Writing/Cutoverzh.md"`} {
		if !strings.Contains(body, back) {
			t.Errorf("no column links back to its own page: %s missing", back)
		}
	}
	if got := strings.Count(body, `ui-status ui-status--draft`); got != 2 {
		t.Errorf("the page shows %d status words, want one per column", got)
	}
	if strings.Contains(body, `action="/status"`) {
		t.Error("a column offers a control that would write a status; adjudication belongs on the note's own page")
	}
}

// TestComparingWithoutASecondNoteReadsTheFirstAlone covers the three ways a
// reader can arrive here with only one note: they named no second one, they
// named this same one, or they named something the vault cannot show. In each
// the note the address opened is the honest answer, and the not-found page
// would tell them nothing to act on.
func TestComparingWithoutASecondNoteReadsTheFirstAlone(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, writeNotes(t, comparePairVault()), loadHomeContract(t))
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, tt := range []struct {
		name   string
		target string
	}{
		{"no second note named", "/compare/Writing/Cutover.md"},
		{"the same note named twice", "/compare/Writing/Cutover.md?with=Writing%2FCutover.md"},
		{"a second note the vault does not hold", "/compare/Writing/Cutover.md?with=Writing%2FNobody.md"},
		{"a second path that is not a note", "/compare/Writing/Cutover.md?with=System%2Fslots%2Fcutover.yaml"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+tt.target, http.NoBody)
			if err != nil {
				t.Fatalf("build GET %s: %v", tt.target, err)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatalf("GET %s: %v", tt.target, err)
			}
			defer func() {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Errorf("close body: %v", closeErr)
				}
			}()
			if response.StatusCode != http.StatusFound {
				t.Fatalf("GET %s = %d, want 302", tt.target, response.StatusCode)
			}
			if got := response.Header.Get("Location"); got != "/notes/Writing/Cutover.md" {
				t.Errorf("GET %s redirected to %q, want the note the address opened", tt.target, got)
			}
			if got := response.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("GET %s kept the answer (Cache-Control %q); the missing note may be written a second later", tt.target, got)
			}
		})
	}
}

// pairNote is one note of a candidate pair, written with everything the
// contract asks for so the only thing under test is which other note is
// offered beside it.
func pairNote(title, language, extra string) string {
	return "---\ntitle: " + title + "\ntype: lesson\ndomain: japanese\nstatus: draft\n" +
		"created: 2026-06-01\nupdated: 2026-06-01\nlang: " + language + "\n" + extra + "---\n\nbody\n"
}

// offeredPartner is the note a reading page offers to be read beside, or "".
// The offer is a link to the side-by-side address, so the partner is read back
// out of that address rather than out of the words around it.
func offeredPartner(t *testing.T, body string) string {
	t.Helper()
	at := strings.Index(body, `class="y-pair"`)
	if at < 0 {
		return ""
	}
	m := regexp.MustCompile(`href="/compare/[^"?]*\?with=([^"]*)"`).FindStringSubmatch(body[at:])
	if m == nil {
		t.Fatalf("the offer block carries no side-by-side address: %q", body[at:min(at+300, len(body))])
	}
	partner, err := url.QueryUnescape(m[1])
	if err != nil {
		t.Fatalf("the offered address is not readable: %v", err)
	}
	return partner
}

// TestWhichNoteIsOfferedBesideThisOne walks the relation itself. A declaration
// is the author's own claim and a shared filename is this program's inference,
// so a declaration wins wherever there is exactly one; and more than one
// candidate at the winning level is no offer, because choosing between two
// names would be a guess.
func TestWhichNoteIsOfferedBesideThisOne(t *testing.T) {
	t.Parallel()
	root := writeNotes(t, map[string]string{
		// Declared one way. Both halves are offered the other, since the
		// original has no way to name the translation itself.
		"Writing/Alpha.md":   pairNote("Alpha", "en", ""),
		"Writing/Alphazh.md": pairNote("Alphazh", "zh-Hant", "based_on: \"[[Alpha]]\"\n"),

		// Inferred: one folder, one filename opening with the other's, two
		// declared languages, and nothing declared either way.
		"Writing/Beta.md":   pairNote("Beta", "en", ""),
		"Writing/Betazh.md": pairNote("Betazh", "zh-Hant", ""),

		// The same shape in one language, which is two notes whose names begin
		// alike and not a translation of anything.
		"Writing/Gamma.md":     pairNote("Gamma", "en", ""),
		"Writing/Gammalong.md": pairNote("Gammalong", "en", ""),

		// Two inferred candidates, so the inference answers with neither.
		"Writing/Delta.md":   pairNote("Delta", "en", ""),
		"Writing/Deltazh.md": pairNote("Deltazh", "zh-Hant", ""),
		"Writing/Deltaja.md": pairNote("Deltaja", "ja", ""),

		// Two declarations, so the declaration answers with neither either.
		// Its sources are its own, since a note named by a second note's
		// declaration has two candidates of its own and would answer with
		// neither for a reason this row is not about.
		"Writing/Epsone.md": pairNote("Epsone", "en", ""),
		"Writing/Epstwo.md": pairNote("Epstwo", "en", ""),
		"Writing/Eps.md":    pairNote("Eps", "en", "based_on:\n  - Epsone\n  - Epstwo\n"),
		// A sibling that would be inferred if the ambiguous declaration fell
		// through to the inference, which it must not: the author named two
		// sources, and answering with a name they did not write would be this
		// page choosing for them.
		"Writing/Epszh.md": pairNote("Epszh", "zh-Hant", ""),

		// Three candidates with exactly one declared: the declaration is the
		// author's claim and it decides.
		"Writing/Zeta.md":   pairNote("Zeta", "en", ""),
		"Writing/Zetazh.md": pairNote("Zetazh", "zh-Hant", ""),
		"Writing/Zetaja.md": pairNote("Zetaja", "ja", "based_on: \"[[Zeta]]\"\n"),
	})
	srv := newServerWithContract(t, root, loadHomeContract(t))

	for _, tt := range []struct {
		name    string
		note    string
		partner string
	}{
		{"the note a translation declares is offered that translation", "Alpha", "Writing/Alphazh.md"},
		{"the translation is offered the note it declares", "Alphazh", "Writing/Alpha.md"},
		{"a sibling whose name this one opens is offered", "Beta", "Writing/Betazh.md"},
		{"a sibling opening with this name is offered", "Betazh", "Writing/Beta.md"},
		{"two notes in one language are not a translation", "Gamma", ""},
		{"two possible translations offer neither", "Delta", ""},
		{"two declared sources offer neither", "Eps", ""},
		{"one declaration decides among three candidates", "Zeta", "Writing/Zetaja.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, body := get(t, srv.Client(), srv.URL+"/notes/Writing/"+tt.note+".md")
			if code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", tt.note, code)
			}
			if got := offeredPartner(t, body); got != tt.partner {
				t.Errorf("%s is offered %q beside it, want %q", tt.note, got, tt.partner)
			}
		})
	}
}
