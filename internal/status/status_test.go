package status_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
)

// testRel is the vault-relative path every fixture note in this package
// uses. Fixing it keeps the git assertions (commit message, dirty-file
// targeting) simple; the path itself is never what's under test.
const testRel = "Writing/lessons/japanese/L05.md"

// The testdata contract is a loader fixture, not a second schema: runtime
// code only ever reads the real vault contract. schema.LoadFile is
// reused as-is — there is no second schema-loading path in this package.
func loadContract(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile(testdata/contract.toml) = %v", err)
	}
	return s
}

// newVault creates a temp git repo standing in for the vault, with a fake
// local git identity scoped to that repo only. The flip's commit author
// must come from this config — never anything yomihon hardcodes —
// so tests exercise the real git behavior instead of mocking it.
// commit.gpgsign is forced off so the test does not depend on whatever the
// host's global git config or GPG agent happens to be set up to do.
func newVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "Test Vault")
	runGit(t, root, "config", "user.email", "test-vault@example.invalid")
	runGit(t, root, "config", "commit.gpgsign", "false")
	return root
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(t.Context(), "git", fullArgs...) // #nosec G204 -- fixed test-controlled git invocation; args never shell-interpreted
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeNote(t *testing.T, root, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(testRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readNote(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(testRel))) // #nosec G304 -- testRel is a fixed in-package constant, not external input
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func commitAll(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "seed lesson")
}

func commitCount(t *testing.T, root string) int {
	t.Helper()
	out := strings.TrimSpace(runGit(t, root, "log", "--oneline"))
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

// lessonContent is a minimal, legal lesson note with a single status line.
func lessonContent(noteStatus string) string {
	return "---\n" +
		"title: L05\n" +
		"type: lesson\n" +
		"domain: japanese\n" +
		"status: " + noteStatus + "\n" +
		"created: 2026-06-01\n" +
		"updated: 2026-06-01\n" +
		"---\n" +
		"\nbody\n"
}

func TestFlipHappyPath(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, loadContract(t))

	original := "---\n" +
		"title: L05 助詞の使い方\n" +
		"type: lesson\n" +
		"domain: japanese\n" +
		"status: draft\n" +
		"based_on: \"[[大家的日本語 第5課]]\"\n" +
		"# hand-written note, must survive verbatim\n" +
		"created: 2026-06-01\n" +
		"updated: 2026-06-15\n" +
		"---\n" +
		"\n" +
		"<ruby>今日<rt>きょう</rt></ruby>は<ruby>晴<rt>は</rt></ruby>れ。\n"
	writeNote(t, root, original)
	commitAll(t, root)
	before := commitCount(t, root)

	if err := svc.Flip(t.Context(), testRel, "draft", "ready"); err != nil {
		t.Fatalf("Flip() = %v, want nil", err)
	}

	// The single highest-stakes assertion in this feature: everything but
	// the status line's content must be byte-identical.
	want := strings.Replace(original, "status: draft", "status: ready", 1)
	got := readNote(t, root)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("file mismatch after flip (-want +got):\n%s", diff)
	}

	if after := commitCount(t, root); after != before+1 {
		t.Fatalf("commit count = %d, want %d (exactly one new commit)", after, before+1)
	}

	wantSubject := "status(" + testRel + "): draft → ready (via yomihon)"
	if got := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); got != wantSubject {
		t.Errorf("commit subject = %q, want %q", got, wantSubject)
	}

	wantName := strings.TrimSpace(runGit(t, root, "config", "user.name"))
	wantEmail := strings.TrimSpace(runGit(t, root, "config", "user.email"))
	gotName := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%an"))
	gotEmail := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%ae"))
	if gotName != wantName || gotEmail != wantEmail {
		t.Errorf("commit author = %q <%s>, want the vault's own git config %q <%s>", gotName, gotEmail, wantName, wantEmail)
	}

	if porcelain := strings.TrimSpace(runGit(t, root, "status", "--porcelain")); porcelain != "" {
		t.Errorf("git status --porcelain not empty after flip: %q", porcelain)
	}
}

func TestFlipRefusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		onDiskStatus string // status committed into the note before flipping
		from, to     string
		dirtyEdit    bool
		wantErr      error
	}{
		{
			name:         "stale form: from does not match actual current status",
			onDiskStatus: "draft",
			from:         "imported",
			to:           "ready",
			wantErr:      status.ErrStale,
		},
		{
			name:         "illegal transition: skips a required intermediate status",
			onDiskStatus: "imported",
			from:         "imported",
			to:           "ready",
			wantErr:      schema.ErrIllegalTransition,
		},
		{
			name:         "dirty file: unrelated uncommitted edit already present",
			onDiskStatus: "draft",
			from:         "draft",
			to:           "ready",
			dirtyEdit:    true,
			wantErr:      status.ErrDirty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := newVault(t)
			svc := status.NewService(root, loadContract(t))

			committed := lessonContent(tt.onDiskStatus)
			writeNote(t, root, committed)
			commitAll(t, root)
			before := commitCount(t, root)

			onDisk := committed
			if tt.dirtyEdit {
				onDisk = committed + "<!-- an unrelated, uncommitted edit -->\n"
				writeNote(t, root, onDisk)
			}

			err := svc.Flip(t.Context(), testRel, tt.from, tt.to)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Flip() = %v, want %v", err, tt.wantErr)
			}
			if got := readNote(t, root); got != onDisk {
				t.Errorf("file was touched:\ngot:  %q\nwant: %q", got, onDisk)
			}
			if after := commitCount(t, root); after != before {
				t.Errorf("commit count = %d, want unchanged %d (no commit created)", after, before)
			}
		})
	}
}

func TestFlipMalformedStatusLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "zero status lines",
			content: "---\n" +
				"title: L05\n" +
				"type: lesson\n" +
				"created: 2026-06-01\n" +
				"---\n" +
				"\nbody\n",
		},
		{
			name: "two status lines",
			content: "---\n" +
				"title: L05\n" +
				"type: lesson\n" +
				"status: draft\n" +
				"status: archived\n" +
				"---\n" +
				"\nbody\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := newVault(t)
			svc := status.NewService(root, loadContract(t))

			writeNote(t, root, tt.content)
			commitAll(t, root)
			before := commitCount(t, root)

			// Neither fixture parses a legal current status (nil
			// Frontmatter or a YAML error both yield Status() == "").
			// "archived" is the one contract entry legal from any status
			// for koopa (applies_to and from are both "*"), so from=""
			// reaches the surgical-rewrite step regardless.
			err := svc.Flip(t.Context(), testRel, "", "archived")
			if !errors.Is(err, status.ErrStatusLine) {
				t.Fatalf("Flip() = %v, want %v", err, status.ErrStatusLine)
			}
			if got := readNote(t, root); got != tt.content {
				t.Errorf("file was touched:\ngot:  %q\nwant: %q", got, tt.content)
			}
			if after := commitCount(t, root); after != before {
				t.Errorf("commit count = %d, want unchanged %d (no commit created)", after, before)
			}
		})
	}
}

func TestFlipFailClosed(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, nil)

	if !svc.Closed() {
		t.Fatal("Closed() = false, want true for a nil contract")
	}
	if got := svc.Transitions("lesson", "draft"); got != nil {
		t.Errorf("Transitions() = %v, want nil", got)
	}

	tests := []struct {
		name          string
		rel, from, to string
	}{
		{"well-formed args", "Writing/lessons/japanese/L05.md", "draft", "ready"},
		{"garbage args", "does/not/exist.md", "", ""},
		{"path escape attempt", "../outside.md", "draft", "ready"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := svc.Flip(t.Context(), tt.rel, tt.from, tt.to); !errors.Is(err, status.ErrClosed) {
				t.Errorf("Flip(%q, %q, %q) = %v, want %v", tt.rel, tt.from, tt.to, err, status.ErrClosed)
			}
		})
	}
}

func TestClosed(t *testing.T) {
	t.Parallel()
	if !status.NewService(t.TempDir(), nil).Closed() {
		t.Error("Closed() = false, want true for nil contract")
	}
	if status.NewService(t.TempDir(), loadContract(t)).Closed() {
		t.Error("Closed() = true, want false for a loaded contract")
	}
}

func TestTransitions(t *testing.T) {
	t.Parallel()
	svc := status.NewService(t.TempDir(), loadContract(t))

	tests := []struct {
		name     string
		noteType string
		current  string
		want     []string
	}{
		// Cross-checked by hand against testdata/contract.toml's lifecycle
		// table for actor "koopa":
		//   imported: from=[] owner=[claude,koopa]
		//   draft:    from=[imported] owner=[claude,koopa]
		//   ready:    from=[draft] owner=[koopa]
		//   archived: from=[*] applies_to=[*] owner=[claude,koopa]
		{"lesson from draft", "lesson", "draft", []string{"ready", "archived"}},
		{"lesson from imported", "lesson", "imported", []string{"draft", "archived"}},
		{"empty note type", "", "draft", nil},
		{"empty current status", "lesson", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := svc.Transitions(tt.noteType, tt.current)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Transitions(%q, %q) mismatch (-want +got):\n%s", tt.noteType, tt.current, diff)
			}
		})
	}
}

// TestAdvanceable checks that the "still awaits my decision" predicate is closed
// under a nil contract and, when open, asks the contract with the operator as
// the actor — so an onward step someone else owns does not count. The contract
// is synthetic (states a, b, c) to keep the test about the actor wiring, not any
// real vault's status words.
func TestAdvanceable(t *testing.T) {
	t.Parallel()

	// a→b is owned by the operator; b→c is owned by someone else, so only a is
	// advanceable by the operator.
	contract := &schema.Schema{Lifecycle: []schema.Stage{
		{Status: "b", AppliesTo: []string{"doc"}, From: []string{"a"}, Owner: []string{"koopa"}},
		{Status: "c", AppliesTo: []string{"doc"}, From: []string{"b"}, Owner: []string{"bot"}},
	}}

	if status.NewService(t.TempDir(), nil).Advanceable("doc", "a") {
		t.Error("Advanceable on a closed write face = true, want false")
	}

	svc := status.NewService(t.TempDir(), contract)
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"operator owns the onward step", "a", true},
		{"onward step owned by someone else", "b", false},
		{"no onward step defined", "c", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := svc.Advanceable("doc", tt.status); got != tt.want {
				t.Errorf("Advanceable(%q, %q) = %v, want %v", "doc", tt.status, got, tt.want)
			}
		})
	}
}

func TestFlipByteIdentical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{
			name: "trailing spaces on other lines survive untouched",
			content: "---\n" +
				"title: L05  \n" +
				"type: lesson  \n" +
				"status: draft\n" +
				"created: 2026-06-01   \n" +
				"---\n" +
				"\nbody\n",
		},
		{
			name: "CRLF line ending on the status line specifically",
			content: "---\n" +
				"title: L05\n" +
				"type: lesson\n" +
				"status: draft\r\n" +
				"created: 2026-06-01\n" +
				"---\n" +
				"\nbody\n",
		},
		{
			name: "a column-zero status: line inside the body is ignored",
			content: "---\n" +
				"title: L05\n" +
				"type: lesson\n" +
				"status: draft\n" +
				"---\n" +
				"\nstatus: this looks like frontmatter but is body text\n\nMore body.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := newVault(t)
			svc := status.NewService(root, loadContract(t))

			writeNote(t, root, tt.content)
			commitAll(t, root)

			if err := svc.Flip(t.Context(), testRel, "draft", "ready"); err != nil {
				t.Fatalf("Flip() = %v, want nil", err)
			}

			want := strings.Replace(tt.content, "status: draft", "status: ready", 1)
			got := readNote(t, root)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("file mismatch after flip (-want +got):\n%s", diff)
			}
		})
	}
}

// TestFlipGitAddFailureWrapsErrCommitFailed guards against the file being
// rewritten on disk while a failing `git add` (staging, not just the final
// `git commit`) is reported as a plain error instead of ErrCommitFailed —
// which would route callers to a generic 500 instead of the
// "file already changed, here is the git error" presentation. A held
// `.git/index.lock` is a realistic, deterministic stand-in for the
// concurrent-git-process contention this guards against (another Flip, an
// Obsidian git-sync plugin, a cron script).
func TestFlipGitAddFailureWrapsErrCommitFailed(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)

	lockPath := filepath.Join(root, ".git", "index.lock")
	if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
		t.Fatalf("create index.lock: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(lockPath) })

	err := svc.Flip(t.Context(), testRel, "draft", "ready")
	if !errors.Is(err, status.ErrCommitFailed) {
		t.Fatalf("Flip() = %v, want an error wrapping %v", err, status.ErrCommitFailed)
	}

	// writeAtomic already ran before commit() ever touched git: the file on
	// disk must show the rewritten status even though no commit exists —
	// exactly the dangerous, silently-diverged state ErrCommitFailed exists
	// to make unmistakable.
	if got := readNote(t, root); !strings.Contains(got, "status: ready") {
		t.Errorf("file after failed git add = %q, want it already rewritten to ready despite the failed commit", got)
	}
}

// TestFlipConcurrentNeverLiesInTheCommitMessage reproduces the double-tab /
// double-click race (the UI offers every legal transition as its own
// pressable key, so two can be in flight at once): two goroutines flip
// the SAME note to two different target statuses concurrently. Without Service serializing Flip, the loser's
// writeAtomic can be silently overwritten by the winner and the loser's
// commit() then stages and commits whatever the winner left on disk under
// the loser's own from→to commit message — a false audit-trail entry that
// still returns err == nil to the loser. With Flip holding the Service's
// lock for its whole duration, exactly one of the two must succeed, and
// the loser must get a real, actionable error (ErrStale — the winner
// already moved the file out from under it) rather than a false success.
func TestFlipConcurrentNeverLiesInTheCommitMessage(t *testing.T) {
	t.Parallel()
	root := newVault(t)
	svc := status.NewService(root, loadContract(t))

	writeNote(t, root, lessonContent("draft"))
	commitAll(t, root)
	before := commitCount(t, root)

	var errReady, errArchived error
	var wg sync.WaitGroup
	wg.Go(func() { errReady = svc.Flip(t.Context(), testRel, "draft", "ready") })
	wg.Go(func() { errArchived = svc.Flip(t.Context(), testRel, "draft", "archived") })
	wg.Wait()

	succeeded := 0
	if errReady == nil {
		succeeded++
	}
	if errArchived == nil {
		succeeded++
	}
	if succeeded != 1 {
		t.Fatalf("exactly one concurrent Flip should succeed, got %d (errReady=%v, errArchived=%v)", succeeded, errReady, errArchived)
	}

	loser, wantStatus := errArchived, "ready"
	if errReady != nil {
		loser, wantStatus = errReady, "archived"
	}
	if !errors.Is(loser, status.ErrStale) {
		t.Errorf("losing Flip() = %v, want it to report %v (the winner already moved the file), not a false success", loser, status.ErrStale)
	}

	// The file and the commit message must agree on which transition
	// actually happened — never the false-audit-trail state the race
	// produces without serialization.
	final := readNote(t, root)
	if want := "status: " + wantStatus; !strings.Contains(final, want) {
		t.Errorf("file after concurrent flips = %q, want it to contain %q", final, want)
	}
	wantSubject := "status(" + testRel + "): draft → " + wantStatus + " (via yomihon)"
	if got := strings.TrimSpace(runGit(t, root, "log", "-1", "--format=%s")); got != wantSubject {
		t.Errorf("commit subject = %q, want %q (must match what was actually committed)", got, wantSubject)
	}
	if after := commitCount(t, root); after != before+1 {
		t.Errorf("commit count = %d, want %d (exactly one new commit; the loser must not also commit)", after, before+1)
	}
}
