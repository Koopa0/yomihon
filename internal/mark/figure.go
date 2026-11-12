package mark

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/vault"
)

// ErrUnreadableApprovals means the figure approvals file exists and cannot be
// read safely. Nothing it holds is honoured — every figure runs without script
// until the file is readable again — and nothing replaces it, so the person
// who owns it can see what went wrong.
var ErrUnreadableApprovals = errors.New("the figure approvals cannot be read")

// approvalsVersion is the only shape of the approvals file this reads back.
const approvalsVersion = 1

// FigureApproval is the owner's word that one figure's exact bytes may run
// their inline scripts in the reader's browser. It names the bytes by their
// digest, so any change to the file, by anyone, leaves it naming bytes that
// are no longer there.
type FigureApproval struct {
	RelPath string    `json:"path"`
	SHA256  string    `json:"sha256"`
	At      time.Time `json:"time"`
}

// approvalsDocument is the file's own shape. Version comes first so a reader
// that does not recognise it stops at the first field.
type approvalsDocument struct {
	Version   int              `json:"version"`
	Vault     string           `json:"vault"`
	Approvals []FigureApproval `json:"approvals"`
}

// ApprovalsPath names the third file in this vault's directory. It is separate
// from the reading marks: a mark is set by a press in the page, and an
// approval only by a person at a terminal, and neither file's damage should
// cost the other.
func (f *File) ApprovalsPath() string {
	return filepath.Join(f.dir, "figures.json")
}

// FigureApproved reports whether the figure at relPath with digest sum is
// approved, and, when it is not, whether relPath was approved for other bytes:
// an approval that lapsed when the file changed. No file is no approval. A
// file that cannot be read, or carries a shape this version does not write, is
// an error: the caller treats it as no approval and says so, rather than
// reading damage as an empty list that a later approval would quietly replace.
//
// Each call reads the file again, so a revocation takes effect at the next
// page and the next frame request, without a restart.
func (f *File) FigureApproved(relPath string, sum [sha256.Size]byte) (approved, lapsed bool, err error) {
	approvals, err := f.FigureApprovals()
	if err != nil {
		return false, false, err
	}
	digest := hex.EncodeToString(sum[:])
	for _, approval := range approvals {
		if approval.RelPath != relPath {
			continue
		}
		if approval.SHA256 == digest {
			return true, false, nil
		}
		lapsed = true
	}
	return false, lapsed, nil
}

// FigureApprovals returns every approval this vault holds, in file order.
func (f *File) FigureApprovals() ([]FigureApproval, error) {
	data, err := os.ReadFile(f.ApprovalsPath())
	if errors.Is(err, os.ErrNotExist) {
		return []FigureApproval{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadableApprovals, err)
	}
	var doc approvalsDocument
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadableApprovals, err)
	}
	if !json.Valid(data) || doc.Version != approvalsVersion || doc.Approvals == nil {
		return nil, fmt.Errorf("%w: not a version %d approvals file", ErrUnreadableApprovals, approvalsVersion)
	}
	seen := make(map[string]bool, len(doc.Approvals))
	for i := range doc.Approvals {
		if err = validateApproval(&doc.Approvals[i]); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrUnreadableApprovals, err.Error())
		}
		if seen[doc.Approvals[i].RelPath] {
			return nil, fmt.Errorf("%w: %s is approved twice", ErrUnreadableApprovals, doc.Approvals[i].RelPath)
		}
		seen[doc.Approvals[i].RelPath] = true
	}
	return doc.Approvals, nil
}

// ApproveFigure records that the figure at relPath may run when its bytes
// have digest sum, replacing any earlier approval of that path: one path is
// approved for one set of bytes at a time. An unreadable file is left as it is
// and the approval is refused.
func (f *File) ApproveFigure(relPath string, sum [sha256.Size]byte, at time.Time) error {
	approval := FigureApproval{RelPath: relPath, SHA256: hex.EncodeToString(sum[:]), At: at.UTC()}
	if err := validateApproval(&approval); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	held, err := f.FigureApprovals()
	if err != nil {
		return err
	}
	held = withoutPath(held, relPath)
	return f.writeApprovals(append(held, approval))
}

// RevokeFigure removes the approval of relPath, whatever bytes it named, and
// reports whether there was one. With none, nothing is written.
func (f *File) RevokeFigure(relPath string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	held, err := f.FigureApprovals()
	if err != nil {
		return false, err
	}
	kept := withoutPath(held, relPath)
	if len(kept) == len(held) {
		return false, nil
	}
	return true, f.writeApprovals(kept)
}

func withoutPath(held []FigureApproval, relPath string) []FigureApproval {
	kept := make([]FigureApproval, 0, len(held))
	for _, approval := range held {
		if approval.RelPath != relPath {
			kept = append(kept, approval)
		}
	}
	return kept
}

// writeApprovals installs the whole list through a sibling temporary file and
// one rename, owner-only like every file here. It is not synchronized to
// durable storage: an approval lost to a power cut is given again at the
// terminal, and is lost in the safe direction.
func (f *File) writeApprovals(approvals []FigureApproval) error {
	data, err := json.MarshalIndent(approvalsDocument{
		Version:   approvalsVersion,
		Vault:     f.vaultRoot,
		Approvals: approvals,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the figure approvals: %w", err)
	}
	if err = os.MkdirAll(f.dir, 0o700); err != nil {
		return fmt.Errorf("create the marks directory: %w", err)
	}
	tmp, err := os.CreateTemp(f.dir, ".figures-*.json")
	if err != nil {
		return fmt.Errorf("create the temporary approvals file: %w", err)
	}
	name := tmp.Name()
	if _, err = tmp.Write(append(data, '\n')); err != nil {
		return abandon(tmp, name, fmt.Errorf("write the temporary approvals file: %w", err))
	}
	if err = tmp.Close(); err != nil {
		return abandon(nil, name, fmt.Errorf("close the temporary approvals file: %w", err))
	}
	if err = os.Rename(name, f.ApprovalsPath()); err != nil {
		return abandon(nil, name, fmt.Errorf("install the approvals file: %w", err))
	}
	return nil
}

// validateApproval holds one approval to the shape this file writes: a
// vault-relative path in NFC that is one line of text, and a lowercase hex
// SHA-256.
func validateApproval(approval *FigureApproval) error {
	relPath := approval.RelPath
	switch {
	case !utf8.ValidString(relPath):
		return fmt.Errorf("%w: the figure path is not valid UTF-8", ErrInvalid)
	case relPath == "" || len(relPath) > maxRelPathBytes:
		return fmt.Errorf("%w: the figure path is empty or too long", ErrInvalid)
	case !fs.ValidPath(relPath) || relPath == ".":
		return fmt.Errorf("%w: the figure path is not vault-relative", ErrInvalid)
	case relPath != vault.NormalizeNFC(relPath):
		return fmt.Errorf("%w: the figure path is not written in NFC", ErrInvalid)
	case strings.ContainsFunc(relPath, origin.EndsALine):
		return fmt.Errorf("%w: the figure path holds a control character", ErrInvalid)
	case len(approval.SHA256) != identityLength || approval.SHA256 != strings.ToLower(approval.SHA256):
		return fmt.Errorf("%w: the digest is not 64 lowercase hex digits", ErrInvalid)
	case approval.At.IsZero():
		return fmt.Errorf("%w: the approval has no time", ErrInvalid)
	}
	if _, err := hex.DecodeString(approval.SHA256); err != nil {
		return fmt.Errorf("%w: the digest is not hex", ErrInvalid)
	}
	return nil
}
