package layouts

import (
	"bytes"
	"encoding/json"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// speculationCondition is one node of a rule's "where" clause, read the way
// the browser reads it. Exactly one member is set on a node that means
// something; the test refuses a node that sets none or several, because the
// browser would refuse it too and the rules would then do nothing at all.
type speculationCondition struct {
	And             []speculationCondition `json:"and"`
	Or              []speculationCondition `json:"or"`
	Not             *speculationCondition  `json:"not"`
	HrefMatches     json.RawMessage        `json:"href_matches"`
	SelectorMatches string                 `json:"selector_matches"`
}

type speculationRule struct {
	Where     speculationCondition `json:"where"`
	Eagerness string               `json:"eagerness"`
}

// admits answers whether the condition lets a link to address be fetched.
// stepLink says whether the link is one the condition's selector clause picks
// out: a selector cannot be evaluated without a document, so the caller states
// it, and the clause is then exactly as true as the caller says.
//
// href_matches follows URL pattern semantics for the two forms the rules use.
// A string names a path, and the query is free. An object names path and query
// and has to give both: an object that names only a query inherits the
// document's own path, which would admit nothing.
func (c *speculationCondition) admits(t *testing.T, address string, stepLink bool) bool {
	t.Helper()
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", address, err)
	}
	set := 0
	for _, present := range []bool{c.And != nil, c.Or != nil, c.Not != nil, c.HrefMatches != nil, c.SelectorMatches != ""} {
		if present {
			set++
		}
	}
	if set != 1 {
		t.Fatalf("condition sets %d members, want exactly 1: %+v", set, c)
	}
	switch {
	case c.And != nil:
		for i := range c.And {
			if !c.And[i].admits(t, address, stepLink) {
				return false
			}
		}
		return true
	case c.Or != nil:
		for i := range c.Or {
			if c.Or[i].admits(t, address, stepLink) {
				return true
			}
		}
		return false
	case c.Not != nil:
		return !c.Not.admits(t, address, stepLink)
	case c.SelectorMatches != "":
		return stepLink
	}
	pathPattern, queryPattern := hrefPatterns(t, c.HrefMatches)
	return patternMatches(pathPattern, parsed.EscapedPath()) && patternMatches(queryPattern, parsed.RawQuery)
}

// hrefPatterns reads one href_matches value into the patterns for the path and
// for the query of an address.
func hrefPatterns(t *testing.T, raw json.RawMessage) (pathPattern, queryPattern string) {
	t.Helper()
	if err := json.Unmarshal(raw, &pathPattern); err == nil {
		return pathPattern, "*"
	}
	var pattern struct {
		Pathname string `json:"pathname"`
		Search   string `json:"search"`
	}
	if err := json.Unmarshal(raw, &pattern); err != nil {
		t.Fatalf("href_matches %s is neither a string nor a pattern object: %v", raw, err)
	}
	if pattern.Pathname == "" {
		t.Fatalf("href_matches %s names no pathname, so it would inherit the document's own", raw)
	}
	if pattern.Search == "" {
		t.Fatalf("href_matches %s names no search; an empty one would mean no query at all", raw)
	}
	return pattern.Pathname, pattern.Search
}

// patternMatches is a URL pattern component with only the wildcard: "*"
// matches anything, every other character matches itself.
func patternMatches(pattern, text string) bool {
	expression := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$"
	return regexp.MustCompile(expression).MatchString(text)
}

// renderedSpeculationRules returns the JSON body of the speculation rules
// element Base writes, and fails unless there is exactly one, carrying the
// response's nonce.
func renderedSpeculationRules(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := Base(Chrome{Title: "測試", Nonce: "response-nonce"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render base: %v", err)
	}
	found := regexp.MustCompile(`<script type="speculationrules" nonce="response-nonce">(.*?)</script>`).FindAllSubmatch(buf.Bytes(), -1)
	if len(found) != 1 {
		t.Fatalf("Base() speculation rules elements with the response nonce = %d, want 1; html = %q", len(found), buf.String())
	}
	return found[0][1]
}

// TestSpeculationRulesPrefetchOnly pins what the rules ask the browser to do:
// prefetch, in the two eagernesses the page wants, and nothing else. Prerender
// would run a page's scripts before the reader is there.
func TestSpeculationRulesPrefetchOnly(t *testing.T) {
	t.Parallel()
	body := renderedSpeculationRules(t)
	var actions map[string]json.RawMessage
	if err := json.Unmarshal(body, &actions); err != nil {
		t.Fatalf("speculation rules are not JSON: %v; body = %s", err, body)
	}
	if got := mapKeys(actions); !reflect.DeepEqual(got, []string{"prefetch"}) {
		t.Errorf("speculation rules ask for %v, want only [prefetch]; body = %s", got, body)
	}
	if bytes.Contains(body, []byte("prerender")) {
		t.Errorf("speculation rules mention prerender; body = %s", body)
	}
	var rules []speculationRule
	if err := json.Unmarshal(actions["prefetch"], &rules); err != nil {
		t.Fatalf("prefetch rules do not decode: %v", err)
	}
	var eagerness []string
	for _, rule := range rules {
		eagerness = append(eagerness, rule.Eagerness)
	}
	if want := []string{"moderate", "immediate"}; !reflect.DeepEqual(eagerness, want) {
		t.Errorf("prefetch rule eagerness = %v, want %v", eagerness, want)
	}
}

// TestSpeculationRulesReachOnlyReadingAddresses holds both rules to the same
// allowlist, so the rule that fetches on page load cannot fetch what the one
// that waits for a hover may not. Each address is judged by the condition as
// the browser reads it.
func TestSpeculationRulesReachOnlyReadingAddresses(t *testing.T) {
	t.Parallel()
	var rules struct {
		Prefetch []speculationRule `json:"prefetch"`
	}
	if err := json.Unmarshal(renderedSpeculationRules(t), &rules); err != nil {
		t.Fatalf("speculation rules do not decode: %v", err)
	}
	if len(rules.Prefetch) != 2 {
		t.Fatalf("prefetch rules = %d, want 2", len(rules.Prefetch))
	}
	hover, load := &rules.Prefetch[0], &rules.Prefetch[1]

	// Reads that a reader opens, query and fragment included.
	fetched := []string{
		"/notes/Lessons/L01%20Point%20yomihon%20at%20a%20folder.md",
		"/notes/Notes/What%20yomihon%20is.md?q=read#a-section",
		"/syllabus/Maps/study.md",
		"/paths",
		"/maps",
		"/journal",
		"/journal?month=2026-07",
		"/folders",
		"/folders/Concepts",
		"/reports",
		"/reports/2026-09-03.html",
	}
	// Everything else a reader's page links to or the server answers, named so
	// that a route added under an allowed prefix has to be added here to be
	// judged. The last three are the allowed prefixes' own traps.
	left := []string{
		"/",
		"/health",
		"/search",
		"/search?q=read",
		"/search/results?q=read",
		"/listen/Maps/study.md",
		"/compare/Concepts/alpha.md?with=Concepts/beta.md",
		"/raw/Notes/What%20yomihon%20is.md",
		"/freshness/Notes/What%20yomihon%20is.md",
		"/preview/Notes/What%20yomihon%20is.md",
		"/thought/Notes/What%20yomihon%20is.md",
		"/open-thoughts",
		"/preferences",
		"/preferences?from=%2Fnotes%2Fa.md",
		"/marks",
		"/uncertainties",
		"/static/app.css",
		"/lang",
		"/status",
		"/reports/2026-09-03.html/raw",
		"/notes/Notes/What%20yomihon%20is.md?from=draft",
		"/notes/Notes/What%20yomihon%20is.md?q=read&from=draft",
	}
	for _, address := range fetched {
		if !hover.Where.admits(t, address, false) {
			t.Errorf("hover rule leaves %q alone, want it fetched", address)
		}
		if !load.Where.admits(t, address, true) {
			t.Errorf("load rule leaves the step link to %q alone, want it fetched", address)
		}
		if load.Where.admits(t, address, false) {
			t.Errorf("load rule fetches %q although the link is not a step link", address)
		}
	}
	for _, address := range left {
		if hover.Where.admits(t, address, false) {
			t.Errorf("hover rule fetches %q, want it left alone", address)
		}
		if load.Where.admits(t, address, true) {
			t.Errorf("load rule fetches a step link to %q, want it left alone", address)
		}
	}

	// The two rules carry one allowlist: the load rule is the hover rule's
	// whole condition with the step selector added.
	if len(hover.Where.And) == 0 || len(load.Where.And) != len(hover.Where.And)+1 {
		t.Fatalf("hover condition has %d clauses and load has %d, want load to add exactly one", len(hover.Where.And), len(load.Where.And))
	}
	if !reflect.DeepEqual(load.Where.And[:len(hover.Where.And)], hover.Where.And) {
		t.Errorf("the load rule's allowlist differs from the hover rule's:\nhover %+v\nload  %+v", hover.Where.And, load.Where.And)
	}
	if got := load.Where.And[len(load.Where.And)-1].SelectorMatches; got != "a[rel~=prev], a[rel~=next]" {
		t.Errorf("load rule selects %q, want the previous and next links", got)
	}
}

// TestSpeculationScriptEscapesTheNonce keeps the nonce an attribute value
// however it is spelled, because the element is written by hand rather than by
// the template.
func TestSpeculationScriptEscapesTheNonce(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := speculationScript(`a"><script>alert(1)</script>`).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := strings.Count(buf.String(), "<script"); got != 1 {
		t.Errorf("a hostile nonce produced %d script elements, want 1; html = %q", got, buf.String())
	}
}

func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
