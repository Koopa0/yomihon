package judge

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

var errVaultScan = errors.New("vault scan failed")

// errWithheldUnreadable is the whole answer about a file the contract keeps out
// of agent-facing output. One file yomihon cannot read stops the judgement,
// which the caller has to be told; which file it was, and what the machine said
// about it, are description of ground the contract closed, so they are withheld
// the way a path filter into the same ground is refused. The sentence is fixed,
// so it says the same thing for every such file and every cause and answers
// nothing about what is in there.
var errWithheldUnreadable = errors.New(
	"vault scan failed: a file under a directory this vault's contract withholds from agent-facing output could not be read; naming it or the reason would describe ground the contract closed",
)

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
		// Opening the folder fails before one vault byte is read, so there is
		// no policy state to report and nothing observed to withhold. Answering
		// that a privacy authority is unavailable named a fault in a contract
		// file that, for the ordinary case of a mistyped folder, is not there to
		// be at fault — and it carried a paragraph telling the reader where that
		// file lives. A scan that could not start is what happened.
		return nil, errVaultScan
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
// gave for it. One file that cannot be read ends the whole judgement, because a
// report built on a partial corpus would answer about ground it never read; the
// operator's only route back is being told which file to look at, which the
// reading face's diagnostics have always said and this face did not. The
// vault-relative path comes from the scan entry rather than from the error,
// whose own path names only the component the read was standing on.
func entryUnreadable(relPath string, cause error, authority scanAuthority) error {
	if !authority.egressAllowed(relPath) {
		return errWithheldUnreadable
	}
	return fmt.Errorf("vault scan failed: %s: %w", relPath, cause)
}

// scanStopped names the path a scan stopped on, when the failure carries one.
// A scan walks the whole folder rather than one selected file, so the path has
// to be recovered from the error rather than taken from the entry that was
// being read; the walk states it relative to the vault root, in whichever
// composed or decomposed spelling the filesystem handed over, and the privacy
// policy canonicalizes what it is asked about, so the contract answers about
// the directory it declared and not about a different string for the same name.
// A cause that names nothing keeps the bare refusal: a path invented for a
// message the operator would go looking with is worse than no path.
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
// thing inside the vault, which is what the contract can be asked a question
// about. A walk that fails on the folder itself reports "." and one that fails
// before it starts may report nothing usable; asking the privacy policy about
// either gets a refusal for the reason that the string is unanswerable, not
// because anything is withheld, and reporting that refusal would tell an
// operator whose contract withholds nothing that his own vault root is private.
func nameableVaultPath(relPath string) bool {
	return relPath != vaultRoot && !strings.Contains(relPath, `\`) && fs.ValidPath(relPath)
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
