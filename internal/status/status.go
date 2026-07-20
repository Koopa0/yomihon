// Package status is the write face: the only package in this repo
// allowed to write vault files or run git. It flips a note's frontmatter
// `status` field — a surgical, single-line rewrite, never a YAML
// re-serialization — and commits the change under the vault's own git
// identity so the commit author is genuinely Koopa.
package status

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Sentinel errors. Callers match with errors.Is.
var (
	// ErrClosed means the write face is fail-closed: either the vault contract
	// failed to load or its pinned root capability has been closed. Fault
	// tolerance is asymmetric by direction — reading tolerates a missing
	// contract, but a write without one could destroy a file, so writing refuses.
	ErrClosed = errors.New("status: write face is closed")
	// ErrArtifactPolicyUnavailable means the core contract loaded but its
	// artifact policy is unavailable, so instance writes cannot be classified.
	ErrArtifactPolicyUnavailable = errors.New("status: artifact policy unavailable")
	// ErrNonInstance means the requested path is a readable artifact rather
	// than a governed note instance.
	ErrNonInstance = errors.New("status: target is not a governable artifact")
	// ErrInvalidPath means a status request did not name a local vault-relative
	// slash path.
	ErrInvalidPath = errors.New("status: invalid vault-relative path")
	// ErrStale means the submitted form's "from" no longer matches the
	// note's on-disk status: the page was loaded before someone else
	// changed the file.
	ErrStale = errors.New("status: note is stale, reload and try again")
	// ErrConcurrentWrite means the named regular file, its parent identity,
	// mode, mtime, or exact bytes changed between Flip's descriptor read and
	// its pre-write recheck. An external tool such as Obsidian raced the flip
	// itself. Distinct from ErrStale: the two cases carry different user-facing
	// presentations and this sentinel must not satisfy errors.Is(err, ErrStale).
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
	// ErrDurabilityUnsupported means the running platform cannot prove that an
	// atomic rename's directory entry reached durable storage. The write face
	// refuses before reading or creating any vault path rather than changing a
	// note and presenting an unconfirmed publication as success.
	ErrDurabilityUnsupported = errors.New("status: durable publication is unsupported on this platform")
	// ErrPublishUncertain means the atomic replacement completed, but the
	// containing directory could not be synchronized. The new bytes are visible
	// now, but their survival across an immediate crash was not confirmed.
	ErrPublishUncertain = errors.New("status: note rewritten but durable publication was not confirmed")
	// ErrCommitFailed means the file was already rewritten on disk but the
	// git commit failed. yomihon deliberately does not roll back — a
	// rollback is a second write that would hide what happened, and the
	// vault git error is surfaced (this is a local, single-operator tool;
	// there is no one else who could read it) so Koopa can fix it by hand.
	ErrCommitFailed = errors.New("status: note rewritten but git commit failed")
)

const (
	// CoreUnavailableDiagnostic is the reading page's stable explanation for a
	// write face closed by an unavailable core contract.
	CoreUnavailableDiagnostic = "vault contract 無法使用；生命週期寫入已關閉（fail-closed）。"
	// DurablePublicationUnavailableDiagnostic is the stable reading-page
	// explanation for a platform on which the status write face cannot prove
	// durable publication.
	DurablePublicationUnavailableDiagnostic = "此平台無法確認狀態檔案的耐久發布；生命週期寫入已關閉（fail-closed）。"
)

var errNotRegular = errors.New("status: target is not a regular file")

// ArtifactPolicyUnavailableError carries the contract-derived diagnostic for
// a write refused because instance classification is unavailable.
type ArtifactPolicyUnavailableError struct {
	diagnostic string
}

// Error returns the artifact policy diagnostic verbatim.
func (e *ArtifactPolicyUnavailableError) Error() string {
	return e.diagnostic
}

// Unwrap makes the error identifiable with ErrArtifactPolicyUnavailable.
func (e *ArtifactPolicyUnavailableError) Unwrap() error {
	return ErrArtifactPolicyUnavailable
}

// actor is the single local operator yomihon writes on behalf of. yomihon is
// a local-only, single-user tool; there is no multi-user concept.
const actor = "koopa"

// Lifecycle is the write face: it flips one note's frontmatter status field
// and commits the change. Constructed once per process with the loaded
// vault contract (or nil, meaning fail-closed).
type Lifecycle struct {
	root     *os.Root
	contract *schema.Contract
	policy   schema.ArtifactPolicy
	// mu serializes View, Flip, provenance reads, and Close: the vault's git repo
	// (index, HEAD) and root capability are shared resources. Neither another
	// flip nor a reading page may observe the rename-to-commit interval, and
	// Close cannot release the root under an operation. One lifecycle-wide lock
	// is deliberately simpler than per-file locking: this is a local,
	// single-operator tool where correctness matters far more than throughput.
	mu sync.Mutex
}

type fileSnapshot struct {
	data       []byte
	file       os.FileInfo
	parent     os.FileInfo
	parentPath string
	name       string
}

// Open pins an independent write capability for source's already-selected
// vault. The pathname is used only to open that second capability; both open
// directory identities must match before Open returns.
//
// A nil core contract or its unavailable artifact policy closes the write
// face: no transitions are offered and every Flip is rejected. Deriving the
// policy here makes it impossible to combine lifecycle authority from one
// contract with instance classification from another.
func Open(source *vault.Reader, contract *schema.Contract) (*Lifecycle, error) {
	if source == nil {
		return nil, errors.New("status: open lifecycle: vault reader is nil")
	}
	root, err := os.OpenRoot(source.Name())
	if err != nil {
		return nil, fmt.Errorf("status: open lifecycle root: %w", err)
	}
	same, err := source.SameRoot(root)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("status: compare lifecycle root: %w", err), root.Close())
	}
	if !same {
		return nil, errors.Join(errors.New("status: vault root changed while opening lifecycle"), root.Close())
	}
	var policy schema.ArtifactPolicy
	if contract != nil {
		policy = contract.ArtifactPolicy()
	}
	return &Lifecycle{root: root, contract: contract, policy: policy}, nil
}

// Close waits for an in-progress Flip or provenance read, then releases the
// pinned write capability. Calls after Close fail closed.
func (lc *Lifecycle) Close() error {
	return lc.close(closeHooks{})
}

type closeHooks struct {
	beforeLock func()
	afterLock  func()
}

func (lc *Lifecycle) close(hooks closeHooks) error {
	if lc == nil {
		return nil
	}
	if hooks.beforeLock != nil {
		hooks.beforeLock()
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if hooks.afterLock != nil {
		hooks.afterLock()
	}
	if lc.root == nil {
		return nil
	}
	root := lc.root
	lc.root = nil
	return root.Close()
}

// View is one immutable read-only lifecycle projection captured from a
// Lifecycle. Its query methods perform no filesystem or contract-source I/O,
// so one request can derive every status decision from the same authority
// sample. A later request captures another View and observes a latched source
// change.
type View struct {
	contract   *schema.Contract
	policy     schema.ArtifactPolicy
	available  bool
	diagnostic string
}

// View captures the write face's current read-only authority. Flip does not use
// this snapshot: writes revalidate the source under the publication lock.
func (lc *Lifecycle) View() View {
	if lc == nil {
		return View{diagnostic: CoreUnavailableDiagnostic}
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.root == nil {
		return View{diagnostic: CoreUnavailableDiagnostic}
	}
	if lc.contract == nil {
		return View{diagnostic: CoreUnavailableDiagnostic}
	}
	policy := lc.policy.Capture()
	if !policy.Available() {
		return View{diagnostic: policy.Diagnostic()}
	}
	return View{contract: lc.contract, policy: policy, available: true}
}

// Closed reports whether this captured view can classify governed instances.
func (v View) Closed() bool {
	return !v.available
}

// Diagnostic explains why this captured view is closed. An empty result means
// lifecycle reads are available.
func (v View) Diagnostic() string {
	if v.available {
		return ""
	}
	if v.diagnostic != "" {
		return v.diagnostic
	}
	return CoreUnavailableDiagnostic
}

// WriteDiagnostic explains why this captured authority cannot offer status
// transitions. Contract and artifact-policy failures also invalidate read-only
// instance projections and therefore take precedence over a platform-only
// write limitation. An empty result means a POST may be offered.
func (v View) WriteDiagnostic() string {
	if diagnostic := v.Diagnostic(); diagnostic != "" {
		return diagnostic
	}
	if !durablePublicationSupported {
		return DurablePublicationUnavailableDiagnostic
	}
	return ""
}

// Transitions returns the operator-owned target statuses from current in
// contract order. It is pure over the captured view.
func (v View) Transitions(relPath, noteType, current string) []string {
	relPath, _, err := normalizeRelPath(relPath)
	if err != nil || v.WriteDiagnostic() != "" || noteType == "" || current == "" ||
		v.policy.IsNonInstance(relPath) {
		return nil
	}
	var legal []string
	for _, to := range v.contract.Statuses(noteType) {
		if err := v.contract.Transition(noteType, current, to, actor); err == nil {
			legal = append(legal, to)
		}
	}
	return legal
}

// Order returns the default note group's statuses in declared order. A nil
// result means this view is closed; an empty non-nil result is a valid empty
// declaration.
func (v View) Order() []string {
	if !v.available {
		return nil
	}
	order := slices.Clone(v.contract.Statuses(""))
	if order == nil {
		return []string{}
	}
	return order
}

// Advanceable reports whether the operator owns a named onward transition,
// excluding wildcard-predecessor escape transitions.
func (v View) Advanceable(noteType, current string) bool {
	return v.available && v.contract.AdvanceableBy(noteType, current, actor)
}

// LastCommitHash returns the short hash of the most recent commit that touched
// rel only while the clean current file still matches expected. expected is
// the digest of the exact bytes rendered by the reading request, so a flip or
// external edit between the note read and this git query yields no provenance
// instead of pairing old content with a newer commit. internal/status is the
// only package permitted to run git; a read-only query is no exception.
func (lc *Lifecycle) LastCommitHash(
	ctx context.Context,
	rel string,
	expected [sha256.Size]byte,
) (string, error) {
	return lc.lastCommitHash(ctx, rel, expected, provenanceHooks{})
}

type provenanceHooks struct {
	beforeLock func()
	afterLock  func()
	afterGit   func()
}

func (lc *Lifecycle) lastCommitHash(
	ctx context.Context,
	rel string,
	expected [sha256.Size]byte,
	hooks provenanceHooks,
) (string, error) {
	relSlash, relPath, err := normalizeRelPath(rel)
	if err != nil {
		return "", fmt.Errorf("status: hash %q: %w", rel, err)
	}
	if hooks.beforeLock != nil {
		hooks.beforeLock()
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if hooks.afterLock != nil {
		hooks.afterLock()
	}
	if lc.root == nil {
		return "", ErrClosed
	}
	matches, err := fileMatches(lc.root, relPath, relSlash, expected)
	if err != nil {
		return "", fmt.Errorf("status: check commit hash for %s: %w", relSlash, err)
	}
	if !matches {
		return "", nil
	}
	dirty, err := lc.dirty(ctx, relPath)
	if err != nil {
		return "", fmt.Errorf("status: check commit hash for %s: %w", relSlash, err)
	}
	if dirty {
		return "", nil
	}
	out, err := runGit(ctx, lc.root, "--literal-pathspecs", "log", "-1", "--format=%h", "--", relPath)
	if err != nil {
		return "", fmt.Errorf("status: last commit hash %s: %w", relSlash, err)
	}
	if hooks.afterGit != nil {
		hooks.afterGit()
	}
	matches, err = fileMatches(lc.root, relPath, relSlash, expected)
	if err != nil {
		return "", fmt.Errorf("status: recheck commit hash for %s: %w", relSlash, err)
	}
	if !matches {
		return "", nil
	}
	return string(bytes.TrimSpace(out)), nil
}

func fileMatches(
	root *os.Root,
	rel, relSlash string,
	expected [sha256.Size]byte,
) (bool, error) {
	snapshot, err := readRegularFile(root, rel, relSlash)
	if err != nil {
		return false, err
	}
	return sha256.Sum256(snapshot.data) == expected, nil
}

// Flip moves the note at rel from status "from" to status "to": it
// validates the transition against the contract, rewrites exactly the
// frontmatter status line, and commits the change under the vault's own
// git identity. Every refusal before publication leaves the file untouched. A
// commit failure is reported after the atomic replacement and therefore leaves
// the rewritten file in place for the operator to recover explicitly.
//
// Flip holds the Lifecycle's lock for its entire duration (see the mu field
// doc): concurrent callers are serialized, never interleaved.
func (lc *Lifecycle) Flip(ctx context.Context, rel, from, to string) error {
	return lc.flip(ctx, rel, from, to, flipHooks{})
}

type flipHooks struct {
	beforeLock    func()
	afterLock     func()
	beforePublish func()
	afterPublish  func()
}

func (lc *Lifecycle) flip(ctx context.Context, rel, from, to string, hooks flipHooks) error {
	relSlash, rel, err := normalizeRelPath(rel)
	if err != nil {
		return err
	}
	if hooks.beforeLock != nil {
		hooks.beforeLock()
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if hooks.afterLock != nil {
		hooks.afterLock()
	}
	if lc.root == nil {
		return ErrClosed
	}

	err = lc.validateWriteTarget(relSlash)
	if err != nil {
		return err
	}

	source, err := readRegularFile(lc.root, rel, relSlash)
	if err != nil {
		return err
	}
	data := source.data

	n := vault.Parse(relSlash, data)
	if current := n.Status(); current != from {
		return fmt.Errorf("%w: status is %q, page said %q", ErrStale, current, from)
	}

	if err = lc.contract.Transition(n.Type(), from, to, actor); err != nil {
		return fmt.Errorf("status: %s %s -> %s: %w", relSlash, from, to, err)
	}

	dirty, err := lc.dirty(ctx, rel)
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
	if err := lc.publishStatus(rel, relSlash, &source, rewritten, hooks); err != nil {
		return err
	}
	if err := lc.commit(ctx, rel, relSlash, from, to); err != nil {
		return err
	}
	return nil
}

// publishStatus crosses Flip's irreversible boundary. It revalidates the
// current artifact authority inside replaceRegularFile's publication section,
// atomically installs the rewritten bytes, and reports only after the durable
// replacement is visible.
func (lc *Lifecycle) publishStatus(
	rel, relSlash string,
	source *fileSnapshot,
	rewritten []byte,
	hooks flipHooks,
) error {
	err := replaceRegularFile(
		lc.root,
		rel,
		relSlash,
		source,
		rewritten,
		publishHooks{beforeAuthority: hooks.beforePublish},
		func() error {
			_, authorityErr := lc.validatedArtifactPolicy()
			return authorityErr
		},
	)
	if err != nil {
		if errors.Is(err, ErrArtifactPolicyUnavailable) {
			return err
		}
		return fmt.Errorf("status: write %s: %w", relSlash, err)
	}
	if hooks.afterPublish != nil {
		hooks.afterPublish()
	}
	return nil
}

func (lc *Lifecycle) validateWriteTarget(relSlash string) error {
	if !durablePublicationSupported {
		return ErrDurabilityUnsupported
	}
	if lc.contract == nil {
		return ErrClosed
	}
	policy, err := lc.validatedArtifactPolicy()
	if err != nil {
		return err
	}
	if policy.IsNonInstance(relSlash) {
		return ErrNonInstance
	}
	return nil
}

func (lc *Lifecycle) validatedArtifactPolicy() (schema.ArtifactPolicy, error) {
	policy := lc.policy.ValidateSource()
	if !policy.Available() {
		return schema.ArtifactPolicy{}, &ArtifactPolicyUnavailableError{diagnostic: policy.Diagnostic()}
	}
	return policy, nil
}

func readRegularFile(root *os.Root, rel, relSlash string) (fileSnapshot, error) {
	parent, parentPath, name, err := openRegularParent(root, rel, relSlash)
	if err != nil {
		return fileSnapshot{}, err
	}
	parentInfo, err := parent.Stat(".")
	if err != nil {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("status: stat parent of %s: %w", relSlash, err)
	}
	before, err := parent.Lstat(name)
	if err != nil {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("status: stat %s: %w", relSlash, err)
	}
	if !before.Mode().IsRegular() {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("%w: %s", errNotRegular, relSlash)
	}
	file, err := parent.Open(name)
	if err != nil {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("status: read %s: %w", relSlash, err)
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close() //nolint:errcheck // the stat error is the actionable failure
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("status: read %s: stat open file: %w", relSlash, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		_ = file.Close() //nolint:errcheck // the identity mismatch is the actionable failure
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("%w: %s changed while opening", ErrConcurrentWrite, relSlash)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		_ = file.Close() //nolint:errcheck // the read error is the actionable failure
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("status: read %s: %w", relSlash, err)
	}
	if err = file.Close(); err != nil {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("status: read %s: close: %w", relSlash, err)
	}
	after, err := parent.Lstat(name)
	if err != nil {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("status: stat %s after read: %w", relSlash, err)
	}
	if !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("%w: %s changed while reading", ErrConcurrentWrite, relSlash)
	}
	snapshot := fileSnapshot{
		data:       data,
		file:       opened,
		parent:     parentInfo,
		parentPath: parentPath,
		name:       name,
	}
	closeRoot(parent)
	return snapshot, nil
}

func openRegularParent(root *os.Root, rel, relSlash string) (parent *os.Root, parentPath, name string, err error) {
	current, err := root.OpenRoot(".")
	if err != nil {
		return nil, "", "", fmt.Errorf("status: duplicate vault root for %s: %w", relSlash, err)
	}
	parentPath, name = filepath.Split(rel)
	parentPath = filepath.Clean(parentPath)
	if parentPath == "." {
		return current, parentPath, name, nil
	}

	for component := range strings.SplitSeq(parentPath, string(filepath.Separator)) {
		before, statErr := current.Lstat(component)
		if statErr != nil {
			closeRoot(current)
			return nil, "", "", fmt.Errorf("status: stat directory of %s: %w", relSlash, statErr)
		}
		if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			closeRoot(current)
			return nil, "", "", fmt.Errorf("status: open %s: path contains a non-directory or symbolic link", relSlash)
		}
		next, openErr := current.OpenRoot(component)
		if openErr != nil {
			closeRoot(current)
			return nil, "", "", fmt.Errorf("status: open directory of %s: %w", relSlash, openErr)
		}
		opened, openStatErr := next.Stat(".")
		after, afterErr := current.Lstat(component)
		if openStatErr != nil {
			closeRoot(current)
			closeRoot(next)
			return nil, "", "", fmt.Errorf("status: stat open directory of %s: %w", relSlash, openStatErr)
		}
		if afterErr != nil {
			closeRoot(current)
			closeRoot(next)
			return nil, "", "", fmt.Errorf("status: restat directory of %s: %w", relSlash, afterErr)
		}
		if !after.IsDir() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			closeRoot(current)
			closeRoot(next)
			return nil, "", "", fmt.Errorf("%w: directory of %s changed while opening", ErrConcurrentWrite, relSlash)
		}
		if closeErr := current.Close(); closeErr != nil {
			closeRoot(next)
			return nil, "", "", fmt.Errorf("status: close directory of %s: %w", relSlash, closeErr)
		}
		current = next
	}
	return current, parentPath, name, nil
}

func closeRoot(root *os.Root) {
	// Root holds only a directory descriptor. A close failure cannot change a
	// completed read or rename, and turning it into a Flip failure after rename
	// would incorrectly skip the required git commit.
	_ = root.Close() //nolint:errcheck // directory-descriptor cleanup is best-effort
}

// normalizeRelPath validates and normalizes the external slash path before any
// contract, filesystem, or git decision. It returns both slash and OS forms.
func normalizeRelPath(rel string) (relSlash, osPath string, err error) {
	if rel == "" || strings.Contains(rel, `\`) || pathpkg.IsAbs(rel) {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidPath, rel)
	}
	normalized := pathpkg.Clean(rel)
	osPath = filepath.FromSlash(normalized)
	if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") || !filepath.IsLocal(osPath) {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidPath, rel)
	}
	return normalized, osPath, nil
}

// rewriteStatusLine replaces the single "status:" line inside data's
// frontmatter block with "status: <to>", leaving every other byte —
// including the block's own delimiters, quoted values, comments, and the
// body — byte-identical to the original. It never re-serializes YAML.
func rewriteStatusLine(data []byte, to string) ([]byte, error) {
	block, found := vault.SplitFrontmatter(data)
	if !found {
		return nil, ErrStatusLine
	}

	lines := bytes.Split(block.Content, []byte("\n"))
	target := -1
	targetOffset := 0
	offset := 0
	for i, line := range lines {
		if bytes.HasPrefix(line, []byte("status:")) {
			if target != -1 {
				return nil, ErrStatusLine
			}
			target = i
			targetOffset = offset
		}
		offset += len(line) + 1
	}
	if target == -1 {
		return nil, ErrStatusLine
	}

	replacement := "status: " + to
	if bytes.HasSuffix(lines[target], []byte("\r")) {
		replacement += "\r"
	}
	start := block.ContentStart + targetOffset
	end := start + len(lines[target])
	out := make([]byte, 0, len(data)-len(lines[target])+len(replacement))
	out = append(out, data[:start]...)
	out = append(out, replacement...)
	out = append(out, data[end:]...)
	return out, nil
}

type publishHooks struct {
	beforeAuthority func()
	afterAuthority  func()
	syncTemp        func(*os.File) error
	syncParent      func(*os.Root) error
}

// replaceRegularFile prepares the complete replacement beside the source,
// then reopens the source's named parent, validates caller-supplied write
// authority, and finally verifies the same regular-file identity, mode, mtime,
// and bytes immediately before one descriptor-relative rename. The rename is
// atomic, so readers see either complete version.
func replaceRegularFile(
	root *os.Root,
	rel, relSlash string,
	source *fileSnapshot,
	data []byte,
	hooks publishHooks,
	authorize func() error,
) error {
	preparedParent, err := openSameParent(root, rel, relSlash, source)
	if err != nil {
		return err
	}
	tmpName, err := writeTemp(preparedParent, data, source.file.Mode().Perm(), hooks.syncTemp)
	if err != nil {
		closeRoot(preparedParent)
		return err
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = preparedParent.Remove(tmpName) //nolint:errcheck // best-effort cleanup after another error is already primary
		}
		closeRoot(preparedParent)
	}()

	if hooks.beforeAuthority != nil {
		hooks.beforeAuthority()
	}

	publishParent, err := openSameParent(root, rel, relSlash, source)
	if err != nil {
		return err
	}
	if authorize != nil {
		if err = authorize(); err != nil {
			closeRoot(publishParent)
			return err
		}
	}
	if hooks.afterAuthority != nil {
		hooks.afterAuthority()
	}
	if err = sourceUnmodified(publishParent, relSlash, source); err != nil {
		closeRoot(publishParent)
		return err
	}
	// Root.Rename is atomic but not a conditional rename by file identity.
	// A cooperating local process should not replace the name in this final
	// instruction window; if it does, the rename replaces that entry without
	// following it and remains confined to the validated parent directory.
	if err = publishParent.Rename(tmpName, source.name); err != nil {
		closeRoot(publishParent)
		return fmt.Errorf("rename temp file: %w", err)
	}
	removeTemp = false
	syncParent := syncDirectory
	if hooks.syncParent != nil {
		syncParent = hooks.syncParent
	}
	if err = syncParent(publishParent); err != nil {
		closeRoot(publishParent)
		return fmt.Errorf("%w: sync containing directory: %w", ErrPublishUncertain, err)
	}
	closeRoot(publishParent)
	return nil
}

func openSameParent(root *os.Root, rel, relSlash string, source *fileSnapshot) (*os.Root, error) {
	parent, parentPath, name, err := openRegularParent(root, rel, relSlash)
	if err != nil {
		return nil, err
	}
	parentInfo, err := parent.Stat(".")
	if err != nil {
		closeRoot(parent)
		return nil, fmt.Errorf("status: stat parent of %s: %w", relSlash, err)
	}
	if parentPath != source.parentPath || name != source.name || !os.SameFile(parentInfo, source.parent) {
		closeRoot(parent)
		return nil, fmt.Errorf("%w: directory of %s changed while flipping", ErrConcurrentWrite, relSlash)
	}
	return parent, nil
}

func sourceUnmodified(parent *os.Root, relSlash string, source *fileSnapshot) error {
	current, opened, err := readCurrentSource(parent, relSlash, source)
	if err != nil {
		return err
	}
	after, err := parent.Lstat(source.name)
	if err != nil {
		return fmt.Errorf("%w: %s changed after reread: %w", ErrConcurrentWrite, relSlash, err)
	}
	if !after.Mode().IsRegular() || !os.SameFile(after, opened) || !after.ModTime().Equal(source.file.ModTime()) || after.Mode() != source.file.Mode() || !bytes.Equal(current, source.data) {
		return fmt.Errorf("%w: %s changed while flipping", ErrConcurrentWrite, relSlash)
	}
	return nil
}

func readCurrentSource(parent *os.Root, relSlash string, source *fileSnapshot) (data []byte, info os.FileInfo, err error) {
	before, err := parent.Lstat(source.name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("%w: %s was removed while flipping", ErrConcurrentWrite, relSlash)
		}
		return nil, nil, fmt.Errorf("status: stat %s before write: %w", relSlash, err)
	}
	if !before.Mode().IsRegular() || !os.SameFile(before, source.file) {
		return nil, nil, fmt.Errorf("%w: %s changed identity while flipping", ErrConcurrentWrite, relSlash)
	}
	file, err := parent.Open(source.name)
	if err != nil {
		return nil, nil, fmt.Errorf("status: reopen %s before write: %w", relSlash, err)
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close() //nolint:errcheck // the stat error is the actionable failure
		return nil, nil, fmt.Errorf("status: stat open %s before write: %w", relSlash, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(opened, source.file) {
		_ = file.Close() //nolint:errcheck // the identity mismatch is the actionable failure
		return nil, nil, fmt.Errorf("%w: %s changed identity while reopening", ErrConcurrentWrite, relSlash)
	}
	current, err := io.ReadAll(file)
	if err != nil {
		_ = file.Close() //nolint:errcheck // the read error is the actionable failure
		return nil, nil, fmt.Errorf("status: reread %s before write: %w", relSlash, err)
	}
	if err = file.Close(); err != nil {
		return nil, nil, fmt.Errorf("status: close %s before write: %w", relSlash, err)
	}
	return current, opened, nil
}

func writeTemp(parent *os.Root, data []byte, mode os.FileMode, syncFile func(*os.File) error) (string, error) {
	name := ".yomihon-status-" + rand.Text() + ".tmp"
	tmp, err := parent.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()         //nolint:errcheck // the write error is the actionable failure
		_ = parent.Remove(name) //nolint:errcheck // best-effort cleanup after the primary write error
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err = tmp.Chmod(mode); err != nil {
		_ = tmp.Close()         //nolint:errcheck // the chmod error is the actionable failure
		_ = parent.Remove(name) //nolint:errcheck // best-effort cleanup after the primary chmod error
		return "", fmt.Errorf("chmod temp file: %w", err)
	}
	if syncFile == nil {
		syncFile = (*os.File).Sync
	}
	if err = syncFile(tmp); err != nil {
		_ = tmp.Close()         //nolint:errcheck // the sync error is the actionable failure
		_ = parent.Remove(name) //nolint:errcheck // best-effort cleanup after the primary sync error
		return "", fmt.Errorf("sync temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		_ = parent.Remove(name) //nolint:errcheck // best-effort cleanup after the primary close error
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return name, nil
}

// dirty reports whether rel has uncommitted changes in the vault's git
// working tree.
func (lc *Lifecycle) dirty(ctx context.Context, rel string) (bool, error) {
	out, err := runGit(ctx, lc.root, "--literal-pathspecs", "status", "--porcelain", "--", rel)
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
func (lc *Lifecycle) commit(ctx context.Context, rel, relSlash, from, to string) error {
	// replaceRegularFile has already rewritten the file on disk by the time this
	// runs, so a failure here — same as a failing `git commit` below — must
	// also carry ErrCommitFailed: the caller owes the operator the "file
	// already changed, here is the git error" presentation whether staging
	// or committing failed, not just the latter.
	if _, err := runGit(ctx, lc.root, "--literal-pathspecs", "add", "--", rel); err != nil {
		return fmt.Errorf("%w: git add %s: %w", ErrCommitFailed, relSlash, err)
	}
	msg := fmt.Sprintf("status(%s): %s → %s (via yomihon)", relSlash, from, to)
	if _, err := runGit(ctx, lc.root, "--literal-pathspecs", "commit", "--only", "-m", msg, "--", rel); err != nil {
		return fmt.Errorf("%w: %w", ErrCommitFailed, err)
	}
	return nil
}
