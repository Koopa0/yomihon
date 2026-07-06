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
//     oracle are dropped, a public concept mounted only by a journal map is
//     re-derived from mounted to orphan, and a public broken link whose target
//     is planned only in the journal is flipped from info back to warn — all
//     keyed on the exact sites the generator recorded, because none of these is
//     visible in the output's structured fields.

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// refFinding reads the few fields the drop-or-rewrite decision keys on from one
// reference JSONL line, leaving the rest of the line untouched so a kept line is
// emitted byte-for-byte.
type refFinding struct {
	RuleID           string   `json:"rule_id"`
	Path             string   `json:"path"`
	Line             *int     `json:"line"`
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
		if flipped, ok := flipJournalPlannedLine(line, man); ok {
			out.Write(flipped)
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

// flipJournalPlannedLine rewrites a reference broken-link line to the warning
// this engine emits when the link's target is planned only in the journal. The
// reference counts the journal's planned list and files the link as info; this
// engine excludes the journal source, so the same link is a warning. Only the
// severity and its two advisory strings differ, so the warning is rebuilt from
// the engine's own broken-link finding — keyed on the exact site the manifest
// recorded — and reproduced through the shared encoder, byte-for-byte. A line
// that does not parse, or one the manifest does not name, is left for the caller
// to keep, so an unrelated broken link is never touched. The severity flip does
// not change the sort keys, so the rewritten line sits where the original did.
func flipJournalPlannedLine(line string, man *genManifest) ([]byte, bool) {
	if len(man.diaryPlannedTargets) == 0 {
		return nil, false
	}
	var f refFinding
	if json.Unmarshal([]byte(line), &f) != nil {
		return nil, false
	}
	if f.RuleID != "link.broken" || f.Target == nil || f.Line == nil {
		return nil, false
	}
	for _, s := range man.diaryPlannedTargets {
		if s.path != f.Path || s.target != *f.Target {
			continue
		}
		warn := brokenLink(&note{path: f.Path}, wikiLink{target: *f.Target, line: *f.Line}, nil)
		var buf bytes.Buffer
		if err := WriteJSONL(&buf, []Finding{warn}); err != nil {
			return nil, false
		}
		return buf.Bytes(), true
	}
	return nil, false
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
	// Each manifest-driven drop is constrained to the one rule its construct
	// produces on the reference side, so a different finding that happens to
	// share a path and target is kept and surfaces as a loud difference.
	if f.RuleID == "link.title_not_alias" {
		for _, s := range man.diaryTitleLinks {
			if s.path == f.Path && s.target == *f.Target {
				return true
			}
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
// journal concepts the manifest recorded (each an orphan) from the total and its
// domain row, re-derives a concept mounted only by a journal map from mounted to
// orphan, and drops any list entry naming a journal note.
func normalizeCoverage(c *Coverage, man *genManifest) Coverage {
	rows := make(map[string]DomainCoverage, len(c.Domains))
	for _, d := range c.Domains {
		rows[d.Domain] = d
	}
	// The manifest names a note the reference counts as an orphan in its domain,
	// so a missing row or one with no orphan concept to remove means the manifest
	// and the reference disagree — a harness fault surfaced at its cause rather
	// than left to underflow into a negative count. A domain reduced to no
	// concept drops its row, matching this engine, which files no journal note in
	// coverage at all.
	removed := 0
	for _, dc := range man.diaryConcepts {
		row, ok := rows[dc.domain]
		if !ok || row.Concepts <= 0 || row.Orphan <= 0 {
			panic("difffuzz: journal-concept re-derivation for domain " + dc.domain + " found no orphan concept to remove")
		}
		removed++
		row.Concepts--
		row.Orphan--
		if row.Concepts <= 0 {
			delete(rows, dc.domain)
		} else {
			rows[dc.domain] = row
		}
	}
	// A concept mounted only by a journal map is filed as mounted on the
	// reference and an orphan here, so its domain row moves one from mounted to
	// orphan and its path joins the orphan list. It stays counted, so the total
	// is unchanged. The manifest names a concept the reference must have filed as
	// mounted, so a missing row or one with no mounted concept to move means the
	// manifest and the reference disagree — a harness fault surfaced at its cause
	// rather than buried under a negative count that would read downstream as an
	// engine divergence.
	extraOrphans := make([]string, 0, len(man.diaryMountTargets))
	for _, dm := range man.diaryMountTargets {
		row, ok := rows[dm.domain]
		if !ok || row.Mounted <= 0 {
			panic("difffuzz: journal-mount re-derivation for " + dm.path + " found domain " + dm.domain + " with no mounted concept to move")
		}
		row.Mounted--
		row.Orphan++
		rows[dm.domain] = row
		extraOrphans = append(extraOrphans, dm.path)
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

	orphans := append(withoutDiary(c.Orphans), extraOrphans...)
	slices.Sort(orphans)

	return Coverage{
		TotalConcepts: c.TotalConcepts - removed,
		Domains:       domains,
		PendingMount:  withoutDiary(c.PendingMount),
		Orphans:       orphans,
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

func TestDiffFuzzNormalizeCheckKeepsOtherRuleAtTitleLinkSite(t *testing.T) {
	t.Parallel()
	// A finding that shares a title-link site's path and target but carries a
	// different rule must be kept: the drop is scoped to the one rule the
	// construct produces, so it can never mask an unrelated divergence.
	man := genManifest{
		diaryTitleLinks: []pathTarget{{path: "Notes/linker.md", target: "Private Title"}},
	}
	titleOwner := `{"rule_id":"link.title_not_alias","severity":"warn","path":"Notes/linker.md","target":"Private Title","fingerprint":"a"}`
	otherRule := `{"rule_id":"provenance.unresolved","severity":"warn","path":"Notes/linker.md","target":"Private Title","fingerprint":"b"}`
	ref := titleOwner + "\n" + otherRule + "\n"
	got := normalizeCheckJSONL([]byte(ref), &man)
	want := otherRule + "\n"
	if string(got) != want {
		t.Errorf("normalizeCheckJSONL rule scoping wrong\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDiffFuzzNormalizeCoverageFailsLoudOnConceptMismatch(t *testing.T) {
	t.Parallel()
	// A journal-concept entry names a note the reference counts as an orphan in
	// its domain, so a domain the reference does not orphan is a manifest /
	// reference disagreement: a missing row, or a row with no orphan concept to
	// remove. Both are surfaced at their cause rather than left to underflow into
	// a negative count that reads downstream as an engine divergence.
	tests := []struct {
		name          string
		conceptDomain string
		domains       []DomainCoverage
	}{
		{name: "no row for the domain", conceptDomain: "ghost", domains: []DomainCoverage{{Domain: "golang", Concepts: 2, Orphan: 2}}},
		{name: "row with no orphan to remove", conceptDomain: "golang", domains: []DomainCoverage{{Domain: "golang", Concepts: 2, Mounted: 2, Orphan: 0}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			man := genManifest{diaryConcepts: []diaryConcept{{domain: tt.conceptDomain}}}
			ref := Coverage{TotalConcepts: 2, Domains: tt.domains, Unrouted: []Unrouted{}}
			defer func() {
				if recover() == nil {
					t.Error("normalizeCoverage did not fail loud on a journal-concept the reference does not orphan")
				}
			}()
			normalizeCoverage(&ref, &man)
		})
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

func TestDiffFuzzNormalizeCheckFlipsJournalPlanned(t *testing.T) {
	t.Parallel()
	// A broken link whose target the manifest records as planned only in the
	// journal is flipped from the reference's info form back to the warning this
	// engine emits: the same rule at the same site, with the severity and the two
	// advisory strings rewritten. The expected line is hand-written, differing
	// from the input in exactly those three fields.
	man := genManifest{
		diaryPlannedTargets: []pathTarget{{path: "Concepts/golang/Journal Planned Link.md", target: "Planned Only In Journal"}},
	}
	info := `{"rule_id":"link.broken","severity":"info","path":"Concepts/golang/Journal Planned Link.md","line":6,"message":"[[Planned Only In Journal]] resolves to no note","evidence":"a tracked forward-reference (under a gap heading or listed as a planned concept)","suggested_action":"if it is written, check the filename/alias matches; otherwise leave it tracked","source_rule":"Note-Schema.md#aliases","target":"Planned Only In Journal","fingerprint":"c1172f70ef3641b2"}`
	want := `{"rule_id":"link.broken","severity":"warn","path":"Concepts/golang/Journal Planned Link.md","line":6,"message":"[[Planned Only In Journal]] resolves to no note","evidence":"no filename or alias matches the target","suggested_action":"create the target note, or change the link to an existing filename/alias","source_rule":"Note-Schema.md#aliases","target":"Planned Only In Journal","fingerprint":"c1172f70ef3641b2"}` + "\n"
	got := normalizeCheckJSONL([]byte(info+"\n"), &man)
	if string(got) != want {
		t.Errorf("normalizeCheckJSONL did not flip the journal-planned link to warn\ngot:\n%swant:\n%s", got, want)
	}
}

func TestDiffFuzzNormalizeCheckKeepsBrokenLinkOffJournalPlanned(t *testing.T) {
	t.Parallel()
	// A broken link the manifest does not name as journal-planned is emitted
	// verbatim, whatever its severity, so the flip can never touch an unrelated
	// link. A public planned link the reference files as info must stay info.
	man := genManifest{
		diaryPlannedTargets: []pathTarget{{path: "Concepts/golang/Journal Planned Link.md", target: "Planned Only In Journal"}},
	}
	elsewhere := `{"rule_id":"link.broken","severity":"info","path":"Concepts/golang/Public Planned Link.md","line":6,"message":"[[Planned In Public]] resolves to no note","evidence":"a tracked forward-reference (under a gap heading or listed as a planned concept)","suggested_action":"if it is written, check the filename/alias matches; otherwise leave it tracked","source_rule":"Note-Schema.md#aliases","target":"Planned In Public","fingerprint":"7593250b44cf5d46"}`
	got := normalizeCheckJSONL([]byte(elsewhere+"\n"), &man)
	want := elsewhere + "\n"
	if string(got) != want {
		t.Errorf("normalizeCheckJSONL changed a link off the journal-planned site\ngot:\n%swant:\n%s", got, want)
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

func TestDiffFuzzNormalizeCoverageReDerivesJournalMount(t *testing.T) {
	t.Parallel()
	// A concept mounted only by a journal map is mounted on the reference and an
	// orphan here, so its domain row moves one from mounted to orphan and its
	// path joins the orphan list, in sorted order. It stays counted, so the total
	// is unchanged.
	man := genManifest{diaryMountTargets: []diaryMount{{path: "Concepts/golang/PubMount.md", domain: "golang"}}}
	ref := Coverage{
		TotalConcepts: 3,
		Domains:       []DomainCoverage{{Domain: "golang", Concepts: 3, Mounted: 2, Orphan: 1}},
		Orphans:       []string{"Concepts/golang/Other Orphan.md"},
		Unrouted:      []Unrouted{},
	}
	want := Coverage{
		TotalConcepts: 3,
		Domains:       []DomainCoverage{{Domain: "golang", Concepts: 3, Mounted: 1, Orphan: 2}},
		Orphans:       []string{"Concepts/golang/Other Orphan.md", "Concepts/golang/PubMount.md"},
		Unrouted:      []Unrouted{},
	}
	got := normalizeCoverage(&ref, &man)
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("normalizeCoverage journal-mount re-derivation mismatch (-want +got):\n%s", diff)
	}
}

func TestDiffFuzzNormalizeCoverageFailsLoudOnMountMismatch(t *testing.T) {
	t.Parallel()
	// A journal-mount entry names a concept the reference must have filed as
	// mounted, so a domain the reference does not mount is a manifest / reference
	// disagreement: either a duplicate entry that has already spent the one
	// mounted concept, or reference output that never mounted it. Both are
	// surfaced at their cause rather than left to underflow into a negative count
	// that reads downstream as an engine divergence.
	tests := []struct {
		name        string
		mountDomain string
		domains     []DomainCoverage
	}{
		{name: "no row for the domain", mountDomain: "ghost", domains: []DomainCoverage{{Domain: "golang", Concepts: 2, Mounted: 1, Orphan: 1}}},
		{name: "row with nothing mounted", mountDomain: "golang", domains: []DomainCoverage{{Domain: "golang", Concepts: 1, Mounted: 0, Orphan: 1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			man := genManifest{diaryMountTargets: []diaryMount{{path: "Concepts/" + tt.mountDomain + "/PubMount.md", domain: tt.mountDomain}}}
			ref := Coverage{TotalConcepts: 2, Domains: tt.domains, Unrouted: []Unrouted{}}
			defer func() {
				if recover() == nil {
					t.Error("normalizeCoverage did not fail loud on a journal-mount the reference does not mount")
				}
			}()
			normalizeCoverage(&ref, &man)
		})
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
