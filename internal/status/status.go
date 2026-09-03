// Package status is the write face: the only package here allowed to write
// vault files. It flips a note's frontmatter status field as a surgical,
// single-line rewrite — never a YAML re-serialization — and leaves every other
// byte of the file exactly as it found it. Nothing authenticates who triggered
// a flip; the file's new bytes are the whole record.
package status

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
	"github.com/koopa0/yomihon/internal/wording"
)

// Sentinel errors. Callers match with errors.Is.
var (
	// ErrClosed means the write face is fail-closed: the vault contract failed
	// to load, or its pinned root capability has been closed. Reading tolerates
	// a missing contract; a write without one could destroy a file.
	ErrClosed = errors.New("write face is closed")
	// ErrArtifactPolicyUnavailable means the core contract loaded but its
	// artifact policy is unavailable, so instance writes cannot be classified.
	ErrArtifactPolicyUnavailable = errors.New("artifact policy unavailable")
	// ErrNonInstance means the requested path is a readable artifact rather
	// than a governed note instance.
	ErrNonInstance = errors.New("target is not a governable artifact")
	// ErrOutsideKnowledgeScope means the requested path is a note the lifecycle
	// never reaches: it lies outside the directories scan.knowledge_dirs names
	// as the knowledge layer, and the state machine runs only there.
	ErrOutsideKnowledgeScope = errors.New("target is outside the knowledge layer scan.knowledge_dirs declares")
	// ErrInvalidPath means a status request did not name a local vault-relative
	// slash path.
	ErrInvalidPath = errors.New("invalid vault-relative path")
	// ErrStale means the submitted form's "from" no longer matches the note's
	// on-disk status: the page was loaded before someone else changed the file.
	ErrStale = errors.New("note is stale, reload and try again")
	// ErrConcurrentWrite means the file's identity, mode, mtime or bytes
	// changed between Flip's descriptor read and its pre-write recheck. It is
	// distinct from ErrStale and does not satisfy errors.Is(err, ErrStale).
	ErrConcurrentWrite = errors.New("note changed while flipping")
	// ErrContentChanged means the note's bytes no longer carry the content
	// identity the caller read: something outside the status line was edited
	// after the page rendered. The status line's own divergence is ErrStale.
	ErrContentChanged = errors.New("note content changed after it was read")
	// ErrStatusLine means the frontmatter block does not contain exactly one
	// line beginning with "status:". yomihon reports; a human edits the file.
	ErrStatusLine = errors.New("frontmatter does not have exactly one status line")
	// ErrPublishedReserved means the flip named published as its target. That
	// status records a completed publication outside the vault, which nothing
	// here can attest, so the value enters a note only by hand.
	ErrPublishedReserved = errors.New("published records a completed publication and no publisher exists to attest one")
	// ErrStatusSyntaxUnsupported means the reader understands the note's
	// status but the frontmatter writes it in a form the surgical rewriter
	// cannot preserve — an explicit or quoted key, a flow mapping, an anchor
	// the rewrite would sever. The note is refused unchanged.
	ErrStatusSyntaxUnsupported = errors.New("frontmatter writes status in a syntax the surgical rewriter does not support")
	// ErrDurabilityUnsupported means the platform cannot prove an atomic
	// rename's directory entry reached durable storage, so the write face
	// refuses rather than leave new bytes a crash could silently discard.
	ErrDurabilityUnsupported = errors.New("durable install is unsupported on this platform")
	// ErrInstallStranded means another program edited the note inside the
	// install window and the edit could not be put back under the note's own
	// name. Both versions are on disk, one named in the error, and nothing was
	// removed. Unlike ErrConcurrentWrite it cannot say which the note carries.
	ErrInstallStranded = errors.New("an edit raced the flip and both versions were left on disk")
	// ErrInstallUncertain means the atomic replacement completed but the
	// containing directory could not be synchronized, so the new bytes are
	// visible without their survival across an immediate crash being confirmed.
	ErrInstallUncertain = errors.New("note rewritten but durability was not confirmed")
)

// The write face rewrites exactly the regular file a path names, so a symbolic
// link — whose target can sit outside the vault — and every other special
// entry is refused, at the leaf and at every directory on the way.
var (
	errNotRegular     = errors.New("target is not a regular file")
	errPathNotRegular = errors.New("path passes through a non-directory or symbolic link")
)

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

// Writer is the write face: it flips one note's frontmatter status field.
// Constructed once per process with the loaded contract, or nil to fail closed.
type Writer struct {
	root       *os.Root
	contract   *schema.Contract
	governance schema.Governance
	policy     schema.ArtifactPolicy
	// log carries the one thing this package says that no caller asked for: a
	// flip killed mid-write left a file behind and the sweep set it aside.
	log *slog.Logger
	// mu serializes View, Flip, ObservedStatus and Close: the pinned root is a
	// shared resource, two flips must never interleave their
	// read-check-replace windows, and Close cannot release the root under an
	// operation. A flip holds it across two synchronizations, so Flip and
	// ObservedStatus consult the request's context before they queue for it.
	mu sync.Mutex
	// receipts is this face's short-lived memory of flips whose confirmation
	// no reading page has shown yet, keyed by normalized path. Only the write
	// face can attest that a transition ran, so the attestation lives here
	// rather than in any address a hand can type. At most one per note.
	receipts map[string]receipt
	// receiptMu guards receipts alone, so a page consuming one never waits
	// behind a flip. It is taken alone or inside mu, never the other way.
	receiptMu sync.Mutex
}

// receipt is one unshown attestation: which status a completed flip left,
// and when, so a receipt that was never read does not vouch forever.
type receipt struct {
	from string
	at   time.Time
}

// receiptTTL bounds how long a completed flip vouches for its receipt: enough
// slack for a throttled tab, not enough to leave a stale attestation around.
const receiptTTL = 2 * time.Minute

type fileSnapshot struct {
	data       []byte
	file       os.FileInfo
	parent     os.FileInfo
	parentPath string
	name       string
}

// Open pins an independent write capability for source's already-selected
// vault; both open directory identities must match before it returns. A nil
// contract, or an unavailable artifact policy, closes the write face.
// governance is what the folder asserted about its own contract, which a nil
// contract cannot answer; with a contract it must be contract.Governance().
func Open(source *vaultfs.Reader, contract *schema.Contract, governance schema.Governance, log *slog.Logger) (*Writer, error) {
	if source == nil {
		panic("status: Open requires a non-nil Reader")
	}
	root, err := os.OpenRoot(source.Name())
	if err != nil {
		return nil, fmt.Errorf("open writer root: %w", err)
	}
	same, err := source.SameRoot(root)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("compare writer root: %w", err), root.Close())
	}
	if !same {
		return nil, errors.Join(errors.New("vault root changed while opening writer"), root.Close())
	}
	var policy schema.ArtifactPolicy
	if contract != nil {
		policy = contract.ArtifactPolicy()
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Writer{root: root, contract: contract, governance: governance, policy: policy, log: log}, nil
}

// Close waits for an in-progress Flip or status read, then releases the
// pinned write capability. Calls after Close fail closed.
func (w *Writer) Close() error {
	return w.close(closeHooks{})
}

type closeHooks struct {
	beforeLock func()
	afterLock  func()
}

func (w *Writer) close(hooks closeHooks) error {
	if w == nil {
		return nil
	}
	if hooks.beforeLock != nil {
		hooks.beforeLock()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if hooks.afterLock != nil {
		hooks.afterLock()
	}
	if w.root == nil {
		return nil
	}
	root := w.root
	w.root = nil
	return root.Close()
}

// Authority is one immutable read-only lifecycle projection captured from a
// Writer. Its query methods perform no filesystem or contract-source I/O, so
// one request derives every status decision from the same sample.
type Authority struct {
	contract *schema.Contract
	policy   schema.ArtifactPolicy
	governed bool
	// released records that the process asserted a write face and then lost
	// it, which is a fault about this process rather than about the folder's
	// contract. It is a fact, not a sentence: no reader has asked yet.
	released bool
	claim    schema.Claim
}

// Authority captures the write face's current read-only authority. Flip does
// not use this snapshot: writes revalidate the source under the writer's lock.
func (w *Writer) Authority() Authority {
	if w == nil {
		return Authority{governed: true, claim: schema.Rejected(""), released: true}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.root == nil {
		return Authority{governed: true, claim: schema.Rejected(""), released: true}
	}
	governed := w.governance.Governed()
	if w.contract == nil {
		// A folder that never claimed authority says nothing; one whose
		// contract could not be read carries the sentence at the vault level.
		return Authority{governed: governed, claim: w.governance.Claim()}
	}
	policy := w.policy.Capture()
	if !policy.Available() {
		return Authority{governed: governed, claim: policy.Claim()}
	}
	return Authority{contract: w.contract, policy: policy, governed: governed, claim: w.governance.Claim()}
}

// Governed reports whether anything claimed authority over this vault. A false
// answer is not a failure: the folder simply has no lifecycle.
func (v Authority) Governed() bool {
	return v.governed
}

// Claim reports how far the lifecycle authority got, so a caller that closes a
// projection carries the same reason value the contract produced.
func (v Authority) Claim() schema.Claim {
	return v.claim
}

// Closed reports whether this captured view can classify governed instances.
func (v Authority) Closed() bool {
	return !v.available()
}

func (v Authority) available() bool {
	return v.contract != nil && v.policy.Available()
}

// Diagnostic explains, in lang, why this captured view is closed. It is empty
// both when lifecycle reads are available and when nothing ever claimed
// authority. A released write face is yomihon's own fault and is answered from
// the dictionary; everything else carries the contract's own sentence.
func (v Authority) Diagnostic(lang wording.Lang) string {
	if v.released {
		return wording.ContractUnavailable.In(lang)
	}
	return v.claim.Diagnostic()
}

// WriteDiagnostic explains, in lang, why this captured authority cannot offer
// status transitions. Contract and artifact-policy failures take precedence
// over a platform-only write limitation. Empty means a POST may be offered.
func (v Authority) WriteDiagnostic(lang wording.Lang) string {
	if !v.writeRefused() {
		return ""
	}
	if diagnostic := v.Diagnostic(lang); diagnostic != "" {
		return diagnostic
	}
	return wording.DurabilityUnsupported.In(lang)
}

// writeRefused reports whether anything stands between this authority and a
// status POST — the question WriteDiagnostic answers in words.
func (v Authority) writeRefused() bool {
	if v.released || v.claim.Diagnostic() != "" {
		return true
	}
	if v.Closed() {
		return false
	}
	return !durableInstallSupported
}

// ungoverned reports why the lifecycle does not reach the note at rel, or nil
// when it does. The two questions stay two: a path under a declared artifact
// directory is no note instance at all, while a note outside the directories
// scan.knowledge_dirs declares is an instance the contract never placed under
// its state machine. A contract declaring no knowledge layer draws no boundary
// and its scope then includes every path, which is what silence declares.
func ungoverned(policy schema.ArtifactPolicy, scope schema.KnowledgeScope, rel string) error {
	if policy.IsNonInstance(rel) {
		return ErrNonInstance
	}
	if !scope.Includes(rel) {
		return ErrOutsideKnowledgeScope
	}
	return nil
}

// WhyUngoverned reports the refusal Flip would return for relPath, so a page
// with an empty transition set can name the reason. A note the lifecycle
// reaches, a closed authority and a path it cannot name all answer nil.
func (v Authority) WhyUngoverned(relPath string) error {
	relPath, named := noteName(relPath)
	if !named || !v.available() {
		return nil
	}
	return ungoverned(v.policy, v.contract.KnowledgeScope(), relPath)
}

// noteName is the vault-relative name the write face would use for relPath,
// and whether it can name one at all: a path it cannot normalize is no note.
func noteName(relPath string) (string, bool) {
	rel, _, err := normalizeRelPath(relPath)
	if err != nil {
		return "", false
	}
	return rel, true
}

// Transitions returns the from-list-legal target statuses from current, in
// contract order. Owner lists are declarative data and never subtract from the
// answer. The published status is never among them: Flip would refuse it, and
// a note the lifecycle does not reach gets none by the same test Flip applies.
func (v Authority) Transitions(relPath, noteType, current string) []string {
	relPath, _, err := normalizeRelPath(relPath)
	if err != nil || v.Closed() || v.writeRefused() || noteType == "" || current == "" ||
		ungoverned(v.policy, v.contract.KnowledgeScope(), relPath) != nil {
		return nil
	}
	var legal []string
	for _, to := range v.contract.Statuses(noteType) {
		if to == schema.PublishedStatus {
			continue
		}
		if err := v.contract.Transition(noteType, current, to); err == nil {
			legal = append(legal, to)
		}
	}
	return legal
}

// CanReturn reports whether some chain of contract-legal transitions leads
// from "to" back to "from", which is what decides between a quiet single press
// and a two-step confirm. The walk never enters the published status, so no
// return exists only by way of it. A closed view claims nothing and is false.
func (v Authority) CanReturn(noteType, from, to string) bool {
	if !v.available() || from == "" || to == "" {
		return false
	}
	from = schema.NormalizeStatus(from)
	to = schema.NormalizeStatus(to)
	if from == to {
		return true
	}
	statuses := v.contract.Statuses(noteType)
	visited := map[string]bool{to: true}
	frontier := []string{to}
	for len(frontier) > 0 {
		current := frontier[0]
		frontier = frontier[1:]
		for _, next := range statuses {
			if next == schema.PublishedStatus || visited[next] {
				continue
			}
			if v.contract.Transition(noteType, current, next) != nil {
				continue
			}
			if next == from {
				return true
			}
			visited[next] = true
			frontier = append(frontier, next)
		}
	}
	return false
}

// LegalTransition reports whether the contract legalises moving a note of this
// type from one status to another. It answers a question about a transition
// that already happened and never authorises a write, which Flip settles.
func (v Authority) LegalTransition(noteType, from, to string) bool {
	if !v.available() || from == "" || to == "" {
		return false
	}
	return v.contract.Transition(noteType, from, to) == nil
}

// KnownStatus reports whether status is among the contract's declared values
// for the given note type. A closed view knows none; so does an undeclared type.
func (v Authority) KnownStatus(noteType, status string) bool {
	return v.available() && slices.Contains(v.contract.Statuses(noteType), schema.NormalizeStatus(status))
}

// DeclaresStatuses reports whether the contract gives this note type a status
// vocabulary at all. KnownStatus cannot carry that distinction — it answers
// false both for a value outside a list and for a type that has no list — and
// the two call for opposite sentences.
func (v Authority) DeclaresStatuses(noteType string) bool {
	return v.available() && len(v.contract.Statuses(noteType)) > 0
}

// Order returns the default note group's statuses in declared order. A nil
// result means the view is closed; an empty non-nil one is a valid declaration.
func (v Authority) Order() []string {
	if !v.available() {
		return nil
	}
	order := slices.Clone(v.contract.Statuses(""))
	if order == nil {
		return []string{}
	}
	return order
}

// VaultRoot is the absolute path of the vault this writer writes into. It is
// empty on a nil Writer and after Close.
func (w *Writer) VaultRoot() string {
	if w == nil {
		return ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.root == nil {
		return ""
	}
	return w.root.Name()
}

// ObservedStatus reports the status the note carries on disk right now. The
// reading page's other values come from a scan seconds old, which is right for
// a body or a link graph; an older status would offer a transition from a
// state the note has already left.
func (w *Writer) ObservedStatus(ctx context.Context, rel string) (string, error) {
	relSlash, osPath, err := normalizeRelPath(rel)
	if err != nil {
		return "", err
	}
	// The lock can be held by a flip across two synchronizations, so a reader
	// who navigated away is refused here rather than parked behind one.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.root == nil {
		return "", ErrClosed
	}
	source, err := readRegularFile(w.root, osPath, relSlash)
	if err != nil {
		return "", err
	}
	return vault.Parse(relSlash, source.data).Status(), nil
}

// Flip moves the note at rel from status "from" to status "to": it validates
// the transition against the contract, confirms the note's bytes still carry
// contentIdentity, rewrites exactly the frontmatter status line, and atomically
// installs the result under the note's own name. Every refusal before the
// install leaves the file untouched. ctx is honoured up to the lock and not
// after: past that point the install runs to completion or refuses.
func (w *Writer) Flip(ctx context.Context, rel, from, to string, contentIdentity [sha256.Size]byte) error {
	return w.flip(ctx, rel, from, to, contentIdentity, flipHooks{})
}

type flipHooks struct {
	beforeLock func()
	afterLock  func()
	// beforeAuthority runs after the replacement bytes are prepared and
	// before the install window's authority recheck.
	beforeAuthority func()
	// beforeInstall runs inside the install window: after the source has been
	// confirmed unmodified and before the rewritten bytes take the note's name.
	beforeInstall func()
	// afterInstall runs once the replacement is durably visible.
	afterInstall func()
}

func (w *Writer) flip(
	ctx context.Context,
	rel, from, to string,
	contentIdentity [sha256.Size]byte,
	hooks flipHooks,
) error {
	relSlash, rel, err := normalizeRelPath(rel)
	if err != nil {
		return err
	}
	if to == schema.PublishedStatus {
		return ErrPublishedReserved
	}
	if hooks.beforeLock != nil {
		hooks.beforeLock()
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if hooks.afterLock != nil {
		hooks.afterLock()
	}
	if w.root == nil {
		return ErrClosed
	}

	err = w.validateWriteTarget(rel, relSlash)
	if err != nil {
		return err
	}

	source, err := readRegularFile(w.root, rel, relSlash)
	if err != nil {
		return err
	}
	data := source.data

	n := vault.Parse(relSlash, data)
	if current := n.Status(); current != from {
		return fmt.Errorf("%w: status is %q, page said %q", ErrStale, current, from)
	}
	// The stale check compares one parsed value; this binds the ruling to every
	// other byte. Order matters: a moved status line names the actual repair.
	if vault.ContentIdentity(data) != contentIdentity {
		return fmt.Errorf("%w: %s", ErrContentChanged, relSlash)
	}

	if err = w.contract.Transition(n.Type(), from, to); err != nil {
		return fmt.Errorf("move %s from %s to %s: %w", relSlash, from, to, err)
	}

	rewritten, err := rewriteStatusChecked(relSlash, data, n.Status() != "", to)
	if err != nil {
		return err
	}
	if err := w.install(rel, relSlash, &source, rewritten, hooks); err != nil {
		return err
	}
	// Minted only after the durable install and still under the lock, so a
	// racing second flip cannot interleave and the later mint wins.
	w.vouchReceipt(relSlash, from)
	return nil
}

// vouchReceipt records that a completed flip left status "from" on relSlash,
// replacing any receipt not yet read. Expired receipts are swept here, so the
// map never outgrows the notes flipped in the last couple of minutes.
func (w *Writer) vouchReceipt(relSlash, from string) {
	w.receiptMu.Lock()
	defer w.receiptMu.Unlock()
	if w.receipts == nil {
		w.receipts = make(map[string]receipt)
	}
	for key, entry := range w.receipts {
		if time.Since(entry.at) > receiptTTL {
			delete(w.receipts, key)
		}
	}
	w.receipts[relSlash] = receipt{from: from, at: time.Now()}
}

// ConsumeReceipt reports whether this write face recently completed a flip
// that left status "from" at rel, and spends the receipt when it does: the
// first reading gets true, every later one false. A mismatched origin spends
// nothing, so a hand-typed address cannot burn the real redirect's receipt.
func (w *Writer) ConsumeReceipt(rel, from string) bool {
	if w == nil || from == "" {
		return false
	}
	relSlash, _, err := normalizeRelPath(rel)
	if err != nil {
		return false
	}
	w.receiptMu.Lock()
	defer w.receiptMu.Unlock()
	entry, ok := w.receipts[relSlash]
	if !ok {
		return false
	}
	if time.Since(entry.at) > receiptTTL {
		delete(w.receipts, relSlash)
		return false
	}
	if entry.from != from {
		return false
	}
	delete(w.receipts, relSlash)
	return true
}

// install crosses Flip's irreversible boundary: it revalidates the artifact
// authority inside the install window, atomically installs the rewritten
// bytes, and reports only once the durable replacement is visible.
func (w *Writer) install(
	rel, relSlash string,
	source *fileSnapshot,
	rewritten []byte,
	hooks flipHooks,
) error {
	err := replaceRegularFile(
		w.root,
		rel,
		relSlash,
		source,
		rewritten,
		w.log,
		installHooks{beforeAuthority: hooks.beforeAuthority, beforeInstall: hooks.beforeInstall},
		func() error {
			_, authorityErr := w.validatedArtifactPolicy()
			return authorityErr
		},
	)
	if err != nil {
		if errors.Is(err, ErrArtifactPolicyUnavailable) {
			return err
		}
		return fmt.Errorf("write %s: %w", relSlash, err)
	}
	if hooks.afterInstall != nil {
		hooks.afterInstall()
	}
	return nil
}

func (w *Writer) validateWriteTarget(rel, relSlash string) error {
	if !durableInstallSupported {
		return ErrDurabilityUnsupported
	}
	if w.contract == nil {
		return ErrClosed
	}
	policy, err := w.validatedArtifactPolicy()
	if err != nil {
		return err
	}
	// The reading scan defines a note as a Markdown file with no dot-prefixed
	// component. The write face applies the whole of that definition, so a
	// resource the reading face never shows cannot acquire a transition. It is
	// asked before the lifecycle's reach is, because whether a file is a note
	// at all comes before which folder the note sits in.
	if !vault.IsMarkdown(relSlash) || vaultfs.OutsideScan(relSlash) {
		return ErrNonInstance
	}
	if err := ungoverned(policy, w.contract.KnowledgeScope(), relSlash); err != nil {
		return err
	}
	return w.targetSpelledAsRequested(rel, relSlash)
}

// targetSpelledAsRequested answers a request whose spelling the directory does
// not hold as a missing note, whatever the filesystem would open. A
// case-insensitive volume opens "L06.MD" for "L06.md"; the vault holds no such
// note, and the answer is the one a case-sensitive volume gives.
func (w *Writer) targetSpelledAsRequested(rel, relSlash string) error {
	parent, _, name, err := openRegularParent(w.root, rel, relSlash)
	if err != nil {
		// Reading the note reports this failure in the operator's own terms.
		return nil
	}
	defer closeRoot(parent)
	if _, statErr := parent.Lstat(name); statErr != nil {
		// Nothing resolves here, which the read reports as a missing note.
		return nil
	}
	dir, err := parent.Open(".")
	if err != nil {
		return fmt.Errorf("confirm the name of %s: %w", relSlash, err)
	}
	names, err := dir.Readdirnames(-1)
	_ = dir.Close() //nolint:errcheck // directory-descriptor cleanup is best-effort
	if err != nil {
		return fmt.Errorf("confirm the name of %s: %w", relSlash, err)
	}
	if !slices.Contains(names, name) {
		return fmt.Errorf("%s: %w", relSlash, fs.ErrNotExist)
	}
	return nil
}

func (w *Writer) validatedArtifactPolicy() (schema.ArtifactPolicy, error) {
	policy := w.policy.ValidateSource()
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
		return fileSnapshot{}, fmt.Errorf("stat parent of %s: %w", relSlash, err)
	}
	before, err := parent.Lstat(name)
	if err != nil {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("stat %s: %w", relSlash, err)
	}
	if !before.Mode().IsRegular() {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("%w: %s", errNotRegular, relSlash)
	}
	file, err := parent.Open(name)
	if err != nil {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("read %s: %w", relSlash, err)
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close() //nolint:errcheck // the stat error is the actionable failure
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("read %s: stat open file: %w", relSlash, err)
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
		return fileSnapshot{}, fmt.Errorf("read %s: %w", relSlash, err)
	}
	if err = file.Close(); err != nil {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("read %s: close: %w", relSlash, err)
	}
	after, err := parent.Lstat(name)
	if err != nil {
		closeRoot(parent)
		return fileSnapshot{}, fmt.Errorf("stat %s after read: %w", relSlash, err)
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
		return nil, "", "", fmt.Errorf("duplicate vault root for %s: %w", relSlash, err)
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
			return nil, "", "", fmt.Errorf("stat directory of %s: %w", relSlash, statErr)
		}
		if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			closeRoot(current)
			return nil, "", "", fmt.Errorf("%w: %s", errPathNotRegular, relSlash)
		}
		next, openErr := current.OpenRoot(component)
		if openErr != nil {
			closeRoot(current)
			return nil, "", "", fmt.Errorf("open directory of %s: %w", relSlash, openErr)
		}
		opened, openStatErr := next.Stat(".")
		after, afterErr := current.Lstat(component)
		if openStatErr != nil {
			closeRoot(current)
			closeRoot(next)
			return nil, "", "", fmt.Errorf("stat open directory of %s: %w", relSlash, openStatErr)
		}
		if afterErr != nil {
			closeRoot(current)
			closeRoot(next)
			return nil, "", "", fmt.Errorf("restat directory of %s: %w", relSlash, afterErr)
		}
		if !after.IsDir() || after.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			closeRoot(current)
			closeRoot(next)
			return nil, "", "", fmt.Errorf("%w: directory of %s changed while opening", ErrConcurrentWrite, relSlash)
		}
		if closeErr := current.Close(); closeErr != nil {
			closeRoot(next)
			return nil, "", "", fmt.Errorf("close directory of %s: %w", relSlash, closeErr)
		}
		current = next
	}
	return current, parentPath, name, nil
}

func closeRoot(root *os.Root) {
	// A close failure cannot change a completed read or rename, so reporting it
	// would call a finished write unsuccessful.
	_ = root.Close() //nolint:errcheck // directory-descriptor cleanup is best-effort
}

// normalizeRelPath validates and normalizes the external slash path before any
// contract or filesystem decision. It returns both slash and OS forms.
func normalizeRelPath(rel string) (relSlash, osPath string, err error) {
	if rel == "" || strings.Contains(rel, `\`) || pathpkg.IsAbs(rel) || hasControlByte(rel) {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidPath, rel)
	}
	normalized := pathpkg.Clean(rel)
	osPath = filepath.FromSlash(normalized)
	if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") || !filepath.IsLocal(osPath) {
		return "", "", fmt.Errorf("%w: %q", ErrInvalidPath, rel)
	}
	return normalized, osPath, nil
}

// hasControlByte reports whether s carries a byte no note's name can carry. A
// zero byte ends a path as far as the operating system is concerned, and no
// note is named with a line ending or a tab either, so a request carrying one
// is malformed and is refused rather than quoted onward into errors and logs.
func hasControlByte(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

// rewriteStatusChecked computes the surgical rewrite and refuses, in the
// reader's terms, whenever the byte-level rewriter cannot honor what the
// reader read: the reader parses full YAML while the rewriter locates one
// column-zero "status:" line. The rewritten bytes are parsed again with the
// reader the product uses, so a reported success can never leave a note the
// reader cannot parse.
func rewriteStatusChecked(relSlash string, data []byte, readable bool, to string) ([]byte, error) {
	rewritten, err := rewriteStatusLine(data, to)
	if err != nil {
		if errors.Is(err, ErrStatusLine) && readable {
			return nil, fmt.Errorf("%w: %s", ErrStatusSyntaxUnsupported, relSlash)
		}
		return nil, fmt.Errorf("%w: %s", err, relSlash)
	}
	if reparsed := vault.Parse(relSlash, rewritten); reparsed.FMDiagnostic != "" || reparsed.Status() != to {
		return nil, fmt.Errorf("%w: %s: the rewritten frontmatter would not read back as %q", ErrStatusSyntaxUnsupported, relSlash, to)
	}
	return rewritten, nil
}

// rewriteStatusLine replaces the frontmatter status value inside data with to,
// leaving every other byte — the block's delimiters, the rest of the status
// line, the body — byte-identical. It never re-serializes YAML and never
// rewrites the whole line: that would delete the author's own words on it, and
// could not tell them from a value whose shape no replacement fits. Where
// vault.StatusValueSpan reports nothing, the note keeps its bytes.
func rewriteStatusLine(data []byte, to string) ([]byte, error) {
	start, end, ok := vault.StatusValueSpan(data)
	if !ok {
		return nil, ErrStatusLine
	}
	out := make([]byte, 0, len(data)-(end-start)+len(to))
	out = append(out, data[:start]...)
	out = append(out, to...)
	out = append(out, data[end:]...)
	return out, nil
}

// installHooks is a seam set: every field is a place a test can stand inside
// an install that no temporary directory could otherwise reach.
type installHooks struct {
	beforeAuthority func()
	afterAuthority  func()
	// beforeInstall runs inside the install window: after the source has been
	// confirmed unmodified and before the rewritten bytes take the note's name.
	beforeInstall func()
	syncTemp      func(*os.File) error
	syncParent    func(*os.Root) error
	// rung, when set, replaces the per-filesystem probe for this install,
	// whose answer is otherwise cached for the whole process.
	rung func() installRung
	// ops, when set, wraps the install's filesystem operations, reproducing a
	// driver or a race no temporary directory offers.
	ops func(installOps) installOps
}

// replaceRegularFile prepares the complete replacement beside the source, then
// reopens the source's named parent, validates write authority, and verifies
// the same regular-file identity, mode, mtime and bytes immediately before
// installing. The rung it ran on is named in every failure it can produce.
func replaceRegularFile(
	root *os.Root,
	rel, relSlash string,
	source *fileSnapshot,
	data []byte,
	log *slog.Logger,
	hooks installHooks,
	authorize func() error,
) error {
	preparedParent, err := openSameParent(root, rel, relSlash, source)
	if err != nil {
		return err
	}
	quarantineStaleTemps(preparedParent, relSlash, log)
	rung := selectRung(preparedParent, hooks)
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

	installParent, err := openSameParent(root, rel, relSlash, source)
	if err != nil {
		return err
	}
	if authorize != nil {
		if err = authorize(); err != nil {
			closeRoot(installParent)
			return err
		}
	}
	if hooks.afterAuthority != nil {
		hooks.afterAuthority()
	}
	if err = sourceUnmodified(installParent, relSlash, source); err != nil {
		closeRoot(installParent)
		return err
	}
	if hooks.beforeInstall != nil {
		hooks.beforeInstall()
	}
	// Once the install starts, the temporary name can hold the version another
	// program wrote, so only the install may remove it.
	removeTemp = false
	ops := installOpsFor(installParent, hooks)
	if err = installRewritten(ops, relSlash, tmpName, rung, source, data); err != nil {
		closeRoot(installParent)
		return err
	}
	syncParent := syncDirectory
	if hooks.syncParent != nil {
		syncParent = hooks.syncParent
	}
	if err = syncParent(installParent); err != nil {
		closeRoot(installParent)
		return fmt.Errorf("%w: sync containing directory: %w", ErrInstallUncertain, err)
	}
	closeRoot(installParent)
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
		return nil, fmt.Errorf("stat parent of %s: %w", relSlash, err)
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
		return nil, nil, fmt.Errorf("stat %s before write: %w", relSlash, err)
	}
	if !before.Mode().IsRegular() || !os.SameFile(before, source.file) {
		return nil, nil, fmt.Errorf("%w: %s changed identity while flipping", ErrConcurrentWrite, relSlash)
	}
	file, err := parent.Open(source.name)
	if err != nil {
		return nil, nil, fmt.Errorf("reopen %s before write: %w", relSlash, err)
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close() //nolint:errcheck // the stat error is the actionable failure
		return nil, nil, fmt.Errorf("stat open %s before write: %w", relSlash, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(opened, source.file) {
		_ = file.Close() //nolint:errcheck // the identity mismatch is the actionable failure
		return nil, nil, fmt.Errorf("%w: %s changed identity while reopening", ErrConcurrentWrite, relSlash)
	}
	current, err := io.ReadAll(file)
	if err != nil {
		_ = file.Close() //nolint:errcheck // the read error is the actionable failure
		return nil, nil, fmt.Errorf("reread %s before write: %w", relSlash, err)
	}
	if err = file.Close(); err != nil {
		return nil, nil, fmt.Errorf("close %s before write: %w", relSlash, err)
	}
	return current, opened, nil
}

// statusTempPrefix and statusTempSuffix bracket the names writeTemp creates
// beside the note, around exactly tempRandomLength base32 characters. The
// sweep matches that whole shape, so a name that merely starts alike is left.
const (
	statusTempPrefix = ".yomihon-status-"
	statusTempSuffix = ".tmp"
	tempRandomLength = 26
)

// statusOrphanPrefix and statusOrphanSuffix bracket the name a stale temp is
// moved to. The sweep never deletes: such a temp can hold the version another
// program wrote, and no name tells that from the note's own retired bytes.
const (
	statusOrphanPrefix = ".yomihon-orphaned-"
	statusOrphanSuffix = ".keep"
)

// staleTempAge is how old an abandoned temp must be before the write face
// moves it aside. Only process death inside the install window can strand one,
// and an hour keeps a concurrently running process's temp out of reach.
const staleTempAge = time.Hour

// quarantineStaleTemps moves aside the temp files a crashed flip abandoned in
// the note's directory: only a regular file in exactly writeTemp's shape and
// older than staleTempAge. Nothing is deleted, nothing is overwritten, and the
// flip about to run does not depend on any of it.
func quarantineStaleTemps(parent *os.Root, relSlash string, log *slog.Logger) {
	dir, err := parent.Open(".")
	if err != nil {
		return
	}
	names, err := dir.Readdirnames(-1)
	_ = dir.Close() //nolint:errcheck // directory-descriptor cleanup is best-effort
	if err != nil {
		return
	}
	for _, name := range names {
		middle, ok := writeTempMiddle(name)
		if !ok {
			continue
		}
		info, statErr := parent.Lstat(name)
		if statErr != nil || !info.Mode().IsRegular() || time.Since(info.ModTime()) < staleTempAge {
			continue
		}
		orphan := statusOrphanPrefix + middle + statusOrphanSuffix
		if _, exists := parent.Lstat(orphan); exists == nil {
			continue
		}
		if parent.Rename(name, orphan) != nil {
			continue
		}
		// Deleting it could destroy the only copy of a note caught mid-write,
		// and dot-prefixed it is invisible to the reader, so it is said once.
		if log != nil {
			log.Warn("a status write left a file behind and it has been set aside; remove it once you are satisfied the note is intact",
				"note", relSlash, "kept", orphan)
		}
	}
}

// writeTempMiddle reports the random middle of a name writeTemp created, and
// whether name has that exact shape at all.
func writeTempMiddle(name string) (string, bool) {
	middle, ok := strings.CutPrefix(name, statusTempPrefix)
	if !ok {
		return "", false
	}
	middle, ok = strings.CutSuffix(middle, statusTempSuffix)
	if !ok || len(middle) != tempRandomLength {
		return "", false
	}
	for _, r := range middle {
		if (r < 'A' || r > 'Z') && (r < '2' || r > '7') {
			return "", false
		}
	}
	return middle, true
}

// tempName is one fresh name for a file placed beside the note during an
// install.
func tempName() string {
	return statusTempPrefix + rand.Text() + statusTempSuffix
}

func writeTemp(parent *os.Root, data []byte, mode os.FileMode, syncFile func(*os.File) error) (string, error) {
	name := tempName()
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
