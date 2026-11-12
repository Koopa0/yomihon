package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/figure"
	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// The two commands that speak about the reader rather than the vault. Their
// names are written once, here; the dispatch, the help and every message
// read them from these constants.
const (
	approveFigureCommand = "approve-figure"
	revokeFigureCommand  = "revoke-figure"
)

// confirmDigits is how much of the digest a person types to approve. Eight
// hex digits cannot be typed by accident, and a program that can read the
// report could type them, which is why the command also demands a terminal.
const confirmDigits = 8

// What the approval commands print, in English only: a command line has no
// language choice to follow, and the one variable the product reads is the
// port. Collected here so the words can be reviewed and changed in one place.
const (
	figureUsageFmt         = "usage: yomihon %s [--root <dir>] <path>"
	figureNeedsTerminal    = "approve-figure needs a person at a terminal, and its input or output is not one; nothing was approved"
	figureNoConfigDir      = "this environment names no configuration directory, so there is nowhere to keep an approval"
	figureNotAFigure       = "%s is not a figure: a figure is the .html file as the vault spells it, relative to its root"
	figureNotInVault       = "%s is not a file this vault serves"
	figureRefusedFmt       = "%s is not shown as a figure (%s), so there is nothing to approve"
	figureNoScriptFmt      = "%s has no inline script, so it already runs as it is; there is nothing to approve."
	figureReportFmt        = "Figure to approve\n  Path     %s\n  Size     %d bytes\n  SHA-256  %s\n  Scripts  %d inline\n"
	figureReportAddresses  = "  Address-like text in the file:"
	figureReportNames      = "  Names in the text that reach outside the frame or run text as script:"
	figureReportNone       = "    (none found)"
	figureReportDisclaimer = "This summary is for reference, not a security boundary: a script can do more than what is listed here."
	figureScriptHeaderFmt  = "  Script %d of %d, %d characters, exactly as approved:"
	figureScriptsFooterFmt = "The %d script(s) above, %d characters in all, are everything in this file that can run. They are printed from the same read that is hashed, with control and direction characters shown as \\x or \\u escapes. Approve only what you have read; to read a long one in a pager, open the file in less or an editor, and approve here only if it matches."
	figureConfirmPromptFmt = "To approve, type the first %d characters of the SHA-256 above: "
	figureNotApproved      = "That does not match; nothing was approved."
	figureApprovedFmt      = "Approved %s (sha256 %s…). Reload its page to run its scripts."
	figureRevokedFmt       = "Revoked the approval for %s. Its scripts no longer run; reload any page already showing it."
	figureNothingToRevoke  = "%s had no approval; nothing changed."
)

// figureRefusalReason is the parenthesis in figureRefusedFmt.
func figureRefusalReason(in *figure.Inspection) string {
	switch in.Refusal {
	case figure.TooLarge:
		return "it is over the 1 MiB limit"
	case figure.NotText, figure.UTF16:
		return "it is not saved as UTF-8"
	default:
		return "it contains " + in.Detail
	}
}

// figureIO is what an approval command reads from and writes to, and whether
// a person is on the other end of both.
type figureIO struct {
	in          io.Reader
	out         io.Writer
	errOut      io.Writer
	interactive bool
	// shown is the first failure to write to out. Nothing is approved once
	// the person may not have seen what they are approving.
	shown error
}

// sayf writes to out and remembers whether the person could read it.
func (stdio *figureIO) sayf(format string, args ...any) {
	if stdio.shown == nil {
		_, stdio.shown = fmt.Fprintf(stdio.out, format, args...)
	}
}

// faultf reports on errOut. A command that is already failing has no second
// channel to report a failure of this one on.
func (stdio *figureIO) faultf(format string, args ...any) {
	_, _ = fmt.Fprintf(stdio.errOut, "yomihon: "+format+"\n", args...) //nolint:errcheck // an already-failing command has no second channel for a stderr failure
}

// runFigureCommand answers approve-figure and revoke-figure. It exits 0 when
// it did what was asked, 1 when it declined — a refused figure, one with
// nothing to approve, a confirmation that did not match — and 2 when it could
// not run: bad arguments, no terminal, an unreadable vault or record.
func runFigureCommand(ctx context.Context, command string, args []string, stdio *figureIO, configDir string, now func() time.Time) int {
	root, rel, err := figureArgs(command, args)
	if err != nil {
		stdio.faultf("%v", err)
		return 2
	}
	// Asked before anything is opened or read, so a program that is not a
	// person learns nothing about the figure by trying.
	if command == approveFigureCommand && !stdio.interactive {
		stdio.faultf("%s", figureNeedsTerminal)
		return 2
	}
	if configDir == "" {
		stdio.faultf("%s", figureNoConfigDir)
		return 2
	}
	source, err := vault.Open(root)
	if err != nil {
		stdio.faultf("%v", err)
		return 2
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			stdio.faultf("close vault: %v", closeErr)
		}
	}()
	// The same namespace the reading server keeps its marks under: the
	// resolved root, so two spellings of one folder are one vault.
	marks, err := mark.New(configDir, source.Name())
	if err != nil {
		stdio.faultf("%v", err)
		return 2
	}
	if command == revokeFigureCommand {
		return revokeFigure(marks, rel, stdio)
	}
	return approveFigure(ctx, source, marks, rel, stdio, now)
}

// figureArgs reads [--root <dir>] <path>. The path is vault-relative, written
// with slashes, and composed to NFC the way the document route composes the
// address it is asked for.
func figureArgs(command string, args []string) (root, rel string, err error) {
	usage := fmt.Errorf(figureUsageFmt, command)
	var positionals, roots []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--root" && i+1 < len(args):
			roots = append(roots, args[i+1])
			i++
		case strings.HasPrefix(arg, "--root="):
			roots = append(roots, strings.TrimPrefix(arg, "--root="))
		case strings.HasPrefix(arg, "-"):
			return "", "", usage
		default:
			positionals = append(positionals, arg)
		}
	}
	root = "."
	if len(roots) > 0 {
		root = roots[0]
	}
	if len(positionals) != 1 || len(roots) > 1 || root == "" {
		return "", "", usage
	}
	rel = vault.NormalizeNFC(filepath.ToSlash(positionals[0]))
	if !fs.ValidPath(rel) || !figure.Is(rel) {
		return "", "", fmt.Errorf(figureNotAFigure, positionals[0])
	}
	return root, rel, nil
}

// approveFigure finds the figure the way the reading server does, reads it
// once, shows the person what those bytes hold, and pins their digest only if
// the person types its opening digits back. The digest recorded is the digest
// of the bytes shown, so a file changed after the read is not what was
// approved and does not run.
func approveFigure(ctx context.Context, source *vault.Reader, marks *mark.File, rel string, stdio *figureIO, now func() time.Time) int {
	data, err := readServedFigure(ctx, source, rel)
	if err != nil {
		stdio.faultf("%v", err)
		return 2
	}
	inspection := figure.Inspect(data)
	if inspection.Refusal != figure.Served {
		stdio.faultf(figureRefusedFmt, rel, figureRefusalReason(&inspection))
		return 1
	}
	if !inspection.Scripted() {
		stdio.sayf(figureNoScriptFmt+"\n", rel)
		return 1
	}
	digest := hex.EncodeToString(inspection.Sum[:])
	report := figure.Describe(data, &inspection)
	stdio.sayf(figureReportFmt, rel, report.Size, digest, report.Scripts)
	stdio.list(figureReportAddresses, report.Addresses)
	stdio.list(figureReportNames, report.Names)
	stdio.sayf("%s\n\n", figureReportDisclaimer)
	stdio.scripts(inspection.Scripts)
	stdio.sayf(figureConfirmPromptFmt, confirmDigits)
	if stdio.shown != nil {
		stdio.faultf("show the figure to approve: %v; nothing was approved", stdio.shown)
		return 2
	}
	answer, err := bufio.NewReader(stdio.in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		stdio.faultf("read the confirmation: %v", err)
		return 2
	}
	if !strings.EqualFold(strings.TrimSpace(answer), digest[:confirmDigits]) {
		stdio.sayf("%s\n", figureNotApproved)
		return 1
	}
	if err := marks.ApproveFigure(rel, inspection.Sum, now()); err != nil {
		stdio.faultf("%v", err)
		return 2
	}
	stdio.sayf(figureApprovedFmt+"\n", rel, digest[:confirmDigits])
	return 0
}

func revokeFigure(marks *mark.File, rel string, stdio *figureIO) int {
	revoked, err := marks.RevokeFigure(rel)
	if err != nil {
		stdio.faultf("%v", err)
		return 2
	}
	if revoked {
		stdio.sayf(figureRevokedFmt+"\n", rel)
	} else {
		stdio.sayf(figureNothingToRevoke+"\n", rel)
	}
	return 0
}

// readServedFigure selects rel exactly as the document route does — a file
// the vault's scan lists, under the contract's scan boundary, re-observed
// through the pinned root so a symlink or a replaced directory is refused —
// and reads up to one byte past the ceiling, so a file over it is known to be.
func readServedFigure(ctx context.Context, source *vault.Reader, rel string) ([]byte, error) {
	// The reading server sets the contract's scan boundary only when the
	// contract loads, and reads on without one when it does not; so does this.
	if contract, err := schema.LoadReader(ctx, source); err == nil {
		source.SetBoundary(contract.ScanBoundary())
	}
	scan, err := source.ScanAvailable(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan the vault: %w", err)
	}
	entry, ok := scan.Entry(rel)
	if !ok {
		return nil, fmt.Errorf(figureNotInVault, rel)
	}
	entry, err = source.Refresh(entry)
	if err != nil {
		return nil, fmt.Errorf(figureNotInVault+": %w", rel, err)
	}
	data, err := source.ReadPrefix(ctx, entry, figure.MaxBytes+1)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	return data, nil
}

// scripts prints every inline script in full, from the bytes being hashed,
// so what the person reads is what the approval pins. A script is shown with
// its control characters and its text-direction overrides as escapes: either
// could make a terminal draw something other than the code that runs.
func (stdio *figureIO) scripts(scripts []string) {
	total := 0
	for i, script := range scripts {
		length := utf8.RuneCountInString(script)
		total += length
		stdio.sayf(figureScriptHeaderFmt+"\n", i+1, len(scripts), length)
		for line := range strings.SplitSeq(visibleScript(script), "\n") {
			stdio.sayf("    | %s\n", line)
		}
	}
	stdio.sayf(figureScriptsFooterFmt+"\n\n", len(scripts), total)
}

// visibleScript escapes what a terminal would act on, reorder or not draw:
// control characters (Unicode category Cc) other than a tab and a line feed,
// format characters (Cf: zero-width and text-direction characters among them),
// and the line and paragraph separators (Zl, Zp). JavaScript ends a line at a
// carriage return and at either separator as well as at a line feed, so after
// those the shown text breaks too: a line the report shows as a comment is one
// line of the script, and what follows the break is shown as the code it is.
func visibleScript(script string) string {
	var b strings.Builder
	for _, r := range script {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r == '\r' || unicode.In(r, unicode.Zl, unicode.Zp):
			b.WriteString(escapedRune(r))
			b.WriteByte('\n')
		case unicode.In(r, unicode.Cc, unicode.Cf):
			b.WriteString(escapedRune(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// escapedRune writes a character as the JavaScript escape that names it.
func escapedRune(r rune) string {
	if r <= 0xFF {
		return fmt.Sprintf(`\x%02x`, r)
	}
	if r <= 0xFFFF {
		return fmt.Sprintf(`\u%04x`, r)
	}
	return fmt.Sprintf(`\u{%x}`, r)
}

// list writes one heading of the report and its items.
func (stdio *figureIO) list(heading string, items []string) {
	stdio.sayf("%s\n", heading)
	if len(items) == 0 {
		stdio.sayf("%s\n", figureReportNone)
		return
	}
	for _, item := range items {
		stdio.sayf("    %s\n", item)
	}
}
