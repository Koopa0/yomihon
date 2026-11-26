package mark

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/koopa0/yomihon/internal/mark/machinefile"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/vault"
)

// What a page is told about whether the reader's folder in the vault can be
// written to. The empty word is that it can: the control to mark something is
// drawn only then.
const (
	WritableOK = ""
	// WritableNotAFolder is something that is not a folder where the folder is.
	WritableNotAFolder = "not_a_folder"
	// WritableLink is a link standing where the folder is, which is refused
	// wherever it points.
	WritableLink = "link"
	// WritableReadOnly is a folder its owner may not write in.
	WritableReadOnly = "read_only"
)

// AllMarks is the gate of the reading server: every mark. The server answers the
// reader at their own browser, on the loopback, about marks they made, so it
// holds none back; the privacy rules the vault declares keep an agent's notes out
// of what leaves the process, which is the gate the commands that print put on,
// not this one.
func AllMarks(*machinemarks.Mark) bool { return true }

// Kept is the reader's highlights, thoughts and questions as the vault keeps
// them: the machine files under reader-notes/ and the ledger beside the other
// files of the vault's configuration directory. The folder is opened when it is
// first asked for and again after a failure, so that a vault whose reader
// folder cannot be used still opens for reading, and one that is mended is
// written to without a restart.
type Kept struct {
	store  *Store
	source *vault.Reader
	log    *slog.Logger
	gate   machinemarks.Gate

	mu     sync.Mutex
	folder *machinefile.Folder
	marks  *machinemarks.Marks
	// said is what the log last said of the state of the files, so that a state
	// that stays is said once and not at every page.
	said string
}

// OpenKept names the marks of the vault source holds. It reads and writes
// nothing: the earlier file of marks, which store keeps, is what the first
// write of a machine with no file takes in. gate is the privacy rules every
// call is made under; a mark it withholds is as if it were not there.
func OpenKept(store *Store, source *vault.Reader, log *slog.Logger, gate machinemarks.Gate) *Kept {
	if store == nil || source == nil || log == nil || gate == nil {
		panic("mark: OpenKept requires a Store, a vault, a logger and a gate")
	}
	return &Kept{store: store, source: source, log: log, gate: gate}
}

// Gate is the gate every read and write of these marks goes through.
func (k *Kept) Gate() machinemarks.Gate { return k.gate }

// Marks is this machine's hand on the marks. It is an error when the reader
// folder cannot be entered: it is a link, or not a folder, or moved.
func (k *Kept) Marks() (*machinemarks.Marks, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.marks != nil {
		return k.marks, nil
	}
	folder, err := machinefile.Open(k.source)
	if err != nil {
		return nil, fmt.Errorf("open the reader folder: %w", err)
	}
	k.folder = folder
	k.marks = machinemarks.Open(folder, k.store.dir, k.log, machinemarks.Options{Legacy: k.store.MachineMarks})
	return k.marks, nil
}

// View reads the marks of every machine, which writes nothing and makes no
// folder.
func (k *Kept) View() (*machinemarks.View, error) {
	marks, err := k.Marks()
	if err != nil {
		return nil, err
	}
	return marks.View()
}

// Writable says whether the reader folder can be written to, in the words a page
// carries: empty when it can, else why not. It writes nothing and makes
// nothing. A refusal the folder's mode does not show, such as an access list or
// a mount that is read only, is not seen here; the write that is refused
// says so when it is made.
func (k *Kept) Writable() string {
	_, err := k.Marks()
	if err == nil {
		k.mu.Lock()
		folder := k.folder
		k.mu.Unlock()
		readOnly, askErr := folder.ReadOnly()
		if askErr == nil {
			if readOnly {
				return WritableReadOnly
			}
			return WritableOK
		}
		err = askErr
	}
	switch {
	case errors.Is(err, machinefile.ErrLink):
		return WritableLink
	case errors.Is(err, machinefile.ErrNotAFolder):
		return WritableNotAFolder
	default:
		return WritableReadOnly
	}
}

// Pending are the marks of the earlier file that this machine's next write takes
// in and that a read of the vault does not show yet. A page that listed only what
// the vault holds would, for a reader who has not marked anything since the
// upgrade, show none of the marks they have: they are shown from the earlier
// file, which a read never changes, until the first write takes them in. A file
// that cannot be read is an error and not an empty list.
func (k *Kept) Pending(view *machinemarks.View) ([]machinemarks.Mark, error) {
	if !view.AwaitsTakeIn() {
		return nil, nil
	}
	earlier, err := k.store.MachineMarks()
	if err != nil {
		return nil, err
	}
	var out []machinemarks.Mark
	for i := range earlier {
		if _, there := view.Entry(earlier[i].ID, k.gate); !there && k.gate(&earlier[i]) {
			out = append(out, earlier[i])
		}
	}
	return out, nil
}

// Holds says the earlier file has a mark with this id that the vault does not
// show yet, so that a change addressed to it takes the earlier marks in first.
func (k *Kept) Holds(view *machinemarks.View, id string) bool {
	pending, err := k.Pending(view)
	return err == nil && slices.ContainsFunc(pending, func(m machinemarks.Mark) bool { return m.ID == id })
}

// Questions is the questions the reader has open, in the shape the pages that
// list them read: the ones that are not deleted and not resolved, in the order
// they were made, the earlier file's among them until they are taken in. A
// conflict is shown as the value that counts.
func (k *Kept) Questions() ([]Mark, error) {
	view, err := k.View()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	pending, err := k.Pending(view)
	if err != nil {
		return nil, err
	}
	var out []Mark
	for _, e := range view.Entries(k.gate) {
		m := e.Mark()
		if e.Deleted() || m.Kind != machinemarks.Question || !m.Resolved.IsZero() {
			continue
		}
		out = append(out, fromMachine(&m))
	}
	for i := range pending {
		if pending[i].Kind == machinemarks.Question && pending[i].Resolved.IsZero() {
			out = append(out, fromMachine(&pending[i]))
		}
	}
	return out, nil
}

// Mend puts back, without being asked and without a word to the reader, what git
// took out of this machine's file before it was committed: the marks the local
// backup holds and the vault does not. It is called as a page that reads the
// marks is drawn, and writes only when something is missing and the reader folder
// can be written to; a page that finds nothing missing writes nothing. What it
// cannot mend, a file another program changed or that cannot be read, is left as
// it is and said to the log, once for as long as it stays so: the reader is not
// told of it here.
func (k *Kept) Mend() {
	view, err := k.View()
	if err != nil {
		k.say(fmt.Sprintf("unreadable: %v", err), "the reader's marks cannot be read", "error", err)
		return
	}
	k.say(edgeState(view), "the reader's marks have a state this machine cannot show or use",
		"other_computers_damaged", view.Damaged(), "own_damaged", view.DamagedOwn(),
		"unreadable_files", view.Unreadable(), "own_file_unreadable", view.OwnUnreadable(),
		"later_format_lines", view.Newer(), "earlier_file_skipped", view.LegacySkipped(),
		"ledger_changed", view.LedgerChanged(), "configuration_copied", view.NewIDUnsaid())
	missing := view.Missing()
	if missing == 0 {
		return
	}
	if why := k.Writable(); why != WritableOK {
		k.log.Warn("marks are missing from the vault and cannot be put back: the reader folder cannot be written to", "missing", missing, "why", why)
		return
	}
	marks, err := k.Marks()
	if err != nil {
		k.log.Warn("marks are missing from the vault and cannot be put back", "missing", missing, "error", err)
		return
	}
	if _, err = marks.Restore(); err != nil {
		k.log.Warn("marks are missing from the vault and could not be put back", "missing", missing, "error", err)
		return
	}
	k.log.Info("put back marks that were no longer in the vault", "marks", missing)
}

// edgeState names the states of the files that the reader is not told of, empty
// when there is none, so that the log says a state when it changes.
func edgeState(v *machinemarks.View) string {
	if v.Damaged()+v.DamagedOwn()+v.Unreadable()+v.Newer()+v.LegacySkipped()+v.LedgerChanged() == 0 && !v.OwnUnreadable() && !v.NewIDUnsaid() {
		return ""
	}
	return fmt.Sprintf("%d/%d/%d/%d/%d/%d/%t/%t", v.Damaged(), v.DamagedOwn(), v.Unreadable(), v.Newer(), v.LegacySkipped(), v.LedgerChanged(), v.OwnUnreadable(), v.NewIDUnsaid())
}

// say logs a state of the files at Warn the first time it is seen and again only
// when it changes; an empty state is none and is not said.
func (k *Kept) say(state, msg string, args ...any) {
	k.mu.Lock()
	changed := k.said != state
	k.said = state
	k.mu.Unlock()
	if changed && state != "" {
		k.log.Warn(msg, args...)
	}
}

// Close lets the folder go. It is called once, after the last request.
func (k *Kept) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.folder == nil {
		return nil
	}
	err := k.folder.Close()
	k.folder, k.marks = nil, nil
	return err
}

// fromMachine is a mark of the machine files as the pages that list marks read
// one: the reader's words are its reason when it is a question and its body
// when it is a thought.
func fromMachine(m *machinemarks.Mark) Mark {
	out := Mark{
		ID: m.ID, Kind: Kind(m.Kind), RelPath: m.Path, Anchor: m.Anchor,
		Quote: m.Quote, Before: m.Before, After: m.After, Identity: m.Identity,
		ResolvedAt: m.Resolved, At: m.At,
	}
	if m.Kind == machinemarks.Question {
		out.Reason = m.Body
	} else {
		out.Body = m.Body
	}
	return out
}

// toMachine is the other way, for a mark an earlier file kept.
func toMachine(m *Mark) machinemarks.Mark {
	words := m.Body
	if m.Kind == Question {
		words = m.Reason
	}
	return machinemarks.Mark{
		ID: m.ID, Kind: machinemarks.Kind(m.Kind), Path: m.RelPath, Anchor: m.Anchor,
		Quote: m.Quote, Before: m.Before, After: m.After, Identity: m.Identity,
		Body: words, Resolved: m.ResolvedAt, At: m.At,
	}
}

// TakeIn takes in the marks of the earlier file that were not taken in, which an
// adoption of the marks of another path leaves to do when the vault had taken in
// its own already. What a machine file already holds, a deleted mark included,
// is not taken in again.
func (k *Kept) TakeIn() error {
	marks, err := k.Marks()
	if err != nil {
		return err
	}
	_, err = marks.TakeInLegacy()
	return err
}

// Held says the reader has marks for this vault anywhere: in the vault's files,
// in the local backup, or in the earlier file. A reader who never marked anything
// and one who cleared every mark both hold none, and only the files tell them
// apart.
func (k *Kept) Held() (bool, error) {
	view, err := k.View()
	if err != nil {
		return true, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	pending, err := k.Pending(view)
	if err != nil {
		return true, err
	}
	if view.MachineID() != "" || len(view.Entries(k.gate)) > 0 || len(pending) > 0 {
		return true, nil
	}
	_, stored, err := k.store.loadMarks()
	return stored, err
}
