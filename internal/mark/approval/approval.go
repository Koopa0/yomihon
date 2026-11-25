// Package approval keeps the figures the owner approved to run their scripts:
// each one a path and the SHA-256 of the exact bytes approved. It is a record
// about the owner's own browser rather than about the vault, which is why it
// lives under the configuration directory and not in the vault.
//
// The reading server only reads it, and only the approve-figure and
// revoke-figure commands write it. No HTTP route does.
package approval

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/vault"
)

// ErrUnreadable means the approvals file exists and cannot be read safely.
// Nothing it holds is honoured — every figure runs without script until the
// file is readable again — and nothing replaces it, so the person who owns it
// can see what went wrong.
var ErrUnreadable = errors.New("the figure approvals cannot be read")

// ErrInvalid means a submitted approval is not a shape this file can hold.
var ErrInvalid = errors.New("the approval is not a shape this file can hold")

// version is the only shape of the file this reads back.
const version = 1

const (
	// fileName is the approvals file inside this vault's own directory.
	fileName = "figures.json"
	// lockFile orders the writers of that file.
	lockFile = "figures.lock"
)

// maxFileBytes bounds what a read takes in and what a write installs. An entry
// is a path of at most maxPathBytes, a digest and a time; a file this long
// holds far more figures than a vault has, and a longer one was not written
// here.
const maxFileBytes = 4 << 20

// maxPathBytes is the longest figure path an approval can name.
const maxPathBytes = 1024

// digestLength is the length of a SHA-256 written as lowercase hex.
const digestLength = 2 * sha256.Size

// Approval is the owner's word that one figure's exact bytes may run their
// inline scripts in the reader's browser. It names the bytes by their digest,
// so any change to the file, by anyone, leaves it naming bytes that are no
// longer there.
type Approval struct {
	RelPath string    `json:"path"`
	SHA256  string    `json:"sha256"`
	At      time.Time `json:"time"`
}

// document is the file's own shape. Version comes first so a reader that does
// not recognise it stops at the first field.
type document struct {
	Version   int        `json:"version"`
	Vault     string     `json:"vault"`
	Approvals []Approval `json:"approvals"`
}

// Store keeps the approvals for one vault. It holds none in memory: the file on
// disk is the record, so a revocation takes effect at the next page and the
// next frame request without a restart.
type Store struct {
	dir       confdir.Dir
	vaultRoot string
	// mu serializes writers, which read the file and replace it whole.
	mu sync.Mutex
}

// Open names this vault's approvals. It creates nothing: the directory and the
// file appear when an owner first approves a figure.
func Open(configDir confdir.ConfigDir, vaultRoot string) (*Store, error) {
	dir, err := confdir.VaultDir(configDir, vaultRoot)
	if err != nil {
		return nil, err
	}
	return &Store{dir: dir, vaultRoot: vaultRoot}, nil
}

// Path is the file the approvals are kept in. It is what a message names when
// it tells the owner where to look.
func (s *Store) Path() string {
	return s.dir.Join(fileName)
}

// Approved reports whether the figure at relPath with digest sum is approved,
// and, when it is not, whether relPath was approved for other bytes: an
// approval that lapsed when the file changed. No file is no approval. A file
// that cannot be read, or carries a shape this version does not write, is an
// error: the caller treats it as no approval and says so, rather than reading
// damage as an empty list that a later approval would quietly replace.
func (s *Store) Approved(relPath string, sum [sha256.Size]byte) (approved, lapsed bool, err error) {
	held, err := s.List()
	if err != nil {
		return false, false, err
	}
	digest := hex.EncodeToString(sum[:])
	for _, approval := range held {
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

// List returns every approval this vault holds, in file order.
func (s *Store) List() ([]Approval, error) {
	data, err := confdir.Read(s.dir, fileName, maxFileBytes)
	if errors.Is(err, os.ErrNotExist) {
		return []Approval{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	var doc document
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	if !json.Valid(data) || doc.Version != version || doc.Approvals == nil {
		return nil, fmt.Errorf("%w: not a version %d approvals file", ErrUnreadable, version)
	}
	seen := make(map[string]bool, len(doc.Approvals))
	for i := range doc.Approvals {
		if err = validate(&doc.Approvals[i]); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrUnreadable, err.Error())
		}
		if seen[doc.Approvals[i].RelPath] {
			return nil, fmt.Errorf("%w: %s is approved twice", ErrUnreadable, doc.Approvals[i].RelPath)
		}
		seen[doc.Approvals[i].RelPath] = true
	}
	return doc.Approvals, nil
}

// Approve records that the figure at relPath may run when its bytes have digest
// sum, replacing any earlier approval of that path: one path is approved for
// one set of bytes at a time. An unreadable file is left as it is and the
// approval is refused.
func (s *Store) Approve(relPath string, sum [sha256.Size]byte, at time.Time) error {
	approval := Approval{RelPath: relPath, SHA256: hex.EncodeToString(sum[:]), At: at.UTC()}
	if err := validate(&approval); err != nil {
		return err
	}
	return s.exclusive(func() error {
		held, err := s.List()
		if err != nil {
			return err
		}
		return s.write(append(without(held, relPath), approval))
	})
}

// Revoke removes the approval of relPath, whatever bytes it named, and reports
// whether there was one. With none, nothing is written.
func (s *Store) Revoke(relPath string) (bool, error) {
	// Asking a store that holds nothing takes no lock: its directory would be
	// left behind. The locked read below decides whenever there is something.
	if held, err := s.List(); err == nil && len(without(held, relPath)) == len(held) {
		return false, nil
	}
	var revoked bool
	err := s.exclusive(func() error {
		held, err := s.List()
		if err != nil {
			return err
		}
		kept := without(held, relPath)
		if len(kept) == len(held) {
			return nil
		}
		revoked = true
		return s.write(kept)
	})
	return revoked, err
}

func without(held []Approval, relPath string) []Approval {
	kept := make([]Approval, 0, len(held))
	for _, approval := range held {
		if approval.RelPath != relPath {
			kept = append(kept, approval)
		}
	}
	return kept
}

// exclusive runs change as the only writer of the approvals, in this process
// and in any other on the same directory.
func (s *Store) exclusive(change func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return confdir.Exclusive(s.dir, lockFile, confdir.LockWait, change)
}

// write installs the whole list. It refuses one the next read would refuse, so
// a file this program writes is always one it can read back.
func (s *Store) write(approvals []Approval) error {
	data, err := json.MarshalIndent(document{Version: version, Vault: s.vaultRoot, Approvals: approvals}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the figure approvals: %w", err)
	}
	data = append(data, '\n')
	if len(data) > maxFileBytes {
		return fmt.Errorf("%w: more approvals than this file holds", ErrInvalid)
	}
	return confdir.Replace(s.dir, fileName, data)
}

// validate holds one approval to the shape this file writes: a vault-relative
// path in NFC that is one line of text, and a lowercase hex SHA-256.
func validate(approval *Approval) error {
	relPath := approval.RelPath
	switch {
	case !utf8.ValidString(relPath):
		return fmt.Errorf("%w: the figure path is not valid UTF-8", ErrInvalid)
	case relPath == "" || len(relPath) > maxPathBytes:
		return fmt.Errorf("%w: the figure path is empty or too long", ErrInvalid)
	case !fs.ValidPath(relPath) || relPath == ".":
		return fmt.Errorf("%w: the figure path is not vault-relative", ErrInvalid)
	case relPath != vault.NormalizeNFC(relPath):
		return fmt.Errorf("%w: the figure path is not written in NFC", ErrInvalid)
	case strings.ContainsFunc(relPath, origin.EndsALine):
		return fmt.Errorf("%w: the figure path holds a control character", ErrInvalid)
	case len(approval.SHA256) != digestLength || approval.SHA256 != strings.ToLower(approval.SHA256):
		return fmt.Errorf("%w: the digest is not 64 lowercase hex digits", ErrInvalid)
	case approval.At.IsZero():
		return fmt.Errorf("%w: the approval has no time", ErrInvalid)
	}
	if _, err := hex.DecodeString(approval.SHA256); err != nil {
		return fmt.Errorf("%w: the digest is not hex", ErrInvalid)
	}
	return nil
}
