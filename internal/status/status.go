// Package status is the write face: the only package in this repo
// allowed to write vault files or run git. It flips a note's frontmatter
// `status` field — a surgical, single-line rewrite, never a YAML
// re-serialization — and commits the change under the vault's own git
// identity so the commit author is genuinely Koopa.
package status

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Sentinel errors. Callers match with errors.Is.
var (
	// ErrClosed means the write face is fail-closed: the vault contract
	// failed to load. Fault tolerance is asymmetric by direction — reading
	// tolerates a missing contract, but a write without one could destroy
	// a file, so writing refuses outright.
	ErrClosed = errors.New("status: write face is closed, no contract")
	// ErrStale means the submitted form's "from" no longer matches the
	// note's on-disk status: the page was loaded before someone else
	// changed the file.
	ErrStale = errors.New("status: note is stale, reload and try again")
	// ErrConcurrentWrite means the file's mtime changed between Flip's
	// initial read and its pre-write recheck — a live concurrent write
	// (an external tool such as Obsidian, not a stale page) raced the
	// flip itself. Distinct from ErrStale: the two cases carry different
	// user-facing presentations and this sentinel must not satisfy
	// errors.Is(err, ErrStale).
	ErrConcurrentWrite = errors.New("status: note changed while flipping")
	// ErrDirty means the target file already has uncommitted changes; a
	// flip here would silently fold an unrelated edit into a
	// Koopa-authored audit commit.
	ErrDirty = errors.New("status: note has uncommitted changes")
	// ErrStatusLine means the frontmatter block does not contain exactly
	// one line beginning with "status:" — a schema violation yomihon does
	// not repair. yomihon only reports faults; fixing the file belongs to
	// a human editor.
	ErrStatusLine = errors.New("status: frontmatter does not have exactly one status line")
	// ErrCommitFailed means the file was already rewritten on disk but the
	// git commit failed. yomihon deliberately does not roll back — a
	// rollback is a second write that would hide what happened, and the
	// vault git error is surfaced (this is a local, single-operator tool;
	// there is no one else who could read it) so Koopa can fix it by hand.
	ErrCommitFailed = errors.New("status: note rewritten but git commit failed")
)

// actor is the single local operator yomihon writes on behalf of. yomihon is
// a local-only, single-user tool; there is no multi-user concept.
const actor = "koopa"

// Service is the write face: it flips one note's frontmatter status field
// and commits the change. Constructed once per process with the loaded
// vault contract (or nil, meaning fail-closed).
type Service struct {
	root     string
	contract *schema.Schema
	// mu serializes Flip: the vault's git repo (index, HEAD) is one shared
	// resource, and two concurrent flips racing its read-check-write-commit
	// sequence can produce a commit whose message asserts a from→to
	// transition that does not match what was actually recorded — a false
	// audit-trail entry. A coarse per-Service lock is deliberately simpler
	// than per-file locking: this is a local, single-operator tool
	// where correctness matters far more than flip throughput.
	mu sync.Mutex
}

// NewService wires the write face for the vault rooted at root. A nil
// contract closes the write face (Closed reports true): no transitions are
// offered and every Flip is rejected. Reading elsewhere in yomihon does not
// go through this package and is unaffected by a nil contract.
func NewService(root string, contract *schema.Schema) *Service {
	return &Service{root: root, contract: contract}
}

// Closed reports whether the write face is fail-closed: true when the
// vault contract is unavailable.
func (s *Service) Closed() bool {
	return s.contract == nil
}

// Transitions returns the legal target statuses for the operator from a
// note's current status, in the contract's declared order. It returns nil
// when the write face is closed or either argument is empty — callers use
// this to decide whether to render any transition keys at all.
func (s *Service) Transitions(noteType, current string) []string {
	if s.Closed() || noteType == "" || current == "" {
		return nil
	}
	var legal []string
	for _, to := range s.contract.Statuses(noteType) {
		if err := s.contract.Transition(noteType, current, to, actor); err == nil {
			legal = append(legal, to)
		}
	}
	return legal
}

// Order returns the default note group's statuses in the contract's declared
// toml order (schema.Statuses("")) — the stable status axis Home's Lifecycle
// block lists, independent of any one note. It returns nil when the
// write face is closed. This is read-only schema vocabulary, not a transition
// decision, and the enum still traces to the toml: nothing is
// hardcoded here.
func (s *Service) Order() []string {
	if s.Closed() {
		return nil
	}
	return s.contract.Statuses("")
}

// Advanceable reports whether the operator still has a legal onward move for a
// note of the given type and status: a named forward transition the operator
// owns, ignoring the always-available retire-to-archive escape. It returns false
// when the write face is closed. The reading page uses it to tell which notes
// still await a decision from the operator, reusing the same contract the write
// path validates against — never a second copy of the state machine.
func (s *Service) Advanceable(noteType, status string) bool {
	if s.Closed() {
		return false
	}
	return s.contract.AdvanceableBy(noteType, status, actor)
}

// LastCommitHash returns the short hash of the most recent commit that touched
// rel, via a read-only `git log -1 --format=%h -- <rel>`. It is the provenance
// line the reading page shows beside a sealed (ready) note. internal/status is
// the only package permitted to run git; a read-only query is no
// exception to that boundary, which is why it lives here. Returns "" (no error)
// when rel has no commits yet — an un-committed note simply shows no provenance
// line.
func (s *Service) LastCommitHash(ctx context.Context, rel string) (string, error) {
	rel = filepath.FromSlash(rel)
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("status: hash %q: path escapes vault root", rel)
	}
	out, err := runGit(ctx, s.root, "log", "-1", "--format=%h", "--", rel)
	if err != nil {
		return "", fmt.Errorf("status: last commit hash %s: %w", filepath.ToSlash(rel), err)
	}
	return string(bytes.TrimSpace(out)), nil
}

// Flip moves the note at rel from status "from" to status "to": it
// validates the transition against the contract, rewrites exactly the
// frontmatter status line, and commits the change under the vault's own
// git identity. Every early-return path below leaves the file untouched.
//
// Flip holds the Service's lock for its entire duration (see the mu field
// doc): concurrent callers are serialized, never interleaved.
func (s *Service) Flip(ctx context.Context, rel, from, to string) error {
	if s.Closed() {
		return ErrClosed
	}

	rel = filepath.FromSlash(rel)
	if !filepath.IsLocal(rel) {
		return fmt.Errorf("status: flip %q: path escapes vault root", rel)
	}
	relSlash := filepath.ToSlash(rel)
	path := filepath.Join(s.root, rel)

	s.mu.Lock()
	defer s.mu.Unlock()

	before, err := os.Stat(path) // #nosec G703 -- rel validated local by filepath.IsLocal above; root is the operator's own vault
	if err != nil {
		return fmt.Errorf("status: stat %s: %w", relSlash, err)
	}
	data, err := os.ReadFile(path) // #nosec G304 G703 -- rel validated local by filepath.IsLocal above; root is the operator's own vault
	if err != nil {
		return fmt.Errorf("status: read %s: %w", relSlash, err)
	}

	n := vault.Parse(relSlash, data)
	if current := n.Status(); current != from {
		return fmt.Errorf("%w: status is %q, page said %q", ErrStale, current, from)
	}

	if err = s.contract.Transition(n.Type(), from, to, actor); err != nil {
		return fmt.Errorf("status: %s %s -> %s: %w", relSlash, from, to, err)
	}

	dirty, err := s.dirty(ctx, rel)
	if err != nil {
		return fmt.Errorf("status: check %s for uncommitted changes: %w", relSlash, err)
	}
	if dirty {
		return fmt.Errorf("%w: %s", ErrDirty, relSlash)
	}

	rewritten, err := rewriteStatusLine(data, to)
	if err != nil {
		return fmt.Errorf("%w: %s", err, relSlash)
	}

	// TOCTOU guard: nothing may have touched the file between the initial
	// stat above and the write below.
	if toctouErr := checkUnmodifiedSince(path, relSlash, before); toctouErr != nil {
		return toctouErr
	}

	if err = writeAtomic(path, rewritten, before.Mode().Perm()); err != nil {
		return fmt.Errorf("status: write %s: %w", relSlash, err)
	}

	if err := s.commit(ctx, rel, relSlash, from, to); err != nil {
		return err
	}
	return nil
}

// checkUnmodifiedSince reports whether path's mtime still matches before —
// i.e. nothing touched the file since Flip's initial read. A mismatch means
// a live concurrent write raced the flip itself — deliberately kept
// distinct from ErrStale's "the submitted form was already out of date",
// which carries a different user-facing presentation.
func checkUnmodifiedSince(path, relSlash string, before os.FileInfo) error {
	after, err := os.Stat(path) // #nosec G703 -- same validated path as the stat above
	if err != nil {
		return fmt.Errorf("status: stat %s: %w", relSlash, err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		return fmt.Errorf("%w: %s changed while flipping", ErrConcurrentWrite, relSlash)
	}
	return nil
}

// rewriteStatusLine replaces the single "status:" line inside data's
// frontmatter block with "status: <to>", leaving every other byte —
// including the block's own delimiters, quoted values, comments, and the
// body — byte-identical to the original. It never re-serializes YAML.
func rewriteStatusLine(data []byte, to string) ([]byte, error) {
	fm, body := vault.SplitFrontmatter(data)
	if fm == nil {
		return nil, ErrStatusLine
	}

	lines := bytes.Split(fm, []byte("\n"))
	target := -1
	for i, line := range lines {
		if bytes.HasPrefix(line, []byte("status:")) {
			if target != -1 {
				return nil, ErrStatusLine
			}
			target = i
		}
	}
	if target == -1 {
		return nil, ErrStatusLine
	}

	replacement := "status: " + to
	if bytes.HasSuffix(lines[target], []byte("\r")) {
		replacement += "\r"
	}
	lines[target] = []byte(replacement)

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(bytes.Join(lines, []byte("\n")))
	out.WriteString("\n---\n")
	out.Write(body)
	return out.Bytes(), nil
}

// writeAtomic replaces path's contents without ever leaving it observable
// half-written: it writes to a temp file in the same directory, restores
// the original mode, and renames over the target.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".yomihon-status-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			//nolint:errcheck,gosec // best-effort cleanup of our own os.CreateTemp output (G703 false positive: not attacker input); nothing meaningful to do with a removal error here
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		//nolint:errcheck // best-effort; the write error above is what's returned
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		//nolint:errcheck // best-effort; the chmod error above is what's returned
		tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil { // #nosec G703 -- tmpPath is our own os.CreateTemp output in dir, not attacker input
		return fmt.Errorf("rename temp file: %w", err)
	}
	ok = true
	return nil
}

// dirty reports whether rel has uncommitted changes in the vault's git
// working tree.
func (s *Service) dirty(ctx context.Context, rel string) (bool, error) {
	out, err := runGit(ctx, s.root, "status", "--porcelain", "--", rel)
	if err != nil {
		return false, err
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

// commit records the flip as one commit, authored by the vault's own git
// identity, never one yomihon sets itself — the audit meaning of the commit
// is "Koopa pressed it". relSlash is used in the commit
// message (a stable, slash-form path); rel is what's passed to git, which
// on this platform are the same string.
func (s *Service) commit(ctx context.Context, rel, relSlash, from, to string) error {
	// writeAtomic has already rewritten the file on disk by the time this
	// runs, so a failure here — same as a failing `git commit` below — must
	// also carry ErrCommitFailed: the caller owes the operator the "file
	// already changed, here is the git error" presentation whether staging
	// or committing failed, not just the latter.
	if _, err := runGit(ctx, s.root, "add", "--", rel); err != nil {
		return fmt.Errorf("%w: git add %s: %w", ErrCommitFailed, relSlash, err)
	}
	msg := fmt.Sprintf("status(%s): %s → %s (via yomihon)", relSlash, from, to)
	if _, err := runGit(ctx, s.root, "commit", "-m", msg); err != nil {
		return fmt.Errorf("%w: %w", ErrCommitFailed, err)
	}
	return nil
}

// runGit runs git against the vault at root. Arguments are always passed
// as a slice to exec.CommandContext, never through a shell, and a literal
// "--" separates flags from path arguments so a filename can never be
// misread as a git flag.
func runGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	fullArgs := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...) // #nosec G204 G702 -- args are fixed yomihon-constructed slices, never shell-interpreted
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %v: %w: %s", args, err, bytes.TrimSpace(out))
	}
	return out, nil
}
