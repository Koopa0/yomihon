package judge

// One named normalizer for each deliberate departure between this engine and
// the reference binary, applied to the reference output before the byte-diff so
// that what remains must match exactly. The set is closed: after these run, any
// remaining difference is a real divergence to bring to review, not noise.
//
// Each departure maps to exactly one strategy:
//   - avoided by construction (no normalizer, documented at the call site): the
//     stderr prefix is never compared, an existence query is never empty, the
//     contract file is always readable, and every invocation is well-formed;
//   - mechanical: the markdown body comparison skips the tool-identity preamble
//     (in the runner), and coverage folds an empty-named domain group into the
//     "(none)" group here;
//   - manifest-driven: a comment-sealed path reference, a link to a journal
//     note's title, and a journal note counted in coverage or matched by the
//     oracle are dropped using the exact sites the generator recorded, because
//     none of these is visible in the output's structured fields.

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// refFinding reads the few fields the drop decision keys on from one reference
// JSONL line, leaving the rest of the line untouched so a kept line is emitted
// byte-for-byte.
type refFinding struct {
	RuleID           string   `json:"rule_id"`
	Path             string   `json:"path"`
	Target           *string  `json:"target"`
	ResolvedTo       *string  `json:"resolved_to"`
	CollisionMembers []string `json:"collision_members"`
}

// normalizeCheckJSONL drops the reference check lines this engine is expected to
// omit and returns the rest verbatim, in order, in this engine's own JSONL
// framing (one object and a newline each). Three journal channels are visible
// in a finding's own fields — the note it is filed against, a collision member,
// and the note a link resolved to; the title-owner channel and the
// comment-sealed path reference are not, so they are matched against the sites
// the manifest recorded.
func normalizeCheckJSONL(ref []byte, man *genManifest) []byte {
	var out bytes.Buffer
	for _, line := range strings.Split(string(ref), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if dropRefCheckLine(line, man) {
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

// dropRefCheckLine reports whether one reference line names a construct this
// engine drops. A line that does not parse is never dropped, so a malformed
// line surfaces as a loud difference rather than being silently discarded.
func dropRefCheckLine(line string, man *genManifest) bool {
	var f refFinding
	if json.Unmarshal([]byte(line), &f) != nil {
		return false
	}
	if underDiary(f.Path) {
		return true
	}
	if f.ResolvedTo != nil && underDiary(*f.ResolvedTo) {
		return true
	}
	if slices.ContainsFunc(f.CollisionMembers, underDiary) {
		return true
	}
	if f.Target == nil {
		return false
	}
	for _, s := range man.diaryTitleLinks {
		if s.path == f.Path && s.target == *f.Target {
			return true
		}
	}
	if f.RuleID == "link.broken.path" {
		for _, s := range man.commentPathRefs {
			if s.path == f.Path && s.target == *f.Target {
				return true
			}
		}
	}
	return false
}

// normalizeCoverage adjusts a reference coverage report to the shape this engine
// produces: it folds an empty-named domain group into "(none)", removes the
// journal concepts the manifest recorded (each an orphan) from the total and
// its domain row, and drops any list entry naming a journal note.
func normalizeCoverage(c *Coverage, man *genManifest) Coverage {
	rows := make(map[string]DomainCoverage, len(c.Domains))
	for _, d := range c.Domains {
		rows[d.Domain] = d
	}
	for _, dc := range man.diaryConcepts {
		row, ok := rows[dc.domain]
		if !ok {
			continue
		}
		row.Concepts--
		row.Orphan--
		if row.Concepts <= 0 {
			delete(rows, dc.domain)
		} else {
			rows[dc.domain] = row
		}
	}
	if empty, ok := rows[""]; ok {
		delete(rows, "")
		merged := rows["(none)"]
		merged.Domain = "(none)"
		merged.Concepts += empty.Concepts
		merged.Mounted += empty.Mounted
		merged.PendingMount += empty.PendingMount
		merged.Orphan += empty.Orphan
		rows["(none)"] = merged
	}
	domains := make([]DomainCoverage, 0, len(rows))
	for name, row := range rows {
		row.Domain = name
		domains = append(domains, row)
	}
	slices.SortFunc(domains, func(a, b DomainCoverage) int { return strings.Compare(a.Domain, b.Domain) })

	return Coverage{
		TotalConcepts: c.TotalConcepts - len(man.diaryConcepts),
		Domains:       domains,
		PendingMount:  withoutDiary(c.PendingMount),
		Orphans:       withoutDiary(c.Orphans),
		Unrouted:      withoutDiaryUnrouted(c.Unrouted),
	}
}

// withoutDiary drops the journal paths from a coverage path list.
func withoutDiary(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if !underDiary(p) {
			out = append(out, p)
		}
	}
	return out
}

// withoutDiaryUnrouted drops the journal entries from a coverage unrouted list.
func withoutDiaryUnrouted(items []Unrouted) []Unrouted {
	out := make([]Unrouted, 0, len(items))
	for _, u := range items {
		if !underDiary(u.Path) {
			out = append(out, u)
		}
	}
	return out
}

// normalizeExists drops the journal matches from a reference existence report,
// since this engine hides a journal note from the oracle. The returned report
// and the exit code it implies are what this engine must reproduce.
func normalizeExists(r existsReport) existsReport {
	matches := make([]existsMatch, 0, len(r.Matches))
	for _, m := range r.Matches {
		if !underDiary(m.Path) {
			matches = append(matches, m)
		}
	}
	return existsReport{Query: r.Query, Matches: matches}
}

// existsExit is the exit code an existence report implies: 0 when a note is
// found, 1 when none is.
func existsExit(r existsReport) int {
	if r.found() {
		return 0
	}
	return 1
}

// --- unit tests: each proves its normalizer earns its place with an input that
// would differ from this engine's output without it.

func TestDiffFuzzNormalizeCheckDropsDiaryChannels(t *testing.T) {
	t.Parallel()
	man := genManifest{
		diaryTitleLinks: []pathTarget{{path: "Notes/linker.md", target: "Private Title"}},
	}
	public := `{"rule_id":"link.broken","severity":"warn","path":"Notes/keep.md","target":"X","fingerprint":"a"}`
	citing := `{"rule_id":"link.broken","severity":"warn","path":"Diary/2026-01-01.md","target":"Y","fingerprint":"b"}`
	collision := `{"rule_id":"collision.alias","severity":"warn","path":"Notes/keep.md","target":"z","collision_members":["Diary/2026-01-02.md","Notes/keep.md"],"fingerprint":"c"}`
	resolved := `{"rule_id":"map.disk_unlisted","severity":"warn","path":"Writing/L.md","target":"Writing/L.md","resolved_to":"Diary/2026-01-03.md","fingerprint":"d"}`
	titleOwner := `{"rule_id":"link.title_not_alias","severity":"warn","path":"Notes/linker.md","target":"Private Title","fingerprint":"e"}`

	ref := strings.Join([]string{public, citing, collision, resolved, titleOwner}, "\n") + "\n"
	got := normalizeCheckJSONL([]byte(ref), &man)
	want := public + "\n"
	if string(got) != want {
		t.Errorf("normalizeCheckJSONL kept the wrong lines\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDiffFuzzNormalizeCheckDropsCommentPathRef(t *testing.T) {
	t.Parallel()
	man := genManifest{
		commentPathRefs: []pathTarget{{path: "Concepts/golang/N.md", target: "in-comment.md"}},
	}
	sealed := `{"rule_id":"link.broken.path","severity":"warn","path":"Concepts/golang/N.md","target":"in-comment.md","fingerprint":"a"}`
	live := `{"rule_id":"link.broken.path","severity":"warn","path":"Concepts/golang/N.md","target":"out-of-comment.md","fingerprint":"b"}`
	ref := sealed + "\n" + live + "\n"
	got := normalizeCheckJSONL([]byte(ref), &man)
	want := live + "\n"
	if string(got) != want {
		t.Errorf("normalizeCheckJSONL did not drop the comment-sealed ref\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDiffFuzzNormalizeCheckKeepsMalformedLine(t *testing.T) {
	t.Parallel()
	// A line that does not parse must be kept, so a real divergence in an
	// unexpected shape is never silently dropped.
	ref := "not json at all\n"
	got := normalizeCheckJSONL([]byte(ref), &genManifest{})
	if string(got) != ref {
		t.Errorf("normalizeCheckJSONL dropped an unparseable line: got %q", got)
	}
}

func TestDiffFuzzNormalizeCoverageFoldsEmptyDomain(t *testing.T) {
	t.Parallel()
	ref := Coverage{
		TotalConcepts: 6,
		Domains: []DomainCoverage{
			{Domain: "", Concepts: 1, Orphan: 1},
			{Domain: "golang", Concepts: 5, Mounted: 1, PendingMount: 1, Orphan: 3},
		},
		PendingMount: []string{"Concepts/golang/A.md"},
		Orphans:      []string{"Concepts/golang/B.md"},
		Unrouted:     []Unrouted{},
	}
	want := Coverage{
		TotalConcepts: 6,
		Domains: []DomainCoverage{
			{Domain: "(none)", Concepts: 1, Orphan: 1},
			{Domain: "golang", Concepts: 5, Mounted: 1, PendingMount: 1, Orphan: 3},
		},
		PendingMount: []string{"Concepts/golang/A.md"},
		Orphans:      []string{"Concepts/golang/B.md"},
		Unrouted:     []Unrouted{},
	}
	got := normalizeCoverage(&ref, &genManifest{})
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("normalizeCoverage empty-domain fold mismatch (-want +got):\n%s", diff)
	}
}

func TestDiffFuzzNormalizeCoverageDropsDiaryConcept(t *testing.T) {
	t.Parallel()
	man := genManifest{diaryConcepts: []diaryConcept{{domain: "golang"}}}
	ref := Coverage{
		TotalConcepts: 9,
		Domains: []DomainCoverage{
			{Domain: "golang", Concepts: 9, Mounted: 1, PendingMount: 1, Orphan: 7},
		},
		PendingMount: []string{"Concepts/golang/P.md"},
		Orphans:      []string{"Concepts/golang/O.md", "Diary/2026-05-01.md"},
		Unrouted:     []Unrouted{},
	}
	want := Coverage{
		TotalConcepts: 8,
		Domains: []DomainCoverage{
			{Domain: "golang", Concepts: 8, Mounted: 1, PendingMount: 1, Orphan: 6},
		},
		PendingMount: []string{"Concepts/golang/P.md"},
		Orphans:      []string{"Concepts/golang/O.md"},
		Unrouted:     []Unrouted{},
	}
	got := normalizeCoverage(&ref, &man)
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("normalizeCoverage diary-concept drop mismatch (-want +got):\n%s", diff)
	}
}

func TestDiffFuzzNormalizeCoverageDropsSoleDiaryDomainRow(t *testing.T) {
	t.Parallel()
	// A journal concept alone in its domain leaves no row on this engine, so its
	// removal must delete the row, not leave a zero-count one.
	man := genManifest{diaryConcepts: []diaryConcept{{domain: "meta"}}}
	ref := Coverage{
		TotalConcepts: 1,
		Domains:       []DomainCoverage{{Domain: "meta", Concepts: 1, Orphan: 1}},
		Orphans:       []string{"Diary/2026-05-02.md"},
	}
	got := normalizeCoverage(&ref, &man)
	want := Coverage{TotalConcepts: 0}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("normalizeCoverage sole-domain drop mismatch (-want +got):\n%s", diff)
	}
}

func TestDiffFuzzNormalizeExistsDropsDiaryMatch(t *testing.T) {
	t.Parallel()
	ref := existsReport{
		Query: "Private Session Note",
		Matches: []existsMatch{
			{Path: "Diary/2026-07-01.md", Field: "title", Value: "Private Session Note"},
		},
	}
	got := normalizeExists(ref)
	want := existsReport{Query: "Private Session Note", Matches: []existsMatch{}}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("normalizeExists diary drop mismatch (-want +got):\n%s", diff)
	}
	if existsExit(got) != 1 {
		t.Errorf("existsExit after diary drop = %d, want 1 (not found)", existsExit(got))
	}
}

func TestDiffFuzzNormalizeExistsKeepsPublicMatch(t *testing.T) {
	t.Parallel()
	ref := existsReport{
		Query: "Go Slice",
		Matches: []existsMatch{
			{Path: "Concepts/golang/Go Slice.md", Field: "filename", Value: "Go Slice"},
			{Path: "Diary/2026-07-01.md", Field: "alias", Value: "Go Slice"},
		},
	}
	got := normalizeExists(ref)
	want := existsReport{
		Query:   "Go Slice",
		Matches: []existsMatch{{Path: "Concepts/golang/Go Slice.md", Field: "filename", Value: "Go Slice"}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("normalizeExists kept the wrong matches (-want +got):\n%s", diff)
	}
	if existsExit(got) != 0 {
		t.Errorf("existsExit with a public match = %d, want 0 (found)", existsExit(got))
	}
}
