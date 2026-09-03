package schema

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"

	"github.com/koopa0/yomihon/internal/vaultfs"
)

// policySource binds a startup-derived capability to the exact contract file it
// came from, so no later consumer reconstructs provenance from a vault root.
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
			// The pinned identity includes the modification time, which a
			// checkout or a save-by-rename moves without changing a byte. Only
			// the bytes matter here, so select the file again and let the
			// digest below answer.
			data, err = s.rereadCurrent()
		}
	} else {
		data, err = os.ReadFile(s.path) // #nosec G304 -- LoadFile recorded the operator-selected contract path
	}
	return err == nil && sha256.Sum256(data) == s.digest
}

// rereadCurrent selects whatever regular file now carries the contract's name
// under the same pinned root. It grants nothing the startup read did not have:
// a symlink, a non-regular component or a different canonical name fails closed
// here too.
func (s policySource) rereadCurrent() ([]byte, error) {
	current, err := s.pinned.reader.Refresh(s.pinned.entry)
	if err != nil {
		return nil, err
	}
	return s.pinned.reader.ReadFile(context.Background(), current)
}
