// Package status is the write face: the only package in this repo
// allowed to write vault files or run git. It flips a note's frontmatter
// `status` field — a surgical, single-line rewrite, never a YAML
// re-serialization — and commits the change with the vault's configured
// git identity, never one yomihon sets itself. Within this tool's
// single-user, local-trust model the commit records that the write face
// performed the transition; it does not authenticate who triggered it.
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
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

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
	// ErrWorkTreeUnreadable means the working tree could not be inspected at
	// all — most often a folder that is no git repository. Distinct from
	// ErrDirty: an uncommitted edit is something the operator can clear and
	// retry, while this refuses every transition until the folder itself
	// changes, because each accepted transition is recorded as a commit.
	ErrWorkTreeUnreadable = errors.New("status: working tree could not be read")
	// ErrDetachedHead means the vault's HEAD names no branch — the repo is
	// mid-rebase, mid-bisect, or checked out at a bare commit. git would
	// happily commit here, but the commit would land on no branch: as soon
	// as the operator returns to one, the note silently reverts and the
	// receipt survives nowhere but the reflog. The write face refuses and
	// leaves the file untouched instead of inheriting that default.
	ErrDetachedHead = errors.New("status: vault HEAD is detached from any branch")
	// ErrStatusLine means the frontmatter block does not contain exactly
	// one line beginning with "status:" — a schema violation yomihon does
	// not repair. yomihon only reports faults; fixing the file belongs to
	// a human editor.
	ErrStatusLine = errors.New("status: frontmatter does not have exactly one status line")
	// ErrStatusSyntaxUnsupported means the reader understands the note's
	// status, but the frontmatter writes that field in a form the surgical
	// single-line rewriter does not support — an explicit or quoted key, a
	// flow mapping, or a YAML anchor the rewrite would sever. The note is
	// refused unchanged: rewriting here would either be impossible or leave
	// frontmatter the reader can no longer parse, and yomihon reports the
	// fault for a human to edit rather than guessing.
	ErrStatusSyntaxUnsupported = errors.New("status: frontmatter writes status in a syntax the surgical rewriter does not support")
	// ErrDurabilityUnsupported means the running platform cannot prove that an
	// atomic rename's directory entry reached durable storage. The write face
	// refuses before reading or creating any vault path rather than changing a
	// note and presenting an unconfirmed publication as success.
	ErrDurabilityUnsupported = errors.New("status: durable publication is unsupported on this platform")
	// ErrPublicationStranded means another program edited the note inside the
	// publication window and the write face could not finish putting that edit
	// back under the note's own name. Both versions are on disk — one under the
	// note's name and one beside it, named in the error — nothing was removed,
	// and no commit was made. It is deliberately distinct from
	// ErrConcurrentWrite: that refusal leaves the note as the other program
	// wrote it, while this one cannot say which of the two the note carries.
	ErrPublicationStranded = errors.New("status: an edit raced the flip and both versions were left on disk")
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
	// ErrReceiptDiverged means the note was rewritten and a commit was
	// created, but reading the commit back shows it does not record exactly
	// the intended change: its tree touches other paths, its blob for the
	// note differs from the bytes just published, or its subject line was
	// replaced. Repo-local hooks and clean filters are the usual causes,
	// and an external writer racing the commit can produce the same shape.
	// Like ErrCommitFailed, nothing is rolled back — a rollback would be a
	// second write hiding what happened; the divergence is surfaced so the
	// operator can inspect the commit by hand.
	ErrReceiptDiverged = errors.New("status: note rewritten and committed, but the commit does not record exactly this change")
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
	root       *os.Root
	contract   *schema.Contract
	governance schema.Governance
	policy     schema.ArtifactPolicy
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
//
// governance is what the folder asserted about its own contract, which a nil
// contract cannot answer: a folder carrying no contract and a folder whose
// contract could not be read both arrive here with no contract, and only the
// second one is a fault. When contract is non-nil, governance must be
// contract.Governance().
func Open(source *vault.Reader, contract *schema.Contract, governance schema.Governance) (*Lifecycle, error) {
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
	return &Lifecycle{root: root, contract: contract, governance: governance, policy: policy}, nil
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
	contract *schema.Contract
	policy   schema.ArtifactPolicy
	governed bool
	claim    schema.Claim
}

// View captures the write face's current read-only authority. Flip does not use
// this snapshot: writes revalidate the source under the publication lock.
func (lc *Lifecycle) View() View {
	// A released capability is a fault whichever folder it was pinned to: the
	// process asserted a write face and then lost it.
	if lc == nil {
		return View{governed: true, claim: schema.Rejected(CoreUnavailableDiagnostic)}
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.root == nil {
		return View{governed: true, claim: schema.Rejected(CoreUnavailableDiagnostic)}
	}
	governed := lc.governance.Governed()
	if lc.contract == nil {
		// Either nothing ever claimed authority here, in which case the closed
		// write face is the ordinary shape of a folder and says nothing, or the
		// contract claimed it and could not be read, in which case the vault
		// level carries the one sentence.
		return View{governed: governed, claim: lc.governance.Claim()}
	}
	policy := lc.policy.Capture()
	if !policy.Available() {
		return View{governed: governed, claim: policy.Claim()}
	}
	return View{contract: lc.contract, policy: policy, governed: governed, claim: lc.governance.Claim()}
}

// Governed reports whether anything claimed authority over this vault. A false
// answer is not a failure: the folder has no lifecycle, so no status face
// belongs on any page it serves.
func (v View) Governed() bool {
	return v.governed
}

// Claim reports how far the lifecycle authority got, so a caller that closes a
// projection carries the same reason value the contract produced.
func (v View) Claim() schema.Claim {
	return v.claim
}

// Closed reports whether this captured view can classify governed instances.
func (v View) Closed() bool {
	return !v.available()
}

func (v View) available() bool {
	return v.contract != nil && v.policy.Available()
}

// Diagnostic explains why this captured view is closed. It is empty both when
// lifecycle reads are available and when nothing ever claimed authority here —
// a folder with no contract is not a folder in trouble.
func (v View) Diagnostic() string {
	return v.claim.Diagnostic()
}

// WriteDiagnostic explains why this captured authority cannot offer status
// transitions. Contract and artifact-policy failures also invalidate read-only
// instance projections and therefore take precedence over a platform-only
// write limitation. An empty result means a POST may be offered.
func (v View) WriteDiagnostic() string {
	if diagnostic := v.Diagnostic(); diagnostic != "" {
		return diagnostic
	}
	if v.Closed() {
		return ""
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
	if err != nil || v.Closed() || v.WriteDiagnostic() != "" || noteType == "" || current == "" ||
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

// WithheldByOwner reports whether the contract defines at least one onward
// transition from current for this note type while granting the operator none
// of them. It separates two states an empty transition list conflates: a
// schema that defines nothing onward, and a schema whose onward steps other
// owners hold — a schema author debugging a missing seal needs to be pointed
// at the owner list, not at a schema gap that does not exist.
func (v View) WithheldByOwner(noteType, current string) bool {
	if !v.available() || noteType == "" || current == "" {
		return false
	}
	withheld := false
	for _, to := range v.contract.Statuses(noteType) {
		err := v.contract.Transition(noteType, current, to, actor)
		switch {
		case err == nil:
			// The operator owns an onward step, so nothing is withheld.
			return false
		case errors.Is(err, schema.ErrOwnerForbidden):
			withheld = true
		}
	}
	return withheld
}

// Order returns the default note group's statuses in declared order. A nil
// result means this view is closed; an empty non-nil result is a valid empty
// declaration.
func (v View) Order() []string {
	if !v.available() {
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
	return v.available() && v.contract.AdvanceableBy(noteType, current, actor)
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

// Block is one operator-facing refusal, in the three parts a reader needs: the
// thing to do, why the transition stopped, and the step that clears it.
//
// The parts stay separate because they are read in different places. A control
// the working tree would refuse carries Headline as its accessible name, where
// three sentences would bury the answer; the paragraph beside it can afford all
// three.
type Block struct {
	Headline string
	Body     string
	NextStep string
}

// Blocked reports whether this value names a refusal at all.
func (b Block) Blocked() bool { return b.Body != "" }

// Label is the short form for an accessible name: the headline where one was
// written, and the body otherwise.
func (b Block) Label() string {
	if b.Headline != "" {
		return b.Headline
	}
	return b.Body
}

// Operator-facing reasons a transition would be refused, stated before the
// press rather than after it.
var (
	// DirtyBlock explains the refusal a note with uncommitted changes would
	// receive. Staging the file would carry the pre-existing edit into the
	// commit that records the transition, so the write face declines instead of
	// folding the two together.
	//
	// It names the operator's own next action first because that is what the
	// reader wants, and it puts committing before reverting deliberately:
	// yomihon never removes anyone's work, and a message that offers discarding
	// as the equal first option reads as an invitation to lose an edit.
	DirtyBlock = Block{
		Headline: "先處理這篇筆記的其他修改",
		Body:     "Yomihon 只會單獨記錄狀態變更。為避免把內容修改一起提交，這次不會變更狀態。",
		NextStep: "請先提交這篇筆記的修改；若確定不需要這些修改，才將它還原。完成後重新整理此頁。",
	}
	// GitBlock explains a refusal caused by the working tree being unreadable —
	// most often a folder that is not a git repository at all.
	GitBlock = Block{
		Body: "無法讀取這個資料夾的 git 狀態。狀態寫入需要可用的 git 版本庫，這個轉換會被拒絕。",
	}

	// ReadBlock is shown when the note's own status line could not be read for
	// this request. The page will not offer a transition from a value it could
	// not confirm: whatever blocked the read blocks the write too, so every
	// control derived from it would be refused on arrival.
	ReadBlock = Block{
		Body: "無法讀取這個筆記目前的狀態。狀態操作暫時關閉，重新載入頁面可以再試一次。",
	}
)

// WriteBlockReason reports why a transition on rel would be refused right now,
// or the zero Block when one could proceed. The transition controls are derived
// from the contract alone, which cannot see the working tree, so without this
// the reading page offers a control the write path then rejects. The answer is
// advisory: it is computed for one request and the write path revalidates
// under its own lock, so a stale zero costs a refusal, never a wrong write.
func (lc *Lifecycle) WriteBlockReason(ctx context.Context, rel string) (Block, error) {
	_, osPath, err := normalizeRelPath(rel)
	if err != nil {
		return Block{}, err
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.root == nil {
		return Block{}, ErrClosed
	}
	dirty, err := lc.dirty(ctx, osPath)
	if err != nil {
		return GitBlock, err
	}
	if dirty {
		return DirtyBlock, nil
	}
	return Block{}, nil
}

// Observed is what one read of a note's own file found. Status and ContentHash
// come from the same bytes deliberately: the page shows the one and the audit
// query is bound to the other, and a receipt paired with bytes the reader is not
// looking at is worse than no receipt.
type Observed struct {
	Status      string
	ContentHash [sha256.Size]byte
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
func (lc *Lifecycle) ObservedStatus(_ context.Context, rel string) (Observed, error) {
	relSlash, osPath, err := normalizeRelPath(rel)
	if err != nil {
		return Observed{}, err
	}
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.root == nil {
		return Observed{}, ErrClosed
	}
	source, err := readRegularFile(lc.root, osPath, relSlash)
	if err != nil {
		return Observed{}, err
	}
	return Observed{
		Status:      vault.Parse(relSlash, source.data).Status(),
		ContentHash: sha256.Sum256(source.data),
	}, nil
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
	beforeInstall func()
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

	err = lc.validateWriteTarget(rel, relSlash)
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

	if stateErr := lc.validateRepoState(ctx, rel, relSlash); stateErr != nil {
		return stateErr
	}

	rewritten, err := rewriteStatusChecked(relSlash, data, n.Status() != "", to)
	if err != nil {
		return err
	}
	if err := lc.publishStatus(rel, relSlash, &source, rewritten, hooks); err != nil {
		return err
	}
	if err := lc.commit(ctx, rel, relSlash, from, to, rewritten); err != nil {
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
		publishHooks{beforeAuthority: hooks.beforePublish, beforeInstall: hooks.beforeInstall},
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

func (lc *Lifecycle) validateWriteTarget(rel, relSlash string) error {
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
	// The reading scan defines a note as a file whose path ends in ".md" and
	// carries no dot-prefixed component: it does not descend into a hidden
	// directory and does not serve a hidden file. The write face applies the
	// whole of that definition before touching the file, so a resource
	// carrying note-shaped frontmatter cannot acquire a committed
	// note-lifecycle receipt for something the reading face never shows.
	if !strings.HasSuffix(relSlash, ".md") || vault.OutsideScan(relSlash) {
		return ErrNonInstance
	}
	return lc.targetSpelledAsRequested(rel, relSlash)
}

// targetSpelledAsRequested refuses a request the filesystem answers with an
// entry spelled differently from the one asked for.
//
// A vault on a case-insensitive filesystem opens "L06.MD" for a request naming
// "L06.md". The scan compares spellings exactly and reads the file on disk as
// a resource, and so does git, whose index holds the entry under the name it
// was added with — which is why this used to end as a rewritten resource and a
// commit that could not find its own path. The check asks the directory what
// it actually holds, so the refusal comes before anything is written. It also
// covers spellings that differ in ways beyond letter case, since it compares
// the resolved entry rather than reasoning about one kind of equivalence.
func (lc *Lifecycle) targetSpelledAsRequested(rel, relSlash string) error {
	parent, _, name, err := openRegularParent(lc.root, rel, relSlash)
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
		return fmt.Errorf("status: confirm the name of %s: %w", relSlash, err)
	}
	names, err := dir.Readdirnames(-1)
	_ = dir.Close() //nolint:errcheck // directory-descriptor cleanup is best-effort
	if err != nil {
		return fmt.Errorf("status: confirm the name of %s: %w", relSlash, err)
	}
	if !slices.Contains(names, name) {
		return ErrNonInstance
	}
	return nil
}

// validateRepoState refuses a flip the vault repository's own state makes
// unrecordable: an uncommitted edit on the target that a commit would fold in,
// or a detached HEAD on which the receipt commit would land outside every
// branch. Every refusal here happens before publication and leaves the file
// untouched.
func (lc *Lifecycle) validateRepoState(ctx context.Context, rel, relSlash string) error {
	dirty, err := lc.dirty(ctx, rel)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrWorkTreeUnreadable, relSlash, err)
	}
	if dirty {
		return fmt.Errorf("%w: %s", ErrDirty, relSlash)
	}
	detached, err := lc.detachedHead(ctx)
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrWorkTreeUnreadable, relSlash, err)
	}
	if detached {
		return fmt.Errorf("%w: %s", ErrDetachedHead, relSlash)
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
// file has ever been named with one; asking to seal such a path reached the
// filesystem and came back as an unrecognized failure, which the reader was
// shown as "yomihon could not do this" rather than as the malformed request it
// was. The rest of this range is refused for what it would do afterwards: this
// path is quoted into the subject line of the commit that records the seal, and
// a line ending inside it would split that one line into a subject and a body
// of the sender's choosing. The receipt is the whole point of writing through
// this face, so nothing that can forge its shape is allowed to reach it.
func hasControlByte(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

// rewriteStatusChecked computes the surgical rewrite and refuses, in the
// reader's terms, whenever the byte-level writer cannot honor what the
// reader read. The reader parses frontmatter as full YAML while the writer
// locates one column-zero "status:" line, and the two definitions disagree
// on legal syntax in both directions. When the reader understood a status
// (readable) but the writer finds no such line, the note is written in a
// key form the writer does not support — not a schema violation. When the
// writer succeeds, the rewritten bytes are parsed again with the same
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
	// beforeInstall runs inside the publication window: after the source has
	// been confirmed unmodified and before the rewritten bytes take the
	// note's name. It is the only seam that can place an external writer
	// exactly where the install has to survive one.
	beforeInstall func()
	syncTemp      func(*os.File) error
	syncParent    func(*os.Root) error
	// rung, when set, replaces the per-filesystem probe for this publication.
	// The probe's answer is cached for the whole process, so a test that needs
	// a particular rung says so for its own call instead of deciding for every
	// other publication on the same filesystem.
	rung func() installRung
	// ops, when set, wraps the publication's filesystem operations, which is
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
	hooks publishHooks,
	authorize func() error,
) error {
	preparedParent, err := openSameParent(root, rel, relSlash, source)
	if err != nil {
		return err
	}
	quarantineStaleTemps(preparedParent)
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
	if hooks.beforeInstall != nil {
		hooks.beforeInstall()
	}
	// The temporary entry stops being spare scratch here. Once the install
	// starts, that name can hold the version another program wrote, so only
	// the install — which reads the bytes before it decides — may remove it.
	removeTemp = false
	ops := rootOps(publishParent)
	if hooks.ops != nil {
		ops = hooks.ops(ops)
	}
	if err = installRewritten(ops, relSlash, tmpName, rung, source, data); err != nil {
		closeRoot(publishParent)
		return err
	}
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
// face moves it aside. Only process death inside the publication window can
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
func quarantineStaleTemps(parent *os.Root) {
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
		_ = parent.Rename(name, orphan) //nolint:errcheck // moving an abandoned temp aside is best-effort
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

// tempName is one fresh name for a file placed beside the note during a
// publication.
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

// detachedHead reports whether the vault's HEAD is attached to no branch.
// git answers through symbolic-ref: on a branch it prints the ref and exits
// zero; detached, it exits one and, under -q, prints nothing at all. Any
// other outcome — a fatal diagnostic, or the git child failing before it
// could exec — carries output, so a silent exit one is the only shape read
// as detached. The check only reads repository state; it never creates or
// moves a branch.
func (lc *Lifecycle) detachedHead(ctx context.Context) (bool, error) {
	out, err := runGit(ctx, lc.root, "symbolic-ref", "-q", "HEAD")
	if err == nil {
		return false, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 && len(bytes.TrimSpace(out)) == 0 {
		return true, nil
	}
	return false, err
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

// commit records the flip as one commit, authored with the vault's
// configured git identity, never one yomihon sets itself. Within this
// tool's single-user, local-trust model the commit records that the write
// face performed the transition; it does not authenticate who triggered
// it. relSlash is used in the commit
// message (a stable, slash-form path); rel is what's passed to git, which
// on this platform are the same string.
func (lc *Lifecycle) commit(ctx context.Context, rel, relSlash, from, to string, rewritten []byte) error {
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
	return lc.verifyReceipt(ctx, relSlash, msg, rewritten)
}

// verifyReceipt reads the commit just created back and confirms it records
// exactly the intended change: the tree touches the note alone, the
// committed blob is the one git stores for the bytes just published, and
// the subject line is the composed message. A zero exit from git commit
// cannot promise any of this — repo-local hooks may edit and restage files
// or replace the message inside the partial commit, and an external writer
// can race the two child processes — so success is reported only after the
// receipt itself has been read back. On any mismatch the commit is left in
// place, mirroring the no-rollback stance of ErrCommitFailed, and the
// divergence is surfaced to the operator. Refusals never reach this point:
// only a flip that already published its bytes has a receipt to verify.
func (lc *Lifecycle) verifyReceipt(ctx context.Context, relSlash, msg string, rewritten []byte) error {
	// --root covers the degenerate first-commit shape; -z keeps names raw
	// so non-ASCII paths are not C-quoted.
	names, err := runGit(ctx, lc.root, "diff-tree", "--no-commit-id", "--name-only", "-r", "-z", "--root", "HEAD")
	if err != nil {
		return fmt.Errorf("%w: %s: inspect committed paths: %w", ErrReceiptDiverged, relSlash, err)
	}
	if got := strings.Split(strings.TrimSuffix(string(names), "\x00"), "\x00"); len(got) != 1 || got[0] != relSlash {
		return fmt.Errorf("%w: %s: commit changed paths %q", ErrReceiptDiverged, relSlash, got)
	}
	committed, err := runGit(ctx, lc.root, "rev-parse", "HEAD:"+relSlash)
	if err != nil {
		return fmt.Errorf("%w: %s: read committed blob id: %w", ErrReceiptDiverged, relSlash, err)
	}
	// The published bytes are fed through the same check-in conversion a
	// hand-run git add applies at this path — the vault's own line-ending
	// and attribute configuration — so the comparison asks whether the
	// commit stores these bytes, not whether the vault is configured to
	// store them verbatim.
	intended, err := runGitInput(ctx, lc.root, rewritten, "hash-object", "--stdin", "--path", relSlash)
	if err != nil {
		return fmt.Errorf("%w: %s: compute intended blob id: %w", ErrReceiptDiverged, relSlash, err)
	}
	if !bytes.Equal(bytes.TrimSpace(committed), bytes.TrimSpace(intended)) {
		return fmt.Errorf("%w: %s: committed bytes differ from the published note", ErrReceiptDiverged, relSlash)
	}
	subject, err := runGit(ctx, lc.root, "log", "-1", "--format=%s", "HEAD")
	if err != nil {
		return fmt.Errorf("%w: %s: read commit subject: %w", ErrReceiptDiverged, relSlash, err)
	}
	if got := strings.TrimSuffix(string(subject), "\n"); got != msg {
		return fmt.Errorf("%w: %s: commit subject %q, want %q", ErrReceiptDiverged, relSlash, got, msg)
	}
	return nil
}
