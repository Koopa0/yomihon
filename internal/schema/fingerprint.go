package schema

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"

	"github.com/koopa0/yomihon/internal/vaultfs"
)

// policySource binds a startup-derived capability to the exact contract file
// it came from. Keeping the source path with the digest avoids reconstructing
// provenance from a later consumer's vault root.
type policySource struct {
	path   string
	digest [sha256.Size]byte
	pinned *pinnedPolicySource
}

type pinnedPolicySource struct {
	reader *vaultfs.Reader
	entry  vaultfs.Entry
}

func (s policySource) unchanged() bool {
	if s.path == "" {
		return false
	}
	var (
		data []byte
		err  error
	)
	if s.pinned != nil {
		data, err = s.pinned.reader.ReadFile(context.Background(), s.pinned.entry)
		if errors.Is(err, vaultfs.ErrSourceChanged) {
			// The pinned entry records which file this was at startup, and the
			// identity it records includes the modification time — so a git
			// checkout, a pull, a restored backup or an editor that saves by
			// rename all move it without changing a byte. The question here is
			// only whether the contract's bytes moved, and refusing to look is
			// what closed the write face on a bare touch. Select the file again
			// under the same pinned root, which fails closed on a symlink, a
			// non-regular path or a different canonical name exactly as before,
			// and let the digest below give the answer.
			data, err = s.rereadCurrent()
		}
	} else {
		data, err = os.ReadFile(s.path) // #nosec G304 -- LoadFile recorded the operator-selected contract path
	}
	return err == nil && sha256.Sum256(data) == s.digest
}

// rereadCurrent selects whatever regular file now carries the contract's name
// under the same pinned root, so a file whose identity moved can still answer
// for its bytes. It grants nothing the startup read did not have: a symlink, a
// non-regular component or a different canonical name fails closed here as it
// does everywhere else, and the caller still decides by digest.
func (s policySource) rereadCurrent() ([]byte, error) {
	current, err := s.pinned.reader.Refresh(s.pinned.entry)
	if err != nil {
		return nil, err
	}
	return s.pinned.reader.ReadFile(context.Background(), current)
}
