package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// The command that hands a program what the reader has left open: the
// locations they marked "Not sure yet" and the thoughts still in an initial
// status, the same selection the "What you left open" shelf shows. Its name,
// and the names and values of its output fields, are gathered here so that
// renaming any of them is one edit: the command line's own words are decided
// when the command that needs them is.
//
// It is the one command that is about the reader rather than the vault. It
// reads the reader's marks file, read-only, and writes nothing anywhere.
const (
	openCommand = "left-open"

	openKindLocation = "location"
	openKindThought  = "thought"

	// The values of a location's lost field: its note is gone, or the note is
	// there and the location the anchor named is not.
	openLostNote     = "note"
	openLostLocation = "location"
)

// openLocation is one marked location as the output carries it. The JSON keys
// are this command's wire words, declared once here beside the kinds above.
type openLocation struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
	// Anchor is left out for a mark on the whole note. For a lost location it
	// is present and empty: an anchor is its heading's words made into an id,
	// and with the heading or the note gone nothing is left to tell whether
	// those words cited withheld ground.
	Anchor     *string `json:"anchor,omitempty"`
	AnchorText string  `json:"anchor_text,omitempty"`
	Title      string  `json:"title,omitempty"`
	Wikilink   string  `json:"wikilink,omitempty"`
	Lost       string  `json:"lost,omitempty"`
	// CreateTime is when the reader marked it, in RFC 3339.
	CreateTime string `json:"create_time"`
}

// openThought is one thought note as the output carries it. Its based_on is
// deliberately absent: the source it cites is a second path, and the list
// answers about the thought alone.
type openThought struct {
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Title    string `json:"title,omitempty"`
	Status   string `json:"status"`
	Wikilink string `json:"wikilink,omitempty"`
	// UpdateDate is the date the note's own frontmatter declares, never a
	// moment yomihon observed.
	UpdateDate string `json:"update_date,omitempty"`
}

var (
	// errOpenNoContract is the refusal for a folder with no contract. The
	// command answers a program, and without the contract's privacy
	// declaration nothing grants it a path to say.
	errOpenNoContract = errors.New(
		"this folder has no vault contract, " + schema.ContractRelPath + ", and " + openCommand +
			" lists nothing for a program to read until one declares [privacy]")

	// errOpenUndeclaredPrivacy is the refusal for a contract that leaves the
	// privacy section out. The repair is named in full because it quotes
	// nothing the vault wrote: the section is simply missing.
	errOpenUndeclaredPrivacy = errors.New(
		"the vault contract declares no privacy policy, so " + openCommand + " lists nothing for a program to read; add a [privacy] table with never_egress_dirs = [] to " +
			schema.ContractRelPath + ", naming in that list any directory whose notes must never leave this machine")

	// errOpenMarksUnreadable is the whole answer about a marks file that
	// exists and cannot be read. The machine's own error names the file's
	// absolute path, which spells the home directory, so it is not repeated.
	errOpenMarksUnreadable = errors.New(
		`the "Not sure yet" marks for this folder cannot be read; nothing was listed`)

	// errOpenScan is what a scan that could not complete answers with. The
	// scan's own error can name two colliding files, either of which may lie
	// in withheld ground, so it is not repeated.
	errOpenScan = errors.New("vault scan failed")

	// errOpenRoot is the whole answer about a root that cannot be opened. The
	// machine's own error spells the absolute path, or where a link points,
	// so it is not repeated.
	errOpenRoot = errors.New("this folder cannot be opened as a vault; nothing was listed")
)

// The lines that say why the list holds no marked location. They go to
// stderr, so stdout stays the data alone, and each says only what was
// observed. The marks file is found by the folder's path as it was spelled,
// so a missing file says nothing of marks kept under another spelling. A file
// that lists nothing here has one line whatever the reason, so that marks all
// in withheld ground read the same as marks all cleared.
const (
	openNoMarksFileNotice = `no location is listed; there is no "Not sure yet" marks file for this folder as spelled`
	openNoMarksNotice     = `no location is listed from the "Not sure yet" marks file for this folder`
)

// openRequest is one resolved invocation of the command.
type openRequest struct {
	root      string
	configDir string
	// noConfigDir is why no configuration directory was resolved, when none
	// was: the marks cannot be found, and a list without them is incomplete.
	noConfigDir string
	format      judge.Format
	// afterAssemble runs once the output is assembled and before the authority
	// it was assembled under is checked again. Nothing in production sets it;
	// a test drives a contract change through that moment with it.
	afterAssemble func()
}

// runOpenThoughts parses and runs the command, writing the list to stdout and
// anything said about it to stderr. It exits 0 with a list, even an empty one,
// and 2 when it refuses; it never exits 1, which another command spends on a
// gate.
func runOpenThoughts(ctx context.Context, args []string, cfg config, stdout, stderr io.Writer, stdoutIsTerminal bool) int {
	request, err := parseOpenArgs(args, stdoutIsTerminal)
	if err != nil {
		return commandError(stderr, err)
	}
	request.configDir, request.noConfigDir = cfg.configDir, cfg.noConfigDir
	return request.run(ctx, stdout, stderr)
}

func parseOpenArgs(args []string, stdoutIsTerminal bool) (openRequest, error) {
	parsed, err := parseCommandArgs(args)
	if err != nil {
		return openRequest{}, err
	}
	switch {
	case parsed.all:
		return openRequest{}, errors.New(openCommand + " does not accept --all")
	case len(parsed.deny) > 0:
		return openRequest{}, errors.New(openCommand + " does not accept --deny")
	case parsed.baseline != "":
		return openRequest{}, errors.New(openCommand + " does not accept --baseline")
	case len(parsed.positionals) > 0:
		return openRequest{}, errors.New(openCommand + " takes no positional arguments")
	}
	if parsed.root == "" {
		if parsed.root, err = os.Getwd(); err != nil {
			return openRequest{}, fmt.Errorf("resolve working directory: %w", err)
		}
	}
	format := judge.ResolveFormat(parsed.format, stdoutIsTerminal)
	if format == judge.FormatMarkdown {
		// As coverage and exists do, md falls back to the human view: a list
		// has no report body of its own.
		format = judge.FormatHuman
	}
	return openRequest{root: parsed.root, format: format}, nil
}

func (r *openRequest) run(ctx context.Context, stdout, stderr io.Writer) int {
	payload, notice, err := r.list(ctx)
	if err != nil {
		return commandError(stderr, err)
	}
	if _, err := stdout.Write(payload); err != nil {
		return commandError(stderr, fmt.Errorf("write output: %w", err))
	}
	if notice != "" {
		_, _ = fmt.Fprintf(stderr, "yomihon: %s\n", notice) //nolint:errcheck // the list is already written; a failing stderr has no second channel
	}
	return 0
}

// list builds the payload under the contract's privacy authority and checks
// that authority again before handing it over. notice is the one line that
// says why no marked location is listed, when that is so.
func (r *openRequest) list(ctx context.Context) (payload []byte, notice string, err error) {
	source, err := vault.Open(r.root)
	if err != nil {
		return nil, "", errOpenRoot
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil && err == nil {
			payload, notice, err = nil, "", errOpenScan
		}
	}()
	contract, err := schema.LoadReader(ctx, source)
	if err != nil {
		if schema.ContractAbsent(err) {
			return nil, "", errOpenNoContract
		}
		// The loader's words name the contract's keys, which is vault content
		// this face has no authority yet to repeat.
		return nil, "", judge.ErrPrivacyAuthorityUnavailable
	}
	if refused := openAuthority(contract); refused != nil {
		return nil, "", refused
	}
	source.SetBoundary(contract.ScanBoundary())
	// The generation logs what it reads by path, private ground included, and
	// this face's stderr goes to the program that called it. Nothing it would
	// say is this command's answer, so it says it to no one.
	store, err := snapshot.New(ctx, source, slog.New(slog.DiscardHandler), contract, contract.Governance())
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, "", ctxErr
		}
		return nil, "", errOpenScan
	}
	snap := store.Current().Capture()
	allowed := openAllowed(contract)
	if missing := openUnreadable(snap, allowed); missing != nil {
		return nil, "", missing
	}
	kept, stored, err := r.marks(source.Name())
	if err != nil {
		return nil, "", err
	}
	entries := openEntries(snap, contract, kept, allowed)
	if r.afterAssemble != nil {
		r.afterAssemble()
	}
	// The list was assembled under the authority read at the start. If the
	// contract changed since, what it withholds may have changed with it, so
	// the list is handed over only while that same source still stands.
	if refused := openAuthority(contract); refused != nil {
		return nil, "", refused
	}
	payload, err = encodeOpen(entries, r.format)
	if err != nil {
		return nil, "", err
	}
	return payload, openNotice(entries, stored), nil
}

// openNotice is the line for a list that holds no location, decided on what
// is listed rather than on what the marks file holds: a count taken before
// the withheld marks were left out would say they are there.
func openNotice(entries []openEntry, stored bool) string {
	switch {
	case slices.ContainsFunc(entries, func(e openEntry) bool { return e.location != nil }):
		return ""
	case !stored:
		return openNoMarksFileNotice
	default:
		return openNoMarksNotice
	}
}

// openAuthority is the contract state this command needs before it may say a
// path to a program: a privacy declaration that loaded and whose source is
// unchanged. Only the missing section is explained; any other fault would be
// explained by quoting the contract back out.
func openAuthority(contract *schema.Contract) error {
	privacy := contract.PrivacyPolicy().ValidateSource()
	switch {
	case privacy.Available():
		return nil
	case privacy.Undeclared():
		return errOpenUndeclaredPrivacy
	default:
		return judge.ErrPrivacyAuthorityUnavailable
	}
}

// openAllowed is the gate every item's path passes before anything about it is
// resolved into the output: the privacy declaration lets it out and the scan
// looks there.
func openAllowed(contract *schema.Contract) func(string) bool {
	privacy := contract.PrivacyPolicy()
	boundary := contract.ScanBoundary()
	return func(rel string) bool {
		return privacy.EgressAllowed(rel) && !boundary.OutsideScan(rel)
	}
}

// openUnreadable refuses a list that could be missing a thought: a note the
// scan saw and could not read may be one, and so may a note whose frontmatter
// could not be read, since its type and status are what would say. An empty
// answer would read as nothing left open. A note in withheld ground is passed
// over in silence, so whether it could be read changes neither the answer nor
// the exit.
func openUnreadable(snap *snapshot.Generation, allowed func(string) bool) error {
	for _, blocked := range snap.Freshness().Blocked {
		if vault.IsMarkdown(blocked.Path) && allowed(blocked.Path) {
			return fmt.Errorf("%s could not be read, so whether it is a thought still open is unknown; nothing was listed", escapeTerminal(blocked.Path))
		}
	}
	answerType := snap.NavigationRoles().AnswerType()
	policy := snap.ArtifactPolicy()
	for _, file := range snap.Files() {
		rel := file.Path()
		reading, ok := snap.Note(rel)
		if !ok || !allowed(rel) || policy.IsNonInstance(rel) {
			continue
		}
		// A type that is not text leaves whether the note is a thought unknown;
		// a status that is not text on a thought leaves whether it is open so.
		unread := snap.NotText(rel, "type") || (schema.NormalizeWord(reading.Type) == answerType && snap.NotText(rel, "status"))
		if reading.FMDiagnostic != "" || reading.FrontmatterUnclosed || unread {
			return fmt.Errorf("%s has frontmatter that could not be read, so whether it is a thought still open is unknown; nothing was listed", escapeTerminal(rel))
		}
	}
	return nil
}

// marks reads the reader's marked locations for this vault, never writing
// the file. stored tells no marks file from one that holds none.
func (r *openRequest) marks(vaultRoot string) (kept []mark.Uncertainty, stored bool, err error) {
	if r.configDir == "" {
		reason := cmp.Or(r.noConfigDir, "the environment names none")
		return nil, false, fmt.Errorf(`no configuration directory, so the "Not sure yet" marks for this folder cannot be found: %s; nothing was listed`, reason)
	}
	file, err := mark.New(r.configDir, vaultRoot)
	if err != nil {
		return nil, false, errOpenMarksUnreadable
	}
	kept, stored, err = file.StoredUncertainties()
	if err != nil {
		return nil, false, errOpenMarksUnreadable
	}
	return kept, stored, nil
}

// openEntry is one listed item: exactly one of its two fields is set.
type openEntry struct {
	location *openLocation
	thought  *openThought
}

// openEntries is the shelf's own selection, each item kept only when its path
// is allowed. That gate runs before the item is resolved any further: a
// location's anchor is its heading's words made into an id, so resolving one
// in withheld ground and then hiding the heading would still have said it.
func openEntries(snap *snapshot.Generation, contract *schema.Contract, kept []mark.Uncertainty, allowed func(string) bool) []openEntry {
	var entries []openEntry
	for _, item := range snap.OpenItems(contract, note.OpenMarks(kept)) {
		if !allowed(item.Path()) {
			continue
		}
		if item.Note != nil {
			if thought, ok := openThoughtEntry(snap, item.Note, allowed); ok {
				entries = append(entries, openEntry{thought: &thought})
			}
			continue
		}
		if location, ok := openLocationEntry(snap, item.Mark, allowed); ok {
			entries = append(entries, openEntry{location: &location})
		}
	}
	return entries
}

func openThoughtEntry(snap *snapshot.Generation, reading *snapshot.Reading, allowed func(string) bool) (openThought, bool) {
	// A thought citing withheld ground is withheld whole: the citation is the
	// shelf's own row text, and leaving it out would still list a thought whose
	// title is often its source's name.
	title := snap.DeclaredTitle(reading.RelPath)
	if !snap.BasedOnWithin(reading.RelPath, allowed) || namesWithheld(snap, title, reading.Body, allowed) {
		return openThought{}, false
	}
	entry := openThought{
		Kind: openKindThought, Path: reading.RelPath, Title: title, Status: reading.Status,
		Wikilink: note.NoteWikilink(snap, reading.RelPath),
	}
	if !reading.Updated.IsZero() {
		entry.UpdateDate = reading.Updated.Format(time.DateOnly)
	}
	return entry, true
}

func openLocationEntry(snap *snapshot.Generation, kept *snapshot.OpenMark, allowed func(string) bool) (openLocation, bool) {
	// The command speaks English, so a card with no pattern of its own is named
	// in the English the page would use.
	located := note.LocateMark(snap, kept.RelPath, kept.Anchor, wording.En)
	reading, _ := snap.Note(kept.RelPath)
	title := ""
	if located.Lost != note.LostNote {
		title = snap.DeclaredTitle(kept.RelPath)
	}
	for _, words := range []string{title, located.AnchorText} {
		if words != "" && namesWithheld(snap, words, reading.Body, allowed) {
			return openLocation{}, false
		}
	}
	entry := openLocation{
		Kind:       openKindLocation,
		Path:       kept.RelPath,
		Title:      title,
		CreateTime: kept.At.UTC().Format(time.RFC3339),
	}
	var anchor string
	switch located.Lost {
	case note.LostNothing:
		anchor = kept.Anchor
		entry.AnchorText = located.AnchorText
		entry.Wikilink = located.Wikilink
	case note.LostNote:
		entry.Lost = openLostNote
	case note.LostLocation:
		entry.Lost = openLostLocation
	default:
		panic("main: unknown Loss: " + located.Lost.String())
	}
	if kept.Anchor != "" {
		entry.Anchor = &anchor
	}
	return entry, true
}

// namesWithheld reports whether text cites, or spells a link name body cites,
// that could reach withheld ground. A heading or a title is the author's
// words, and a wikilink inside one is shown as the name it was written with,
// so a heading citing a withheld note would say that note's path in the list —
// and its anchor, made from the same words, would say it again. A title is
// frontmatter, not body, so the links it writes are read from the title
// itself; a heading's words arrive rendered, so its links are found in body.
func namesWithheld(snap *snapshot.Generation, text, body string, allowed func(string) bool) bool {
	targets := slices.Concat(judge.LinkTargets(text), rawLinkTargets(text))
	if slices.ContainsFunc(targets, func(target string) bool { return !snap.NameWithin(target, allowed) }) {
		return true
	}
	folded := graph.FoldFragment(text)
	for _, target := range judge.LinkTargets(body) {
		name := graph.FoldFragment(strings.TrimSpace(target))
		if name != "" && strings.Contains(folded, name) && !snap.NameWithin(target, allowed) {
			return true
		}
	}
	return false
}

func encodeOpen(entries []openEntry, format judge.Format) ([]byte, error) {
	switch format {
	case judge.FormatJSON:
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		for _, entry := range entries {
			var err error
			if entry.location != nil {
				err = enc.Encode(entry.location)
			} else {
				err = enc.Encode(entry.thought)
			}
			if err != nil {
				return nil, fmt.Errorf("encode the list: %w", err)
			}
		}
		return buf.Bytes(), nil
	case judge.FormatHuman, judge.FormatMarkdown:
		var b strings.Builder
		for _, entry := range entries {
			if entry.location != nil {
				b.WriteString(humanLocationLine(entry.location))
			} else {
				b.WriteString(humanThoughtLine(entry.thought))
			}
		}
		return []byte(b.String()), nil
	default:
		panic("main: unknown Format: " + format.String())
	}
}

// rawLinkTargets is every name text spells as a link, read as plain text
// rather than as Markdown: a title is written out as it stands, so a code span
// or a backslash that keeps a page from rendering a link keeps nothing out of
// the list. It takes the target of each [[…]], with any alias, heading or
// block part cut away, and the destination of each ](…), as written and
// percent-decoded. A destination that climbs out of its folder needs no
// resolving here: NameWithin's gate refuses any path that climbs.
func rawLinkTargets(text string) []string {
	var targets []string
	for rest := text; ; {
		_, after, found := strings.Cut(rest, "[[")
		if !found {
			break
		}
		rest = after
		inner, _, closed := strings.Cut(after, "]]")
		if !closed {
			break
		}
		if i := strings.LastIndex(inner, "[["); i >= 0 {
			inner = inner[i+len("[["):]
		}
		name, _, _ := strings.Cut(inner, "|")
		name, _, _ = strings.Cut(name, "#")
		name, _, _ = strings.Cut(name, "^")
		targets = append(targets, name)
	}
	for rest := text; ; {
		_, after, found := strings.Cut(rest, "](")
		if !found {
			break
		}
		rest = after
		destination, _, closed := strings.Cut(after, ")")
		if !closed {
			break
		}
		destination = strings.TrimSpace(destination)
		if quoted := strings.IndexAny(destination, ` "'`); quoted > 0 {
			destination = destination[:quoted]
		}
		destination = strings.Trim(destination, "<>")
		destination, _, _ = strings.Cut(destination, "#")
		destination, _, _ = strings.Cut(destination, "?")
		targets = append(targets, destination)
		if decoded, err := url.PathUnescape(destination); err == nil {
			targets = append(targets, decoded)
		}
	}
	return targets
}

// The terminal view's own words. A thought that declares no date shows a run
// as wide as a date, so the columns still line up; a location whose note, or
// whose location in the note, is gone says which.
const (
	humanOpenDateless     = "----------"
	humanOpenNoteLost     = "(note not found)"
	humanOpenLocationLost = "(location not found)"
)

// humanLocationLine is one location as a person at a terminal reads it: the
// day it was marked, the kind, the path, then what names the location.
func humanLocationLine(e *openLocation) string {
	line := fmt.Sprintf("%s  %-8s  %s", e.CreateTime[:len(time.DateOnly)], e.Kind, escapeTerminal(e.Path))
	switch {
	case e.AnchorText != "":
		line += "  " + escapeTerminal(e.AnchorText)
	case e.Anchor != nil && *e.Anchor != "":
		line += "  #" + escapeTerminal(*e.Anchor)
	}
	switch e.Lost {
	case openLostNote:
		line += "  " + humanOpenNoteLost
	case openLostLocation:
		line += "  " + humanOpenLocationLost
	}
	return line + "\n"
}

// humanThoughtLine is one thought as a person at a terminal reads it: its
// declared date, the kind, the path, its title and its status.
func humanThoughtLine(e *openThought) string {
	date := cmp.Or(e.UpdateDate, humanOpenDateless)
	line := fmt.Sprintf("%s  %-8s  %s", date, e.Kind, escapeTerminal(e.Path))
	if e.Title != "" {
		line += "  " + escapeTerminal(e.Title)
	}
	return line + "  (" + escapeTerminal(e.Status) + ")\n"
}

// escapeTerminal makes text one line a terminal shows as written: a C0 or C1
// control, DELETE, a Unicode line or paragraph separator, a direction mark
// (LRM, RLM, ALM), or a direction embedding, override or isolate is written as a visible escape — \n, \r and
// \t by name, any other as \u and four hex digits, as JSON writes them —
// rather than handed to the terminal to act on. A path, a title and a heading
// are the vault's words, and the view must not let them forge a line or send
// an escape sequence.
func escapeTerminal(text string) string {
	if !strings.ContainsFunc(text, actsOnTerminal) {
		return text
	}
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case actsOnTerminal(r):
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func actsOnTerminal(r rune) bool {
	return origin.EndsALine(r) || r == 0x200e || r == 0x200f || r == 0x061c ||
		(r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}
