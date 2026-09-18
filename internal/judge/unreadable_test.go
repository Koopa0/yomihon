package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The fixture that carries a file nothing can be read from, and the file
// itself. The vault trips every rule that answers from the whole vault, so a
// run over it with the file sealed can show each of them going quiet; the
// sealed note is cited by nothing and cites nothing, so removing it from what
// was read changes no other verdict, and a rule that disappears disappeared for
// the reason under test.
//
// The permissions are taken at test time rather than recorded in the fixture,
// because a checked-out tree carries none.
const (
	unreadableFixture = "testdata/vault-unreadable"
	unreadableGolden  = "testdata/golden/unreadable.jsonl"
	sealedNote        = "Concepts/golang/Sealed.md"
)

// sealedVault copies the fixture above into a private directory and takes every
// permission from its one sealed note, returning the copy's root. It fails
// rather than skips where the filesystem ignores the request: the case worth
// hearing about is the system that could not hold the fixture, and a skip there
// leaves every claim below untested while reading green.
func sealedVault(tb testing.TB) string {
	tb.Helper()
	root := judgeFixtureRoot(tb, unreadableFixture)
	if !unreadable(tb, filepath.Join(root, filepath.FromSlash(sealedNote))) {
		tb.Fatal("this process can still read a file it took every permission from, so no vault here has a hole in it to judge")
	}
	return root
}

// ruleSet reduces findings to the rules they carry, optionally ignoring the
// ones a single path owns.
func ruleSet(findings []Finding, exceptPath string) map[RuleID]bool {
	out := make(map[RuleID]bool, len(findings))
	for i := range findings {
		if findings[i].Path == exceptPath {
			continue
		}
		out[findings[i].RuleID] = true
	}
	return out
}

// TestTheWholeVaultRulesSayNothingOverAPartialCorpus is the lock on the
// judgement's honesty about its own hole. A rule that concludes something is
// nowhere in the vault cannot be reached once part of the vault was never read,
// so those rules say nothing at all for that run — while every rule that reads
// one note, or the scan, goes on judging.
//
// The same fixture is judged twice, whole and holed, and the two verdicts are
// compared. That is what makes the list load-bearing in both directions: the
// whole run has to carry every rule the list names, or the fixture is not
// exercising it, and the holed run has to carry none of them. Dropping any rule
// from the list leaves it in the second verdict and fails here.
func TestTheWholeVaultRulesSayNothingOverAPartialCorpus(t *testing.T) {
	t.Parallel()

	whole, err := Check(t.Context(), judgeFixtureRoot(t, unreadableFixture))
	if err != nil {
		t.Fatalf("Check(whole vault) error = %v; the holed run below proves nothing without it", err)
	}
	holed, err := Check(t.Context(), sealedVault(t))
	if err != nil {
		t.Fatalf("Check(vault with a sealed note) error = %v, want a judgement of the rest", err)
	}

	wholeRules, holedRules := ruleSet(whole, ""), ruleSet(holed, "")
	for _, rule := range withheldOnPartialCorpus {
		if !wholeRules[rule] {
			t.Errorf("the fixture never trips %q, so this test cannot tell whether withholding it does anything", rule)
		}
		if holedRules[rule] {
			t.Errorf("%q answered over a vault one file of which was never read, and that file may hold what it says is nowhere", rule)
		}
	}
	if !holedRules[unreadableRule] {
		t.Errorf("the holed run reports no %q, so the rules it silenced are silently missing", unreadableRule)
	}
	if wholeRules[unreadableRule] {
		t.Errorf("the whole run reports %q for a vault it read entirely", unreadableRule)
	}

	// The other half: judging continues. Every rule some other file tripped in
	// the whole run, and that the withheld list does not name, is still here.
	for rule := range ruleSet(whole, sealedNote) {
		if slices.Contains(withheldOnPartialCorpus, rule) || holedRules[rule] {
			continue
		}
		t.Errorf("%q was reported over the whole vault and vanished over the holed one, though it reads one note rather than the vault", rule)
	}
}

// TestTheUnreadableNoticeIsReportedAtTheGravestWeightAndGatesByName holds the
// one frozen line this change moves. A file that could not be read used to end
// the command, which answered 2; it is now a finding, and a finding decides the
// exit code the way every other one does — 0 unless the caller named something
// to fail on. A gate that fired on its own would be a second gating rule living
// beside --deny.
func TestTheUnreadableNoticeIsReportedAtTheGravestWeightAndGatesByName(t *testing.T) {
	t.Parallel()

	root := sealedVault(t)
	tests := []struct {
		name string
		deny []string
		want int
	}{
		{name: "nothing denied", want: 0},
		{name: "the rule denied by name", deny: []string{string(unreadableRule)}, want: 1},
		{name: "every error denied", deny: []string{"error"}, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Deny: tt.deny, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunCheck() error = %v, want a judgement of what it could read", err)
			}
			if exit != tt.want {
				t.Errorf("RunCheck() exit = %d, want %d", exit, tt.want)
			}
			if !bytes.Contains(stdout, []byte(`"rule_id":"`+unreadableRule+`","severity":"error"`)) {
				t.Errorf("RunCheck() output carries no error-weight %q line:\n%s", unreadableRule, stdout)
			}
		})
	}
}

// TestANameFoundExistsEvenWhereAFileCouldNotBeRead draws the line inside the
// exists command. "A note answers to this name" is a statement about a note
// that was read, and a hole elsewhere in the vault leaves it true; only "no note
// does" needs the whole vault, and only that answer is refused. Refusing both
// would take a working answer away from a caller for a fault in another folder.
func TestANameFoundExistsEvenWhereAFileCouldNotBeRead(t *testing.T) {
	t.Parallel()

	root := sealedVault(t)
	stdout, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: "Slice Header", Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunExists(a name that is there) error = %v, want the answer", err)
	}
	if exit != 0 {
		t.Errorf("RunExists(a name that is there) exit = %d, want 0", exit)
	}
	if !bytes.Contains(stdout, []byte("Go Slice.md")) {
		t.Errorf("RunExists(a name that is there) = %s, want the note that answers", stdout)
	}
}

// TestABaselineNeverSilencesTheUnreadableNotice pins the one finding a baseline
// does not subtract. A baseline records what an operator has already seen and
// accepted, so a run reports only what is new. This notice is not a fault in
// the vault to be accepted: it is the run's account of where its own judgement
// has a hole, and the rules it silenced are silent in this output whether or
// not the previous run met the same file. Subtracted, it would leave a report
// short of those rules with nothing saying why.
func TestABaselineNeverSilencesTheUnreadableNotice(t *testing.T) {
	t.Parallel()

	root := sealedVault(t)
	first, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck(first run) error = %v", err)
	}
	baseline := filepath.Join(t.TempDir(), "baseline.jsonl")
	if err := os.WriteFile(baseline, first, 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	second, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Baseline: baseline, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck(against its own output as baseline) error = %v", err)
	}
	if !bytes.Contains(second, []byte(`"rule_id":"`+unreadableRule+`"`)) {
		t.Errorf("a run baselined against its own output dropped the %q notice:\n%s", unreadableRule, second)
	}
	// The control: everything else the same run reported is subtracted, so the
	// survival above is this rule's exemption and not a baseline that failed to
	// load.
	for line := range strings.SplitSeq(strings.TrimSpace(string(second)), "\n") {
		if line != "" && !strings.Contains(line, `"rule_id":"`+string(unreadableRule)+`"`) {
			t.Errorf("a run baselined against its own output still reports: %s", line)
		}
	}
}

// TestTheUnreadableNoticeOutlastsEveryFilter holds the notice past the scope
// cuts. A reader narrowing a run to one folder is saying which ground they want
// judged; they are not saying the hole in the judgement is somebody else's
// business. A filter that kept the missing whole-vault rules and removed the
// sentence explaining them would leave a report that reads complete.
func TestTheUnreadableNoticeOutlastsEveryFilter(t *testing.T) {
	t.Parallel()

	// The sealed note is outside the declared knowledge layer and outside every
	// path filter below, so each cut has something to remove and the notice
	// surviving is the append's position rather than the filter's mercy.
	root := t.TempDir()
	write(t, root, "Notes/ok.md", "---\ntitle: Readable\n---\nA link to [[Ghost]].\n")
	writeTestContract(t, root, nil)
	write(t, root, "Outside/sealed.md", "---\ntitle: Outside\n---\n")
	if !unreadable(t, filepath.Join(root, "Outside", "sealed.md")) {
		t.Fatal("this process can still read a file it took every permission from")
	}

	tests := []struct {
		name  string
		paths []string
		all   bool
	}{
		{name: "the default scope, which the sealed note is outside of"},
		{name: "the whole vault", all: true},
		{name: "one folder, which is not the sealed note's", paths: []string{"Notes"}},
		{name: "one file", paths: []string{"Notes/ok.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout, _, err := RunCheck(t.Context(), &CheckOptions{
				Root: root, Paths: tt.paths, All: tt.all, Format: FormatJSON,
			})
			if err != nil {
				t.Fatalf("RunCheck() error = %v", err)
			}
			if !bytes.Contains(stdout, []byte(`"rule_id":"`+unreadableRule+`"`)) {
				t.Errorf("a narrowed run reports no %q, so its missing rules have no explanation:\n%s", unreadableRule, stdout)
			}
		})
	}
}

// TestANameSharedWithAFileNobodyReadResolvesToNeither pins why a file nothing
// was read from still enters the resolver. Its name is the scan's, and no read
// was needed for it. Left out, a name two files answer to would resolve to
// whichever of them opened, and the fragment rules would go on judging that
// note's headings against an address written for the other one. Inside, the
// name resolves ambiguously, which is the truth about it.
func TestANameSharedWithAFileNobodyReadResolvesToNeither(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, "Notes/Twin.md", "---\ntitle: The one that opens\n---\n## Present\n")
	write(t, root, "Other/Twin.md", "---\ntitle: The one that does not\n---\n")
	write(t, root, "Notes/Cite.md", "---\ntitle: Cite\n---\nA link into [[Twin#Absent]].\n")
	writeTestContract(t, root, nil)
	if !unreadable(t, filepath.Join(root, "Other", "Twin.md")) {
		t.Fatal("this process can still read a file it took every permission from")
	}

	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	for i := range findings {
		if findings[i].RuleID == "link.section_missing" {
			t.Errorf("a section was judged absent from a note the run chose between two files for, one of which it never read: %+v", findings[i])
		}
	}
	if rules := ruleSet(findings, ""); !rules["collision.name"] {
		t.Errorf("two files answer to one name and no collision was reported; rules = %v", rules)
	}
}

// TestAFragmentPointedAtANoteNobodyReadIsNotJudged is the per-link half of the
// same honesty, at the three places a rule concludes something from a target
// note's body. A section or block absent from a note nothing was read from is
// not absent; it is unknown, and this one address says nothing while every other
// link in the run is still judged.
//
// That is narrower than withholding a rule, and it can be: a withheld rule
// concludes that something is nowhere in the vault, which no single link can be
// asked about, whereas here one address has one place left to look and that
// place could not be opened.
func TestAFragmentPointedAtANoteNobodyReadIsNotJudged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "a section in it", body: "A link into [[Sealed#Absent]].\n"},
		{name: "a block in it", body: "A link into [[Sealed#^absent]].\n"},
		{name: "a section it excerpts", body: "An excerpt of [[Sealed#Absent]].\n"},
		{name: "a section reached through a note it excerpts", body: "A link into [[Relay#Absent]].\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			write(t, root, "Notes/Sealed.md", "---\ntitle: Sealed\n---\n## Absent\n")
			write(t, root, "Notes/Relay.md", "---\ntitle: Relay\n---\n![[Sealed]]\n")
			write(t, root, "Notes/Cite.md", "---\ntitle: Cite\n---\n"+tt.body)
			writeTestContract(t, root, nil)
			if !unreadable(t, filepath.Join(root, "Notes", "Sealed.md")) {
				t.Fatal("this process can still read a file it took every permission from")
			}

			findings, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			for i := range findings {
				switch findings[i].RuleID {
				case "link.section_missing", "link.block_missing", "embed.section_missing", "embed.block_missing":
					t.Errorf("a fragment was judged against a note nothing was read from: %+v", findings[i])
				}
			}
		})
	}
}

// TestAWithheldFileThatCouldNotBeReadIsNamedByNothing carries the contract's
// privacy cut into the new finding. A file under a directory the contract keeps
// out of agent-facing output cannot be named, and the reason the machine gave
// for it describes the same closed ground — so every such file collapses into
// one fixed sentence saying a hole exists without saying where. A run missing
// the whole-vault rules has to say so even when it may not say about what.
func TestAWithheldFileThatCouldNotBeReadIsNamedByNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, "Notes/ok.md", "---\ntitle: Readable\n---\nA link to [[Ghost]].\n")
	write(t, root, "Diary/2026-08-27.md", "---\ntitle: Private\n---\n")
	writeTestContract(t, root, []string{"Diary"})
	if !unreadable(t, filepath.Join(root, "Diary", "2026-08-27.md")) {
		t.Fatal("this process can still read a file it took every permission from")
	}

	stdout, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck() error = %v, want a judgement of what it could read", err)
	}
	if !bytes.Contains(stdout, []byte(`"rule_id":"`+unreadableRule+`","severity":"error","path":""`)) {
		t.Errorf("RunCheck() carries no path-less %q line:\n%s", unreadableRule, stdout)
	}
	for _, leaked := range []string{"Diary", "2026-08-27", "permission denied"} {
		if bytes.Contains(stdout, []byte(leaked)) {
			t.Errorf("RunCheck() describes withheld ground with %q:\n%s", leaked, stdout)
		}
	}
	// The whole-vault rules go quiet here too, so the sentence is the only
	// account the reader gets of why the link to nothing went unreported.
	if bytes.Contains(stdout, []byte(`"rule_id":"link.broken"`)) {
		t.Errorf("link.broken answered over a vault one file of which was never read:\n%s", stdout)
	}
}

// TestAVaultNothingCouldBeReadFromIsRefusedRatherThanReported holds the floor
// under "the run continues". A judgement is of something, and where every read
// failed there is nothing in hand to judge: what the caller would get is a page
// whose every line says the same thing, under an exit code that reads as
// success. The refusal is the answer that folder has earned.
func TestAVaultNothingCouldBeReadFromIsRefusedRatherThanReported(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestContract(t, root, nil)
	write(t, root, "Notes/One.md", "---\ntitle: One\n---\n")
	write(t, root, "Notes/Two.md", "---\ntitle: Two\n---\n")
	for _, name := range []string{"One.md", "Two.md"} {
		if !unreadable(t, filepath.Join(root, "Notes", name)) {
			t.Fatal("this process can still read a file it took every permission from")
		}
	}

	got := refuse(t.Context(), t, "check", root).Error()
	if !strings.HasPrefix(got, "vault scan failed: Notes/One.md: ") {
		t.Errorf("check error = %q, want the refusal naming the first file the reads stopped on", got)
	}
}

// TestEveryWithheldFileThatCouldNotBeReadIsOneSentence is the arithmetic the
// fixed sentence exists to withhold. How many files are in a closed folder, and
// how many of them have a permission wrong, are both description of it — so one
// sentence stands for all of them however many there are, and two runs over
// folders holding different numbers of them read the same.
func TestEveryWithheldFileThatCouldNotBeReadIsOneSentence(t *testing.T) {
	t.Parallel()

	sealed := func(t *testing.T, count int) []byte {
		t.Helper()
		root := t.TempDir()
		write(t, root, "Notes/ok.md", "---\ntitle: Readable\n---\n")
		for i := range count {
			write(t, root, "Diary/2026-08-0"+string(rune('1'+i))+".md", "---\ntitle: Private\n---\n")
		}
		writeTestContract(t, root, []string{"Diary"})
		for i := range count {
			name := "Diary/2026-08-0" + string(rune('1'+i)) + ".md"
			if !unreadable(t, filepath.Join(root, filepath.FromSlash(name))) {
				t.Fatal("this process can still read a file it took every permission from")
			}
		}
		stdout, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunCheck(%d withheld files) error = %v", count, err)
		}
		return stdout
	}

	one, three := sealed(t, 1), sealed(t, 3)
	if !bytes.Equal(one, three) {
		t.Errorf("a folder with three unreadable withheld files reads differently from one with a single file, which counts them:\none:\n%s\nthree:\n%s", one, three)
	}
	if got := bytes.Count(one, []byte(`"rule_id":"`+unreadableRule+`"`)); got != 1 {
		t.Errorf("withheld notice lines = %d, want exactly 1", got)
	}
}

// TestTheUnreadableFingerprintCarriesThePathAndNotTheCause pins what a baseline
// keys on. A file that cannot be opened is one finding whichever way the
// operating system words the reason, and a release that reworded it would
// otherwise move every consumer's baseline on a vault nothing had changed in.
func TestTheUnreadableFingerprintCarriesThePathAndNotTheCause(t *testing.T) {
	t.Parallel()

	same := unreadableFinding("Notes/bad.md", "openat bad.md: permission denied")
	reworded := unreadableFinding("Notes/bad.md", "openat bad.md: operation not permitted")
	elsewhere := unreadableFinding("Notes/other.md", "openat bad.md: permission denied")
	if same.Fingerprint != reworded.Fingerprint {
		t.Errorf("one file reworded by the machine changes fingerprint: %q vs %q", same.Fingerprint, reworded.Fingerprint)
	}
	if same.Fingerprint == elsewhere.Fingerprint {
		t.Errorf("two files share one fingerprint: %q", same.Fingerprint)
	}
	if same.Evidence == reworded.Evidence {
		t.Errorf("the cause is not carried into the evidence a reader is given: %q", same.Evidence)
	}
}

// TestUnreadableGolden pins the bytes a consumer parses for a vault with a hole
// in it: the notice's own line, and the findings the run went on to reach. It
// is its own test rather than a row of the golden table because the fixture is
// sealed at test time, which a checked-out tree cannot carry.
func TestUnreadableGolden(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile(unreadableGolden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got := runCheck(t, sealedVault(t))
	if !bytes.Equal(got, want) {
		t.Errorf("check findings differ from golden %s\ngot:\n%s\nwant:\n%s\ngot hex:\n%s\nwant hex:\n%s",
			unreadableGolden, got, want, hex.Dump(got), hex.Dump(want))
	}
}

// TestAReportLineWithNoPathEndsAtItsSentence covers what the two faces a person
// reads had never been asked to render: a finding with no file behind it. Every
// other finding names one, so both lines appended the path unconditionally, and
// this one ended in an empty bracket or a dangling dash — which reads as a path
// the report lost rather than one it is declining to give.
//
// The wording is left to whoever writes it; what is asserted is that the line
// stops where its sentence does, and that a finding which does have a file
// still shows it.
func TestAReportLineWithNoPathEndsAtItsSentence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, "Notes/ok.md", "---\ntitle: Readable\n---\n")
	write(t, root, "Diary/2026-08-27.md", "---\ntitle: Private\n---\n")
	writeTestContract(t, root, []string{"Diary"})
	if !unreadable(t, filepath.Join(root, "Diary", "2026-08-27.md")) {
		t.Fatal("this process can still read a file it took every permission from")
	}

	tests := []struct {
		name    string
		format  Format
		emptied string
		named   string
	}{
		{name: "human", format: FormatHuman, emptied: "()", named: "(Notes/ok.md)"},
		{name: "markdown", format: FormatMarkdown, emptied: "—", named: "— Notes/ok.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: tt.format})
			if err != nil {
				t.Fatalf("RunCheck() error = %v", err)
			}
			var notice string
			for line := range strings.SplitSeq(string(stdout), "\n") {
				if strings.Contains(line, "withholds from agent-facing output") {
					notice = line
				}
			}
			if notice == "" {
				t.Fatalf("the %s report carries no line for the withheld file:\n%s", tt.name, stdout)
			}
			if strings.HasSuffix(strings.TrimRight(notice, " "), tt.emptied) {
				t.Errorf("the %s line for a finding with no path trails off: %q", tt.name, notice)
			}
			// The control: a finding that does have a file still names it, so
			// the line above is short for the right reason.
			if !strings.Contains(string(stdout), tt.named) {
				t.Errorf("the %s report no longer names the file a finding is about:\n%s", tt.name, stdout)
			}
		})
	}
}
