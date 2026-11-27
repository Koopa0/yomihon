package mark

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

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

// AllMarks is the gate every read of these marks is made under: every mark. The
// reader is the owner of them, and the server answers them at their own
// browser, on the loopback, about marks they made; the privacy rules the vault
// declares keep an agent's notes out of what a command prints, which is the
// gate the export puts on its own reads.
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

	mu     sync.Mutex
	folder *machinefile.Folder
	marks  *machinemarks.Marks
	// said is what the log last said of each topic of the state of the files, so
	// that a state that stays is said once and not at every page.
	said map[string]string
	// mendedAt is when the last mend began, and now the clock, for tests.
	mendedAt time.Time
	now      func() time.Time
}

// SetClock gives the marks a clock, for tests, so that a test moves time on
// without waiting. Nil is the clock.
func (k *Kept) SetClock(now func() time.Time) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.now = now
}

// clock is the time now. It is called with k.mu held.
func (k *Kept) clock() time.Time {
	if k.now != nil {
		return k.now()
	}
	return time.Now()
}

// OpenKept names the marks of the vault source holds. It reads and writes
// nothing: the earlier file of marks, which store keeps, is what the first
// write of a machine with no file takes in.
func OpenKept(store *Store, source *vault.Reader, log *slog.Logger) *Kept {
	if store == nil || source == nil || log == nil {
		panic("mark: OpenKept requires a Store, a vault and a logger")
	}
	return &Kept{store: store, source: source, log: log}
}

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
//
// Where the reader folder cannot be entered, because it is a link or not a
// folder, the view holds no mark of the vault and the earlier file's marks are
// what Pending lists, so that the reader still sees the marks they have; the
// writes are refused, and Mend says so to the log.
func (k *Kept) View() (*machinemarks.View, error) {
	marks, err := k.Marks()
	if err != nil {
		if errors.Is(err, machinefile.ErrLink) || errors.Is(err, machinefile.ErrNotAFolder) {
			return machinemarks.LegacyView(), nil
		}
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
		if _, there := view.Entry(earlier[i].ID, AllMarks); !there {
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
	for _, e := range view.Entries(AllMarks) {
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

// Live is the marks of one note the reader has kept and not deleted, in the
// order they were made, the earlier file's among them until they are taken in. A
// conflict is shown as the value that counts, so a page that draws these draws
// what the newest of the writers wrote. An error says the marks cannot be read;
// the page is then drawn without them.
func (k *Kept) Live(rel string) ([]machinemarks.Mark, error) {
	view, err := k.View()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	pending, err := k.Pending(view)
	if err != nil {
		return nil, err
	}
	var out []machinemarks.Mark
	for _, e := range view.Entries(AllMarks) {
		if m := e.Mark(); !e.Deleted() && m.Path == rel {
			out = append(out, m)
		}
	}
	for i := range pending {
		if pending[i].Path == rel && !pending[i].Deleted {
			out = append(out, pending[i])
		}
	}
	return out, nil
}

// mendEvery is the least time between two mends in one process. A page that
// reads the marks is drawn many times a minute, and each mend reads every
// machine file; the marks git takes out of the working tree are put back at the
// next page after this long, not at each page.
const mendEvery = 5 * time.Second

// Mend puts back, without being asked and without a word to the reader, what git
// took out of this machine's file before it was committed: the marks the local
// backup holds and the vault does not. It is called as a page that reads the
// marks is drawn, at most once in mendEvery, and writes only when something is
// missing and the reader folder can be written to; a page that finds nothing
// missing writes nothing, so a page that reads the marks writes into the vault
// only here. What it cannot mend, a file another program changed or that cannot
// be read, is left as it is and said to the log, once for as long as it stays
// so: the reader is not told of it here.
func (k *Kept) Mend() {
	k.mu.Lock()
	now := k.clock()
	if !k.mendedAt.IsZero() && now.Sub(k.mendedAt) < mendEvery {
		k.mu.Unlock()
		return
	}
	k.mendedAt = now
	k.mu.Unlock()

	marks, err := k.Marks()
	if err != nil {
		k.say("folder", fmt.Sprintf("unusable: %v", err), "the reader folder cannot be used: the marks shown are those of the earlier file, and none is kept", "error", err)
		return
	}
	k.say("folder", "", "")
	view, err := marks.View()
	if err != nil {
		k.say("read", fmt.Sprintf("unreadable: %v", err), "the reader's marks cannot be read", "error", err)
		return
	}
	k.say("read", "", "")
	k.say("files", edgeState(view), "the reader's marks have a state this machine cannot show or use",
		"other_computers_damaged", view.Damaged(), "own_damaged", view.DamagedOwn(),
		"unreadable_files", view.Unreadable(), "own_file_unreadable", view.OwnUnreadable(),
		"later_format_lines", view.Newer(), "earlier_file_skipped", view.LegacySkipped(),
		"ledger_changed", view.LedgerChanged(), "configuration_copied", view.IDRetired())
	missing := view.Missing()
	if missing == 0 {
		k.say("missing", "", "")
		return
	}
	if why := k.Writable(); why != WritableOK {
		k.say("missing", "unwritable:"+why, "marks are missing from the vault and cannot be put back: the reader folder cannot be written to", "missing", missing, "why", why)
		return
	}
	// Mend is the vault's, not the page's that happened to call it: what it puts
	// back is put back whether or not that page is still waiting.
	if _, err = marks.Restore(context.Background()); err != nil {
		k.say("missing", "failed:"+err.Error(), "marks are missing from the vault and could not be put back", "missing", missing, "error", err)
		return
	}
	k.say("missing", "", "")
	k.log.Info("put back marks that were no longer in the vault", "marks", missing)
}

// edgeState names the states of the files that the reader is not told of, empty
// when there is none, so that the log says a state when it changes.
func edgeState(v *machinemarks.View) string {
	if v.Damaged()+v.DamagedOwn()+v.Unreadable()+v.Newer()+v.LegacySkipped()+v.LedgerChanged() == 0 && !v.OwnUnreadable() && !v.IDRetired() {
		return ""
	}
	return fmt.Sprintf("%d/%d/%d/%d/%d/%d/%t/%t", v.Damaged(), v.DamagedOwn(), v.Unreadable(), v.Newer(), v.LegacySkipped(), v.LedgerChanged(), v.OwnUnreadable(), v.IDRetired())
}

// say logs a state of one topic at Warn the first time it is seen and again only
// when it changes; an empty state is none and is not said.
func (k *Kept) say(topic, state, msg string, args ...any) {
	k.mu.Lock()
	changed := k.said[topic] != state
	if k.said == nil {
		k.said = map[string]string{}
	}
	k.said[topic] = state
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
// is not taken in again. ctx bounds its waits for the locks.
func (k *Kept) TakeIn(ctx context.Context) error {
	marks, err := k.Marks()
	if err != nil {
		return err
	}
	_, err = marks.TakeInLegacy(ctx)
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
	if view.MachineID() != "" || len(view.Entries(AllMarks)) > 0 || len(pending) > 0 {
		return true, nil
	}
	_, stored, err := k.store.loadMarks()
	return stored, err
}
