package judge

import (
	"context"
	"errors"
	"path"

	"github.com/koopa0/yomihon/internal/vault"
)

var errVaultScan = errors.New("vault scan failed")

type actionHooks struct {
	afterScan     func()
	afterNoteRead func(string)
}

// action is one complete, pinned observation used by a judge command. The
// reader, contract authority, file membership, and parsed notes all belong to
// the same selected vault directory.
type action struct {
	reader    *vault.Reader
	scan      vault.Scan
	authority scanAuthority
	notes     []note
	resources []string
}

func openAction(root string, hooks actionHooks) (*action, error) {
	ctx := context.Background()
	reader, err := vault.Open(root)
	if err != nil {
		return nil, errPrivacyAuthorityUnavailable
	}
	a := &action{reader: reader}
	a.authority, err = loadScanAuthority(ctx, reader)
	if err != nil {
		return nil, a.abort(err)
	}
	a.scan, err = reader.ScanComplete(ctx)
	if err != nil {
		return nil, a.abort(errVaultScan)
	}
	if hooks.afterScan != nil {
		hooks.afterScan()
	}
	for _, entry := range a.scan.Files() {
		relPath := entry.Path()
		if path.Ext(relPath) != ".md" {
			a.resources = append(a.resources, relPath)
			continue
		}
		data, readErr := reader.ReadFile(ctx, entry)
		if readErr != nil {
			return nil, a.abort(errVaultScan)
		}
		if hooks.afterNoteRead != nil {
			hooks.afterNoteRead(relPath)
		}
		a.notes = append(a.notes, parseNote(relPath, data))
	}
	return a, nil
}

func (a *action) finish() error {
	if a == nil {
		return errVaultScan
	}
	authorityErr := a.authority.validate()
	closeErr := a.close()
	if authorityErr != nil {
		return authorityErr
	}
	return closeErr
}

func (a *action) abort(cause error) error {
	if closeErr := a.close(); closeErr != nil {
		return closeErr
	}
	return cause
}

func (a *action) close() error {
	if a == nil || a.reader == nil {
		return nil
	}
	reader := a.reader
	a.reader = nil
	if err := reader.Close(); err != nil {
		return errVaultScan
	}
	return nil
}
