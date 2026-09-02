// Package status is the write face: the only package in this repo allowed
// to write vault files. It flips a note's frontmatter `status` field — a
// surgical, single-line rewrite, never a YAML re-serialization — and leaves
// every other byte of the file, and everything else on the machine, exactly
// as it found it. Within this tool's single-user, local-trust model nothing
// authenticates who triggered a flip; the file's new bytes are the whole
// record.
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
	// ErrClosed means the write face is fail-closed: either the vault contract
	// failed to load or its pinned root capability has been closed. Fault
	// tolerance is asymmetric by direction — reading tolerates a missing
	// contract, but a write without one could destroy a file, so writing refuses.
	ErrClosed = errors.New("write face is closed")
	// ErrArtifactPolicyUnavailable means the core contract loaded but its
	// artifact policy is unavailable, so instance writes cannot be classified.
	ErrArtifactPolicyUnavailable = errors.New("artifact policy unavailable")
	// ErrNonInstance means the requested path is a readable artifact rather
	// than a governed note instance.
	ErrNonInstance = errors.New("target is not a governable artifact")
	// ErrInvalidPath means a status request did not name a local vault-relative
	// slash path.
	ErrInvalidPath = errors.New("invalid vault-relative path")
	// ErrStale means the submitted form's "from" no longer matches the
	// note's on-disk status: the page was loaded before someone else
	// changed the file.
	ErrStale = errors.New("note is stale, reload and try again")
	// ErrConcurrentWrite means the named regular file, its parent identity,
	// mode, mtime, or exact bytes changed between Flip's descriptor read and
	// its pre-write recheck. An external tool such as Obsidian raced the flip
	// itself. Distinct from ErrStale: the two cases carry different user-facing
	// presentations and this sentinel must not satisfy errors.Is(err, ErrStale).
	ErrConcurrentWrite = errors.New("note changed while flipping")
	// ErrContentChanged means the note's bytes on disk no longer carry the
	// content identity the caller read: something outside the status line was
	// edited between the page render and the flip, so the ruling was made
	// against content the disk no longer holds. The status line itself is
	// bound through "from" and its divergence reports as ErrStale; this
	// sentinel covers every other byte, which the stale check cannot see.
	ErrContentChanged = errors.New("note content changed after it was read")
	// ErrStatusLine means the frontmatter block does not contain exactly
	// one line beginning with "status:" — a schema violation yomihon does
	// not repair. yomihon only reports faults; fixing the file belongs to
	// a human editor.
	ErrStatusLine = errors.New("frontmatter does not have exactly one status line")
	// ErrPublishedReserved means the flip named published as its target.
	// That status records a completed publication — a receipt-backed fact
	// about the world outside the vault — and nothing in yomihon can attest
	// one, so the write face refuses before touching the note. The value
	// enters a note only by hand.
	ErrPublishedReserved = errors.New("published records a completed publication and no publisher exists to attest one")
	// ErrStatusSyntaxUnsupported means the reader understands the note's
	// status, but the frontmatter writes that field in a form the surgical
	// single-line rewriter does not support — an explicit or quoted key, a
	// flow mapping, or a YAML anchor the rewrite would sever. The note is
	// refused unchanged: rewriting here would either be impossible or leave
	// frontmatter the reader can no longer parse, and yomihon reports the
	// fault for a human to edit rather than guessing.
	ErrStatusSyntaxUnsupported = errors.New("frontmatter writes status in a syntax the surgical rewriter does not support")
	// ErrDurabilityUnsupported means the running platform cannot prove that an
	// atomic rename's directory entry reached durable storage. The write face
	// refuses before reading or creating any vault path rather than changing a
	// note whose new bytes a crash could silently discard.
	ErrDurabilityUnsupported = errors.New("durable install is unsupported on this platform")
	// ErrInstallStranded means another program edited the note inside the
	// install window and the write face could not finish putting that edit
	// back under the note's own name. Both versions are on disk — one under the
	// note's name and one beside it, named in the error — and nothing was
	// removed. It is deliberately distinct from ErrConcurrentWrite: that
	// refusal leaves the note as the other program wrote it, while this one
	// cannot say which of the two the note carries.
	ErrInstallStranded = errors.New("an edit raced the flip and both versions were left on disk")
	// ErrInstallUncertain means the atomic replacement completed, but the
	// containing directory could not be synchronized. The new bytes are visible
	// now, but their survival across an immediate crash was not confirmed.
	ErrInstallUncertain = errors.New("note rewritten but durability was not confirmed")
)

// The reading page's stable explanations for a closed write face. They read
// from the dictionary so each sentence has one source, and they resolve to the
// default language because they reach the page through a view that carries no
// request and therefore no choice — the surface that would carry one is a
// separate change.
var (
	CoreUnavailableDiagnostic           = wording.ContractUnavailable.In(wording.ZhHant)
	DurableInstallUnavailableDiagnostic = wording.DurabilityUnsupported.In(wording.ZhHant)
	NoteUnreadableDiagnostic            = wording.NoteStatusUnreadable.In(wording.ZhHant)
)

// The write face rewrites exactly the regular file a vault-relative path
// names, so a symbolic link — whose target can sit outside the vault — and
// every other special entry is refused, at the leaf and at every directory
// on the way. Two sentinels because the operator repairs them in the same
// way but the error should name which part of the path was not plain.
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
// Constructed once per process with the loaded vault contract (or nil,
// meaning fail-closed).
type Writer struct {
	root       *os.Root
	contract   *schema.Contract
	governance schema.Governance
	policy     schema.ArtifactPolicy
	// log carries the one thing this package has to say that no caller asked
	// for: that a flip killed mid-write left a file behind, and the sweep has
	// set it aside rather than deleting it. Nothing else here reports, because
	// everything else has a caller waiting for an answer.
	log *slog.Logger
	// mu serializes View, Flip, ObservedStatus, and Close: the pinned root
	// capability is a shared resource, two flips must never interleave their
	// read-check-replace windows, and Close cannot release the root under an
	// operation. One lock for the whole writer is deliberately simpler than
	// per-file locking: this is a local, single-operator tool where correctness
	// matters far more than throughput.
	//
	// The wait is therefore unbounded in principle — a flip holds it across
	// two synchronizations — so Flip and ObservedStatus consult the request's
	// context before they queue for it. Nothing has been touched at that
	// point, so a reader who has gone away is answered rather than parked.
	// View takes the same lock and has no context to consult: it is reached
	// through provider closures the composition point builds before any
	// request exists, and honouring one there means every closure carrying a
	// context through to here.
	mu sync.Mutex
	// receipts is the write face's short-lived memory of flips it performed
	// whose confirmation no reading page has shown yet, keyed by the note's
	// normalized vault-relative path. Only the write face can attest that a
	// transition actually ran, so the attestation lives here rather than in
	// any address a hand can type. At most one receipt per note: a second
	// flip before the first receipt is read replaces it, and the page then
	// states the change the note most recently made.
	receipts map[string]receipt
	// receiptMu guards receipts on its own, so a reading page consuming a
	// receipt never waits behind a flip holding mu across a full write. It
	// is always taken alone or inside mu, never the other way around.
	receiptMu sync.Mutex
}

// receipt is one unshown attestation: which status a completed flip left,
// and when, so a receipt that was never read does not vouch forever.
type receipt struct {
	from string
	at   time.Time
}

// receiptTTL bounds how long a completed flip vouches for its receipt. A
// browser follows the redirect within moments; minutes of slack cover a
// throttled or backgrounded tab without leaving a stale attestation around
// for the rest of the process's life.
const receiptTTL = 2 * time.Minute

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
//
// governance is what the folder asserted about its own contract, which a nil
// contract cannot answer: a folder carrying no contract and a folder whose
// contract could not be read both arrive here with no contract, and only the
// second one is a fault. When contract is non-nil, governance must be
// contract.Governance().
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
// Writer. Its query methods perform no filesystem or contract-source I/O,
// so one request can derive every status decision from the same authority
// sample. A later request captures another Authority and observes a latched source
// change.
type Authority struct {
	contract *schema.Contract
	policy   schema.ArtifactPolicy
	governed bool
	claim    schema.Claim
}

// Authority captures the write face's current read-only authority. Flip does not use
// this snapshot: writes revalidate the source under the writer's lock.
func (w *Writer) Authority() Authority {
	// A released capability is a fault whichever folder it was pinned to: the
	// process asserted a write face and then lost it.
	if w == nil {
		return Authority{governed: true, claim: schema.Rejected(CoreUnavailableDiagnostic)}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.root == nil {
		return Authority{governed: true, claim: schema.Rejected(CoreUnavailableDiagnostic)}
	}
	governed := w.governance.Governed()
	if w.contract == nil {
		// Either nothing ever claimed authority here, in which case the closed
		// write face is the ordinary shape of a folder and says nothing, or the
		// contract claimed it and could not be read, in which case the vault
		// level carries the one sentence.
		return Authority{governed: governed, claim: w.governance.Claim()}
	}
	policy := w.policy.Capture()
	if !policy.Available() {
		return Authority{governed: governed, claim: policy.Claim()}
	}
	return Authority{contract: w.contract, policy: policy, governed: governed, claim: w.governance.Claim()}
}

// Governed reports whether anything claimed authority over this vault. A false
// answer is not a failure: the folder has no lifecycle, so no status face
// belongs on any page it serves.
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

// Diagnostic explains why this captured view is closed. It is empty both when
// lifecycle reads are available and when nothing ever claimed authority here —
// a folder with no contract is not a folder in trouble.
func (v Authority) Diagnostic() string {
	return v.claim.Diagnostic()
}

// WriteDiagnostic explains why this captured authority cannot offer status
// transitions. Contract and artifact-policy failures also invalidate read-only
// instance projections and therefore take precedence over a platform-only
// write limitation. An empty result means a POST may be offered.
func (v Authority) WriteDiagnostic() string {
	if diagnostic := v.Diagnostic(); diagnostic != "" {
		return diagnostic
	}
	if v.Closed() {
		return ""
	}
	if !durableInstallSupported {
		return DurableInstallUnavailableDiagnostic
	}
	return ""
}

// Transitions returns the from-list-legal target statuses from current in
// contract order. The lifecycle rows' owner lists are declarative data and
// never subtract from the answer. The published status is never among the
// results: it records a completed publication, which no interactive control
// can attest, so Flip would refuse it. It is pure over the captured view.
func (v Authority) Transitions(relPath, noteType, current string) []string {
	relPath, _, err := normalizeRelPath(relPath)
	if err != nil || v.Closed() || v.WriteDiagnostic() != "" || noteType == "" || current == "" ||
		v.policy.IsNonInstance(relPath) {
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

// CanReturn reports whether, after a note of this type moves from one status
// to another, the write face could still walk it back: whether some chain of
// contract-legal transitions leads from "to" back to "from". It is derived
// from the same transition rules Transitions reads, never configured on its
// own, and it is what decides between a quiet single press and a two-step
// confirm — the press worth confirming is the one the reader cannot undo
// from where it lands, not the one whose destination happens to offer
// nothing onward.
//
// The walk never passes through the published status: it records a completed
// publication no interactive control can attest, so no offered chain can
// enter it, and a return that would exist only by way of it does not exist
// here. Both spellings of a status name the same stop, matching how every
// other verdict over the captured contract reads them. A closed view claims
// nothing and reports false.
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
// type from one status to another. It answers the reading page's question
// about a transition that has already happened — whether the move a redirect
// claims to have made is one this contract admits — and never authorises a
// write, which Flip settles for itself against the note's own bytes.
func (v Authority) LegalTransition(noteType, from, to string) bool {
	if !v.available() || from == "" || to == "" {
		return false
	}
	return v.contract.Transition(noteType, from, to) == nil
}

// KnownStatus reports whether status is among the contract's declared values
// for the given note type. A closed view knows no values, and an undeclared
// type declares none. The reading page uses it to flag a status the schema
// never listed, instead of implying the schema defines no onward transitions
// from a value it never defined.
func (v Authority) KnownStatus(noteType, status string) bool {
	return v.available() && slices.Contains(v.contract.Statuses(noteType), schema.NormalizeStatus(status))
}

// DeclaresStatuses reports whether the contract gives this note type a status
// vocabulary at all, which is what has to be true before any answer about a
// particular status means anything.
//
// KnownStatus cannot carry that distinction: it answers false both for a value
// outside a list and for a type that has no list, and those call for opposite
// sentences. A note whose type the schema never declared has a status that is
// legal under every type it did declare, so telling its author the value is
// outside the list points them at the one field that is fine — while the field
// actually at fault is the type, which the page names separately.
func (v Authority) DeclaresStatuses(noteType string) bool {
	return v.available() && len(v.contract.Statuses(noteType)) > 0
}

// Order returns the default note group's statuses in declared order. A nil
// result means this view is closed; an empty non-nil result is a valid empty
// declaration.
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

// VaultRoot reports the absolute path of the vault this writer writes
// into: the resolved directory its capability was pinned to at Open. It is
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

// ObservedStatus reports the status the note carries on disk right now.
//
// The reading page takes everything else from a scan that is up to a couple of
// seconds old, which is right for a body, a title and a link graph: those are
// projections over the whole folder and they have to agree with each other. A
// status is not one of those. It is a statement about one file, the write path
// re-reads that same file under its own lock before allowing anything, and a
// page that shows an older value offers a transition from a state the note has
// already left — most visibly right after a write, when the reader is looking
// straight at the thing they just did.
func (w *Writer) ObservedStatus(ctx context.Context, rel string) (string, error) {
	relSlash, osPath, err := normalizeRelPath(rel)
	if err != nil {
		return "", err
	}
	// The lock this read is about to take can be held by a flip across two
	// synchronizations, so a reader who navigated away is refused here rather
	// than parked behind one. Nothing has been touched at this point, so
	// refusing costs the caller only the answer it no longer wants.
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
// contentIdentity — the vault.ContentIdentity of the source the caller read
// before ruling — rewrites exactly the frontmatter status line, and
// atomically installs the rewritten bytes under the note's own name. Every
// refusal before the install leaves the file untouched.
//
// Flip holds the Writer's lock for its entire duration (see the mu field
// doc): concurrent callers are serialized, never interleaved.
//
// ctx is honoured up to the moment the lock is taken and not after. A caller
// who has gone away before then is refused with the file untouched, which is
// the whole of what cancellation can safely mean here: past that point the
// note is being replaced under its own name, and a write abandoned partway is
// the one outcome this package exists to make impossible. The install runs to
// completion, or refuses and leaves the file as it found it.
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
	// The stale check above compares one parsed value; this one binds the
	// ruling to every other byte the caller read. The order matters: when the
	// status line itself moved on, the stale page names the actual repair.
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
	// The durable install is the fact the receipt attests, so the receipt is
	// minted only after it, still under the writer's lock: a racing second
	// flip of the same note cannot interleave, and the later mint wins.
	w.vouchReceipt(relSlash, from)
	return nil
}

// vouchReceipt records that a completed flip left status "from" on the note
// at relSlash, replacing any receipt not yet read. Receipts other notes never
// collected are swept once they expire, so the map never outgrows the handful
// of notes flipped in the last couple of minutes.
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
// that left status "from" on the note at rel, and spends the receipt when it
// does: the first reading that asks gets true, every later one false. A
// mismatched origin spends nothing, so a hand-typed address cannot burn the
// receipt the real redirect is about to collect; an expired receipt answers
// false and is dropped. The reading page keeps its own contract checks — this
// method only attests that the write happened, not that repeating it is a
// sentence the page can stand behind.
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

// install crosses Flip's irreversible boundary. It revalidates the current
// artifact authority inside replaceRegularFile's install window, atomically
// installs the rewritten bytes, and reports only after the durable
// replacement is visible.
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
	if policy.IsNonInstance(relSlash) {
		return ErrNonInstance
	}
	// The reading scan defines a note as a file whose path ends in ".md" and
	// carries no dot-prefixed component: it does not descend into a hidden
	// directory and does not serve a hidden file. The write face applies the
	// whole of that definition before touching the file, so a resource
	// carrying note-shaped frontmatter cannot acquire a note-lifecycle
	// transition for something the reading face never shows.
	if !vault.IsMarkdown(relSlash) || vaultfs.OutsideScan(relSlash) {
		return ErrNonInstance
	}
	return w.targetSpelledAsRequested(rel, relSlash)
}

// targetSpelledAsRequested answers a request whose spelling the directory
// does not hold as a missing note, whatever the filesystem would open for it.
// A case-insensitive volume opens "L06.MD" for "L06.md"; the vault holds no
// such note, and the answer is the same one a case-sensitive volume gives.
func (w *Writer) targetSpelledAsRequested(rel, relSlash string) error {
	parent, _, name, err := openRegularParent(w.root, rel, relSlash)
	if err != nil {
		// The walk to the note's directory failed, and reading the note
		// reports that failure in the operator's own terms a moment later.
		return nil
	}
	defer closeRoot(parent)
	if _, statErr := parent.Lstat(name); statErr != nil {
		// The name resolves to nothing at all, which is a missing note rather
		// than a resource, and the read says so.
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
	// Root holds only a directory descriptor. A close failure cannot change a
	// completed read or rename, so reporting it as a Flip failure would call a
	// finished write unsuccessful.
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

// hasControlByte reports whether s carries a byte no note's name can carry.
//
// A zero byte ends a path as far as the operating system is concerned, so no
// file has ever been named with one; asking to flip such a path reached the
// filesystem and came back as an unrecognized failure, which the reader was
// shown as "yomihon could not do this" rather than as the malformed request it
// was. The rest of the range — line endings, tabs, escapes — has the same
// character: no note is named with one, so a request carrying one is
// malformed, and it is refused up front rather than quoted onward into
// errors and logs.
func hasControlByte(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

// rewriteStatusChecked computes the surgical rewrite and refuses, in the
// reader's terms, whenever the byte-level rewriter cannot honor what the
// reader read. The reader parses frontmatter as full YAML while the rewriter
// locates one column-zero "status:" line, and the two definitions disagree
// on legal syntax in both directions. When the reader understood a status
// (readable) but the rewriter finds no such line, the note is written in a
// key form the rewriter does not support — not a schema violation. When the
// rewriter succeeds, the rewritten bytes are parsed again with the same
// reader the product uses: a result that no longer parses (a severed YAML
// anchor leaves a dangling alias) or does not read back as the target
// status is refused before anything is written, so a reported success can
// never leave a note the reader cannot parse.
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

// rewriteStatusLine replaces the frontmatter status value inside data with
// to, leaving every other byte — the block's own delimiters, the rest of the
// status line, the body — byte-identical to the original. It never
// re-serializes YAML. The bytes it replaces are the span
// vault.StatusValueSpan reports, the same bytes the content identity excises,
// so a write can never move something the identity still covers.
//
// Writing the whole line instead would deletes the author's own words on it:
// a reason recorded in a trailing comment, the quoting they chose. Worse, it
// could not tell those apart from a value whose shape no replacement fits —
// a sequence, an anchor, a scalar continuing onto the next line — and rewrote
// those into something never written, because the result still read back as
// the target status. Where the span reports nothing, the note keeps its bytes.
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

// installHooks is a seam set, and nothing else: every field is a place a test
// can stand inside an install that no temporary directory could otherwise
// reach. Production sets exactly the two that carry a flip's own hooks, and
// leaves the rest nil.
type installHooks struct {
	beforeAuthority func()
	afterAuthority  func()
	// beforeInstall runs inside the install window: after the source has
	// been confirmed unmodified and before the rewritten bytes take the
	// note's name. It is the only seam that can place an external writer
	// exactly where the install has to survive one.
	beforeInstall func()
	syncTemp      func(*os.File) error
	syncParent    func(*os.Root) error
	// rung, when set, replaces the per-filesystem probe for this install.
	// The probe's answer is cached for the whole process, so a test that needs
	// a particular rung says so for its own call instead of deciding for every
	// other install on the same filesystem.
	rung func() installRung
	// ops, when set, wraps the install's filesystem operations, which is
	// how a test reproduces a driver or a race no temporary directory offers.
	ops func(installOps) installOps
}

// replaceRegularFile prepares the complete replacement beside the source,
// then reopens the source's named parent, validates caller-supplied write
// authority, and finally verifies the same regular-file identity, mode, mtime,
// and bytes immediately before installing the replacement under the note's own
// name. The rung the install ran on — which is what any guarantee about a
// racing external edit rests on — is named in every failure it can produce.
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
	// The temporary entry stops being spare scratch here. Once the install
	// starts, that name can hold the version another program wrote, so only
	// the install — which reads the bytes before it decides — may remove it.
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
// beside the note, around exactly tempRandomLength characters of the base32
// alphabet crypto/rand's Text produces. The sweep recognizes an abandoned
// temp by that whole shape, so the pieces stay defined in one place and a
// name that merely starts the same way is left alone.
const (
	statusTempPrefix = ".yomihon-status-"
	statusTempSuffix = ".tmp"
	tempRandomLength = 26
)

// statusOrphanPrefix and statusOrphanSuffix bracket the name a stale temp is
// moved to. The sweep never deletes: a temp a dead flip left behind can hold
// the version another program wrote, displaced there by the atomic exchange
// and never put back because the process died first, and there is no way to
// tell that from the note's own retired bytes by looking at the name. Moving
// it out of the temp shape leaves it where a person can compare it against
// the note, and keeps it out of every later sweep.
const (
	statusOrphanPrefix = ".yomihon-orphaned-"
	statusOrphanSuffix = ".keep"
)

// staleTempAge is how old an abandoned temp file must be before the write
// face moves it aside. Only process death inside the install window can
// strand one — every in-process failure clears its own temp or names it in
// the error — and a stranded file is invisible to the reading scan, which
// hides dot-prefixed names, so nothing else ever reclaims it. An hour is far
// beyond any flip's lifetime and keeps a temp belonging to a concurrently
// running process out of reach.
const staleTempAge = time.Hour

// quarantineStaleTemps moves aside the temp files a crashed flip abandoned in
// the note's directory. The sweep is deliberately narrow: only a regular file
// named in exactly the shape writeTemp creates, older than staleTempAge, is
// touched; directories, symbolic links, young files, and every other name are
// left where they are. Nothing is deleted, and an entry already sitting under
// the destination name is left alone rather than overwritten. Best-effort
// throughout — the flip about to run does not depend on it.
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
		// The file stays because deleting it could destroy the only copy of a
		// note a crash caught mid-write, and it is dot-prefixed, so nothing
		// the reader ever opens will show it. Saying so once is what keeps it
		// from being kept forever and known to nobody: it is the operator's to
		// read and to remove.
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
