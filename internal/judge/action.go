package judge

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// errVaultScan is what an observation that could not be made answers with. It
// and the withheld variant below are declared here rather than with the
// package's other types because choosing between them is this file's whole
// subject: how much an observation may say about what it could not read. A
// reader following that decision needs both in front of the code that makes it.
var errVaultScan = errors.New("vault scan failed")

// errWithheldUnreadable is the whole answer about a file the contract keeps out
// of agent-facing output. The caller is told the command has none; which file
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
	reader    *vault.Reader
	scan      vault.Scan
	authority scanAuthority
	notes     []note
	resources []string
	// unreadable are the files the scan listed and the read could not open.
	// They are the observation's own account of where it has a hole, which
	// decides both what may still be concluded from it and what each command
	// is willing to answer at all.
	unreadable []unreadableEntry
}

// unreadableEntry is one file the scan listed and the read could not open: the
// path the scan gave it, and the machine's own account of what stopped. The
// cause is kept as the error rather than as its text so the refusal a command
// builds from it wraps the same value it always did.
type unreadableEntry struct {
	path  string
	cause error
}

// partialCorpus reports whether this observation has a hole in it. A rule whose
// verdict rests on something being absent cannot be reached over one, and a
// command whose whole answer is such a verdict cannot be given at all.
func (a *action) partialCorpus() bool {
	return len(a.unreadable) > 0
}

// unreadableRefusal is the refusal a command gives instead of an answer it
// cannot stand behind. It names the first file the reads stopped on, in scan
// order, which is the one a run used to end at.
func (a *action) unreadableRefusal() error {
	first := a.unreadable[0]
	return entryUnreadable(first.path, first.cause, a.authority)
}

func openAction(ctx context.Context, root string, hooks actionHooks) (*action, error) {
	reader, err := vault.Open(root)
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
	marks := plannedMarksFrom(a.authority.contract)
	for _, entry := range a.scan.Files() {
		relPath := entry.Path()
		if !vault.IsMarkdown(relPath) || a.authority.contract.SkipsBasename(relPath) {
			a.resources = append(a.resources, relPath)
			continue
		}
		data, readErr := reader.ReadFile(ctx, entry)
		if readErr != nil {
			if readVoidsTheObservation(ctx, readErr) {
				return nil, a.abort(entryUnreadable(relPath, readErr, a.authority))
			}
			a.unreadable = append(a.unreadable, unreadableEntry{path: relPath, cause: readErr})
			continue
		}
		if hooks.afterNoteRead != nil {
			hooks.afterNoteRead(relPath)
		}
		a.notes = append(a.notes, parseNoteWithMarks(relPath, data, marks))
	}
	// A judgement needs something to be about. Where every read failed there is
	// nothing in hand to judge, and the caller is owed the refusal rather than a
	// page whose every line says the same thing under an exit code that reads as
	// success. A root taken away under the run arrives here too, since a read
	// through it then fails for every file at once.
	if len(a.notes) == 0 && a.partialCorpus() {
		return nil, a.abort(a.unreadableRefusal())
	}
	return a, nil
}

// readVoidsTheObservation reports whether a failed read ended the observation
// rather than holed it. Two do. A cancelled run is over, and every read left in
// the walk would fail the same way, so reporting them would turn a caller
// hanging up into a report. A path that no longer names what the scan observed
// says the folder changed identity underneath this run — a note deleted,
// replaced, or moved out from under its parent all arrive this way — which
// makes every finding already collected a statement about a vault that is not
// there; the pinned root exists to catch exactly that, and downgrading it to a
// note about one file would spend it.
//
// Everything else the machine reports is about one file that is still the file
// the scan saw — a permission taken away, a device that would not answer — and
// the rest of the vault is there to be judged.
func readVoidsTheObservation(ctx context.Context, readErr error) bool {
	return ctx.Err() != nil || errors.Is(readErr, vault.ErrSourceChanged)
}

// entryUnreadable names a file a read stopped on and the reason the machine
// gave. It is what every command says when it has no answer to give about the
// file: the two whose whole answer is a verdict about something being absent —
// a census of what nothing points at, and an answer that no note carries a
// name — because a corpus with a hole in it supports neither; all three when
// the read failure ended the observation rather than holing it, or when nothing
// at all could be read. The check command otherwise reports such a file at the
// weight of the gravest thing it has and goes on judging what it did read.
//
// The path comes from the scan entry, not from the error, whose own path names
// one component.
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

// abort ends a run that failed. The cause was built from the authority the run
// started with, and it may name a file: the contract can have been narrowed
// since that name was taken, in which case saying it would describe ground the
// contract has closed. So the same source a finished payload is checked against
// is checked again here, and a run that fails says no more about the folder
// than a run that succeeds would.
//
// Authority that never loaded has nothing to recheck, and its own refusal is
// what the caller is owed: a folder carrying no contract has to keep saying so
// rather than report a privacy authority it never had.
func (a *action) abort(cause error) error {
	if a.authority.contract != nil {
		if authorityErr := a.authority.validate(); authorityErr != nil {
			cause = authorityErr
		}
	}
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
