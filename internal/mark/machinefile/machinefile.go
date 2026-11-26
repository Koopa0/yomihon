// Package machinefile is the one code that may write inside a vault, and the one
// file it may write there: this machine's own file of marks, at
// reader-notes/.yomihon/marks/<machine id>.jsonl.
//
// What it works on is a Folder, which only Open makes, from the vault the server
// pinned. A function that changes a file takes one, and every change goes
// through os.Root handles opened from it, one folder at a time, each held to
// being a folder and not a link. Inside the marks folder it does three things
// and no others: it creates a temporary file under a name it made, renames that
// file over this machine's file, and removes a temporary file it made or one a
// crash left. internal/archlock reads this package with its types and holds it
// to exactly that, so a change here that opened, renamed or removed any other
// name, the reader's own Markdown above all, fails the build.
//
// It knows nothing of what the file holds; the lines and their rules belong to
// the code that calls it. It takes no lock: a caller that reads the file before
// it replaces it holds the vault's lock around both.
package machinefile

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/vault"
)

// The folders on the way to the machine files. They are not configurable, and
// they are constants so that the guard can read, at each call that makes or
// enters a folder, which one it is.
const (
	readerFolder  = "reader-notes"
	yomihonFolder = ".yomihon"
	marksFolder   = "marks"
)

const (
	// extension is what every machine file's name ends in, and the one thing a
	// reader of the folder lists by.
	extension = ".jsonl"
	// tempPrefix begins every temporary file's name. It is a dot name, which no
	// reader of the folder lists, and does not end in extension, so a file a
	// crash leaves behind is never read as another machine's marks.
	tempPrefix = ".tmp-"
	// staleAfter is how old a temporary file has to be before a write removes
	// it. A younger one may be another process's write still in progress, and a
	// sync on a USB disk or a network share can take a long time, so the margin
	// is wide: a temporary file a crash left costs nothing while it waits.
	staleAfter = 10 * time.Minute
	folderMode = 0o755
	fileMode   = 0o644
	// renameAttempts and renameWait are how long a rename that failed for a
	// reason that passes is tried again: on Windows a rename over a file
	// someone has open to read is refused until they close it, and a reader of
	// the folder holds a file open for the instant it takes to read it. Nothing
	// else is retried, and on other platforms no rename is, since none of their
	// failures is one that passes.
	renameAttempts = 8
	renameWait     = 10 * time.Millisecond
)

// MaxFileBytes is the most a machine file may hold. Every reader of the folder
// reads another machine's file up to this size, so a longer one would be a file
// no other machine can read, and is refused rather than written.
const MaxFileBytes = 16 << 20

// ErrNotAFolder means a name on the way to the machine file is a link, or
// something other than a folder. A link is refused wherever it points: one that
// pointed at a folder of notes would have the marks written among them.
var ErrNotAFolder = errors.New("is a link or not a folder")

// ErrMoved means the vault's folder is no longer at the path it was pinned
// from, so the path would open some other folder.
var ErrMoved = errors.New("the vault's folder is no longer where it was opened")

// ErrMachineID means a machine id is not 8 lowercase hexadecimal digits.
var ErrMachineID = errors.New("a machine id is 8 lowercase hexadecimal digits")

// ErrTooLarge means the data is longer than MaxFileBytes.
var ErrTooLarge = errors.New("the machine file would be longer than its limit")

// Folder is the vault's reader folder, as a capability to write this machine's
// file in it. Only Open makes one, and nothing outside this package can fill
// one in. The zero Folder names nothing and is refused by everything here.
type Folder struct {
	vault *os.Root
}

// The names a write in the marks folder may use. os.Root takes strings, so each
// call passes the one field of one of these, and internal/archlock reads that
// from the types: a rename only moves a tempName over a machineFileName, and an
// open to write or a removal only ever names a tempName. Only newTemp and
// staleTemps make a tempName and only machineFile makes a machineFileName, so
// no other string can reach those calls.
type (
	tempName        struct{ temp string }
	machineFileName struct{ file string }
)

// Open pins the reader folder of the vault r reads. It writes nothing and makes
// no folder: a vault that has never had a mark written stays as it is. It
// refuses a reader folder, or a folder inside it on the way to the machine
// files, that is a link or not a folder, with ErrNotAFolder.
//
// The vault is opened again from the path r was pinned at and compared with
// r's own handle, so a folder moved there since is refused with ErrMoved.
func Open(r *vault.Reader) (*Folder, error) {
	root, err := os.OpenRoot(r.Name())
	if err != nil {
		return nil, fmt.Errorf("open the vault to write: %w", err)
	}
	same, err := r.SameRoot(root)
	if err == nil && !same {
		err = ErrMoved
	}
	f := &Folder{vault: root}
	if err == nil {
		var d *folders
		d, err = f.enter()
		err = errors.Join(err, d.close())
	}
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return f, nil
}

// Close releases the vault's handle.
func (f *Folder) Close() error {
	if f == nil || f.vault == nil {
		return nil
	}
	return f.vault.Close()
}

// Measurement is what one replacement took, for the write log. It holds no
// path and no content.
type Measurement struct {
	// Bytes is the length of the data written.
	Bytes int
	// Write is the folders made and entered, and the temporary file written and
	// synced.
	Write time.Duration
	// Install is the rename and the sync of each folder it changed.
	Install time.Duration
	// ReadBack is the file read back and compared.
	ReadBack time.Duration
}

// ReplaceMachineFile installs data as the machine file of machineID, making the
// folders on the way on the first write.
//
// machineID is this machine's own id, which the caller keeps and passes. Only
// its shape is checked here: nothing in this package or in the guard can tell
// this machine's id from another machine's, so a caller that passed another's
// would replace that machine's file.
//
// The bytes go to a temporary file in the marks folder, which is synced and
// renamed over the machine file, so a machine that stops part way leaves the
// old file or the new one and never part of either. Nil means the write is on
// stable storage: each folder on the way is synced before the temporary file is
// written, the marks folder after the rename, and the file is read back and
// compared with data. A step that fails is an error. One that fails before the
// rename leaves the machine file as it was; one that fails after it, a sync of
// the marks folder or the read back, means the new file is installed and
// whether it is on stable storage is not known. A temporary file older than ten
// minutes, which only a crash leaves, is removed first.
//
// The measurement says what each step that ran took, whether or not it failed.
func (f *Folder) ReplaceMachineFile(machineID string, data []byte) (Measurement, error) {
	m := Measurement{Bytes: len(data)}
	if f == nil || f.vault == nil {
		return m, errors.New("replace the machine file: no folder")
	}
	name, err := machineFile(machineID)
	if err != nil {
		return m, err
	}
	if len(data) > MaxFileBytes {
		return m, fmt.Errorf("%w: %d bytes", ErrTooLarge, len(data))
	}
	start := time.Now()
	d, err := f.prepare()
	if err == nil {
		err = d.replace(name, data, &m, start)
	}
	return m, errors.Join(err, d.close())
}

// File is one machine file as ReadMachineFiles found it.
type File struct {
	// ID is the machine id the file's name says.
	ID string
	// Data is what the file holds. It is nil when Err is not.
	Data []byte
	// Err is why the file could not be read: it is longer than MaxFileBytes, it
	// is not a regular file, or the system refused it. One file that cannot be
	// read does not stop the others being read.
	Err error
}

// ErrNotRegular means a name in the marks folder that ends as a machine file's
// does is a folder, a link, a named pipe or some other thing than a file. It is
// not opened: a named pipe would wait for ever for a writer that never comes.
var ErrNotRegular = errors.New("is not a regular file")

// ErrFileTooLarge means a machine file is longer than MaxFileBytes, the most
// any machine writes, so it was not written by one.
var ErrFileTooLarge = errors.New("is longer than a machine file may be")

// ReadMachineFiles reads every machine file in the marks folder, in the order
// of their names: every entry whose name is a machine id and the extension. It
// reads through the folders it enters one at a time, as a write does, so a link
// on the way is refused, and it makes nothing and changes nothing: a vault with
// no marks folder answers no files, and nil.
//
// The folder is listed and each file then opened, so a file replaced between
// the two is read as the new one, whole, and one removed is left out. A rename
// never shows a reader part of a file.
func (f *Folder) ReadMachineFiles() (files []File, err error) {
	if f == nil || f.vault == nil {
		return nil, errors.New("read the machine files: no folder")
	}
	d, err := f.enter()
	defer func() { err = errors.Join(err, d.close()) }()
	if err != nil || d.marks == nil {
		return nil, err
	}
	entries, err := fs.ReadDir(d.marks.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("list the marks folder: %w", err)
	}
	for _, entry := range entries {
		id, found := strings.CutSuffix(entry.Name(), extension)
		if !found || !machineIDShape.MatchString(id) {
			continue
		}
		file := File{ID: id}
		if !entry.Type().IsRegular() {
			file.Err = fmt.Errorf("%s %w", entry.Name(), ErrNotRegular)
		} else if file.Data, file.Err = readWhole(d.marks, entry.Name()); file.Err != nil {
			file.Data = nil
		}
		files = append(files, file)
	}
	return files, nil
}

// readWhole reads the machine file called name, up to MaxFileBytes.
func readWhole(marks *os.Root, name string) ([]byte, error) {
	file, err := marks.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	info, err := file.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("%s %w", name, ErrNotRegular)
	}
	var data []byte
	if err == nil {
		data, err = io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
	}
	if err = errors.Join(err, file.Close()); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("%s %w", name, ErrFileTooLarge)
	}
	return data, nil
}

// folders are the three folders on the way to the machine files, each opened
// through the one above it.
type folders struct {
	notes, yomihon, marks *os.Root
}

// replace is ReplaceMachineFile inside the marks folder.
func (d *folders) replace(name machineFileName, data []byte, m *Measurement, start time.Time) error {
	removeStale(d.marks, start)
	// Opened before anything is installed, so that a folder that cannot be synced
	// refuses the write while the machine file still holds what it held.
	marks, err := openDirectory(d.marks)
	if err != nil {
		return fmt.Errorf("open the marks folder to sync it: %w", err)
	}
	defer marks.close() //nolint:errcheck // closing a directory opened to sync it reports nothing about the write
	t, err := newTemp(d.marks, data)
	m.Write = time.Since(start)
	if err != nil {
		return err
	}
	installed := time.Now()
	for attempt := 1; ; attempt++ {
		err = d.marks.Rename(t.temp, name.file)
		if err == nil || attempt == renameAttempts || !transient(err) {
			break
		}
		time.Sleep(time.Duration(attempt) * renameWait)
	}
	if err != nil {
		_ = d.marks.Remove(t.temp) //nolint:errcheck // best-effort: the failed rename is what the caller acts on
		return fmt.Errorf("install the machine file: %w", err)
	}
	if err = marks.sync(); err != nil {
		return fmt.Errorf("sync the marks folder: %w", err)
	}
	m.Install = time.Since(installed)
	read := time.Now()
	err = readBack(d.marks, name, data)
	m.ReadBack = time.Since(read)
	return err
}

// machineIDShape is what a machine id is: eight lowercase hexadecimal digits,
// chosen at random once per machine.
var machineIDShape = regexp.MustCompile(`^[0-9a-f]{8}$`)

// machineFile is the name of the machine file of id, and the one way to make a
// machineFileName: a name of the marks folder that ends in extension and holds
// nothing but the id.
func machineFile(id string) (machineFileName, error) {
	if !machineIDShape.MatchString(id) {
		return machineFileName{}, fmt.Errorf("%w: %q", ErrMachineID, id)
	}
	return machineFileName{file: id + extension}, nil
}

// tempShape is what a temporary file this package made is called.
var tempShape = regexp.MustCompile(`^\.tmp-[0-9a-f]{12}$`)

// newTemp writes data to a new temporary file in marks and syncs it. The name
// is new: the file is created exclusively, so nothing that was there is opened.
func newTemp(marks *os.Root, data []byte) (tempName, error) {
	random := make([]byte, 6)
	_, _ = rand.Read(random) // crypto/rand.Read never returns an error
	t := tempName{temp: tempPrefix + hex.EncodeToString(random)}
	file, err := marks.OpenFile(t.temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return tempName{}, fmt.Errorf("create the temporary file: %w", err)
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if err = errors.Join(err, file.Close()); err != nil {
		_ = marks.Remove(t.temp) //nolint:errcheck // best-effort: the failed write is what the caller acts on
		return tempName{}, fmt.Errorf("write the temporary file: %w", err)
	}
	return t, nil
}

// removeStale removes the temporary files in marks that are older than
// staleAfter at now. It is best-effort: a file it cannot remove is one more
// dot file in a folder nothing lists, and is no reason to refuse a write.
func removeStale(marks *os.Root, now time.Time) {
	for _, t := range staleTemps(marks, now) {
		_ = marks.Remove(t.temp) //nolint:errcheck // best-effort, as above
	}
}

// staleTemps lists the temporary files in marks that are older than staleAfter
// at now, and is the other way to make a tempName: from a name that has the
// shape newTemp gives one, of a regular file.
func staleTemps(marks *os.Root, now time.Time) []tempName {
	entries, err := fs.ReadDir(marks.FS(), ".")
	if err != nil {
		return nil
	}
	var stale []tempName
	for _, entry := range entries {
		if !tempShape.MatchString(entry.Name()) || !entry.Type().IsRegular() {
			continue
		}
		if info, err := entry.Info(); err == nil && now.Sub(info.ModTime()) > staleAfter {
			stale = append(stale, tempName{temp: entry.Name()})
		}
	}
	return stale
}

// readBack reads the machine file and compares it with data.
func readBack(marks *os.Root, name machineFileName, data []byte) error {
	file, err := marks.Open(name.file)
	if err != nil {
		return fmt.Errorf("read the machine file back: %w", err)
	}
	held, err := io.ReadAll(io.LimitReader(file, int64(len(data))+1))
	if err = errors.Join(err, file.Close()); err != nil {
		return fmt.Errorf("read the machine file back: %w", err)
	}
	if !bytes.Equal(held, data) {
		return errors.New("the machine file does not read back as what was written")
	}
	return nil
}

// enter opens the reader folder, .yomihon in it and marks in that, as far as
// they are there. It makes nothing and changes nothing, and is all Open does to
// the folders: a folder that is not there is not an error, and enter stops at it
// and answers the ones it entered.
func (f *Folder) enter() (*folders, error) {
	d := &folders{}
	var err error
	d.notes, err = enterReaderFolder(f.vault)
	if err == nil {
		d.yomihon, err = enterYomihonFolder(d.notes)
	}
	if err == nil {
		d.marks, err = enterMarksFolder(d.yomihon)
	}
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	return d, err
}

// prepare is enter for a write: it makes each of the three folders that is not
// there before it enters it, and syncs the folder above each, whether this
// write made the folder or found it.
func (f *Folder) prepare() (*folders, error) {
	d := &folders{}
	err := held(f.vault, f.vault.Mkdir(readerFolder, folderMode))
	if err == nil {
		d.notes, err = enterReaderFolder(f.vault)
	}
	if err == nil {
		err = held(d.notes, d.notes.Mkdir(yomihonFolder, folderMode))
	}
	if err == nil {
		d.yomihon, err = enterYomihonFolder(d.notes)
	}
	if err == nil {
		err = held(d.yomihon, d.yomihon.Mkdir(marksFolder, folderMode))
	}
	if err == nil {
		d.marks, err = enterMarksFolder(d.yomihon)
	}
	return d, err
}

func enterReaderFolder(root *os.Root) (*os.Root, error) {
	opened, err := root.OpenRoot(readerFolder)
	return entered(root, readerFolder, opened, err)
}

func enterYomihonFolder(notes *os.Root) (*os.Root, error) {
	opened, err := notes.OpenRoot(yomihonFolder)
	return entered(notes, yomihonFolder, opened, err)
}

func enterMarksFolder(yomihon *os.Root) (*os.Root, error) {
	opened, err := yomihon.OpenRoot(marksFolder)
	return entered(yomihon, marksFolder, opened, err)
}

// held takes what a Mkdir in parent answered and syncs parent, so that the
// folder's entry in it is on stable storage: nil when the folder was made or
// was there already and the sync succeeded, the failure otherwise. A folder that
// was there is synced too, since nothing says that the write that made it
// synced it: that write may have failed at this very step. It costs a sync of
// each of the three folders above the machine file on every write.
func held(parent *os.Root, mkdir error) error {
	if mkdir != nil && !errors.Is(mkdir, fs.ErrExist) {
		return fmt.Errorf("make a folder on the way to the machine file: %w", mkdir)
	}
	d, err := openDirectory(parent)
	if err == nil {
		err = errors.Join(d.sync(), d.close())
	}
	if err != nil {
		return fmt.Errorf("sync the folder above a folder on the way to the machine file: %w", err)
	}
	return nil
}

// entered holds a folder just opened through parent to being the folder that
// parent names and not a link. It is checked after the open, against what is
// open, so a link put in its place at any moment is refused: the name is a
// folder only if it is not a link now and is the very folder that was opened.
func entered(parent *os.Root, name string, opened *os.Root, openErr error) (*os.Root, error) {
	if openErr != nil {
		if info, err := parent.Lstat(name); err == nil && info.Mode().Type() != fs.ModeDir {
			return nil, fmt.Errorf("%s %w", name, ErrNotAFolder)
		}
		return nil, fmt.Errorf("open %s: %w", name, openErr)
	}
	named, err := parent.Lstat(name)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open %s: %w", name, err), opened.Close())
	}
	here, err := opened.Stat(".")
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open %s: %w", name, err), opened.Close())
	}
	if named.Mode().Type() != fs.ModeDir || !os.SameFile(named, here) {
		return nil, errors.Join(fmt.Errorf("%s %w", name, ErrNotAFolder), opened.Close())
	}
	return opened, nil
}

// close releases the folders that were entered.
func (d *folders) close() error {
	if d == nil {
		return nil
	}
	var err error
	for _, r := range []*os.Root{d.marks, d.yomihon, d.notes} {
		if r != nil {
			err = errors.Join(err, r.Close())
		}
	}
	return err
}

// ConflictMarkers counts the lines of data that are git's conflict markers: a
// run of seven or more '<', '>' or '|' alone or followed by a space, or a line
// of seven or more '=' alone. A line of a machine file is a JSON object, which
// begins with '{', so no line of marks is one. A trailing carriage return is
// not part of the line.
func ConflictMarkers(data []byte) int {
	count := 0
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if conflictMarker(bytes.TrimSuffix(line, []byte("\r"))) {
			count++
		}
	}
	return count
}

func conflictMarker(line []byte) bool {
	if len(line) < 7 {
		return false
	}
	c := line[0]
	if c != '<' && c != '>' && c != '|' && c != '=' {
		return false
	}
	run := len(line) - len(bytes.TrimLeft(line, string(c)))
	if run < 7 {
		return false
	}
	if run == len(line) {
		return true
	}
	return c != '=' && line[run] == ' '
}
