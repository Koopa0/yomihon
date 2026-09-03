package judge

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

// errVaultScan is what an observation that could not be made answers with. It
// and the withheld variant below are declared here rather than with the
// package's other types because choosing between them is this file's whole
// subject: how much an observation may say about what it could not read. A
// reader following that decision needs both in front of the code that makes it.
var errVaultScan = errors.New("vault scan failed")

// errWithheldUnreadable is the whole answer about a file the contract keeps out
// of agent-facing output. The caller is told the judgement stopped; which file
// and what the machine said about it would describe ground the contract closed.
// The sentence is fixed, so every such file and cause reads the same.
var errWithheldUnreadable = errors.New(
	"vault scan failed: a file under a directory this vault's contract withholds from agent-facing output could not be read; naming it or the reason would describe ground the contract closed",
)

// actionHooks are the two seams a test drives an observation through: after the
// scan is pinned, and after each note is read. Nothing in production sets
// either, and no caller outside this package can — the moments they name are
// inside the observation, which is why the coverage they buy cannot be had from
// the binary that drives it.
type actionHooks struct {
	afterScan     func()
	afterNoteRead func(string)
}

// action is one complete, pinned observation used by a judge command. The
// reader, contract authority, file membership, and parsed notes all belong to
// the same selected vault directory.
type action struct {
	reader    *vaultfs.Reader
	scan      vaultfs.Scan
	authority scanAuthority
	notes     []note
	resources []string
}

func openAction(ctx context.Context, root string, hooks actionHooks) (*action, error) {
	reader, err := vaultfs.Open(root)
	if err != nil {
		// Opening the folder fails before one vault byte is read, so there is
		// no policy state to report and nothing observed to withhold. Answering
		// that a privacy authority is unavailable named a fault in a contract
		// file that, for the ordinary case of a mistyped folder, is not there to
		// be at fault — and it carried a paragraph telling the reader where that
		// file lives. A scan that could not start is what happened.
		//
		// It says which folder and why, because both are already the reader's:
		// the folder is the one they typed and the reason is the machine's
		// answer about it. Withholding them left somebody who mistyped a
		// directory with nothing to correct.
		return nil, fmt.Errorf("%w: %w", errVaultScan, err)
	}
	a := &action{reader: reader}
	a.authority, err = loadScanAuthority(ctx, reader)
	if err != nil {
		return nil, a.abort(err)
	}
	a.scan, err = reader.ScanComplete(ctx)
	if err != nil {
		return nil, a.abort(scanStopped(err, a.authority))
	}
	if hooks.afterScan != nil {
		hooks.afterScan()
	}
	for _, entry := range a.scan.Files() {
		relPath := entry.Path()
		if !vault.IsMarkdown(relPath) {
			a.resources = append(a.resources, relPath)
			continue
		}
		data, readErr := reader.ReadFile(ctx, entry)
		if readErr != nil {
			return nil, a.abort(entryUnreadable(relPath, readErr, a.authority))
		}
		if hooks.afterNoteRead != nil {
			hooks.afterNoteRead(relPath)
		}
		a.notes = append(a.notes, parseNote(relPath, data))
	}
	return a, nil
}

// entryUnreadable names the file a read stopped on and the reason the machine
// gave. One unreadable file ends the whole judgement, because a report built on
// a partial corpus would answer about ground it never read. The path comes from
// the scan entry, not from the error, whose own path names one component.
func entryUnreadable(relPath string, cause error, authority scanAuthority) error {
	if !authority.egressAllowed(relPath) {
		return errWithheldUnreadable
	}
	return fmt.Errorf("vault scan failed: %s: %w", relPath, cause)
}

// scanStopped names the path a scan stopped on, when the failure carries one.
// A scan walks the whole folder, so the path is recovered from the error; the
// privacy policy canonicalizes what it is asked, so a decomposed spelling still
// resolves to the directory the contract declared. A cause that names nothing
// keeps the bare refusal rather than inventing a path to go looking with.
func scanStopped(cause error, authority scanAuthority) error {
	pathErr, ok := errors.AsType[*fs.PathError](cause)
	if !ok || !nameableVaultPath(pathErr.Path) {
		return errVaultScan
	}
	if !authority.egressAllowed(pathErr.Path) {
		return errWithheldUnreadable
	}
	return fmt.Errorf("vault scan failed: %w", pathErr)
}

// nameableVaultPath reports whether a path recovered from a failure names one
// thing inside the vault, which is what the contract can be asked about. A walk
// failing on the folder itself reports "." and one failing before it starts may
// report nothing usable; asking about either would refuse for unanswerability
// and read as though the operator's own vault root were private.
func nameableVaultPath(relPath string) bool {
	return relPath != vaultRoot && !strings.Contains(relPath, `\`) && fs.ValidPath(relPath)
}

func (a *action) finish() error {
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

// close releases the vault capability once. The reader field is the idempotency
// latch: finish closes, and an abort on the way out of a finished run closes
// again, so the second call has to be a no-op rather than a double close.
func (a *action) close() error {
	if a.reader == nil {
		return nil
	}
	reader := a.reader
	a.reader = nil
	if err := reader.Close(); err != nil {
		return errVaultScan
	}
	return nil
}
