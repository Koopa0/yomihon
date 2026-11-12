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
}

// runFigureCommand answers approve-figure and revoke-figure. It exits 0 when
// it did what was asked, 1 when it declined — a refused figure, one with
// nothing to approve, a confirmation that did not match — and 2 when it could
// not run: bad arguments, no terminal, an unreadable vault or record.
func runFigureCommand(ctx context.Context, command string, args []string, stdio figureIO, configDir string, now func() time.Time) int {
	root, rel, err := figureArgs(command, args)
	if err != nil {
		fmt.Fprintf(stdio.errOut, "yomihon: %v\n", err)
		return 2
	}
	// Asked before anything is opened or read, so a program that is not a
	// person learns nothing about the figure by trying.
	if command == approveFigureCommand && !stdio.interactive {
		fmt.Fprintf(stdio.errOut, "yomihon: %s\n", figureNeedsTerminal)
		return 2
	}
	if configDir == "" {
		fmt.Fprintf(stdio.errOut, "yomihon: %s\n", figureNoConfigDir)
		return 2
	}
	source, err := vault.Open(root)
	if err != nil {
		fmt.Fprintf(stdio.errOut, "yomihon: %v\n", err)
		return 2
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			fmt.Fprintf(stdio.errOut, "yomihon: close vault: %v\n", closeErr)
		}
	}()
	// The same namespace the reading server keeps its marks under: the
	// resolved root, so two spellings of one folder are one vault.
	marks, err := mark.New(configDir, source.Name())
	if err != nil {
		fmt.Fprintf(stdio.errOut, "yomihon: %v\n", err)
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
	var positionals []string
	rootSeen := false
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--root":
			if rootSeen || i+1 >= len(args) {
				return "", "", usage
			}
			rootSeen = true
			root = args[i+1]
			i++
		case strings.HasPrefix(arg, "--root="):
			if rootSeen {
				return "", "", usage
			}
			rootSeen = true
			root = strings.TrimPrefix(arg, "--root=")
		case strings.HasPrefix(arg, "-") && arg != "-":
			return "", "", usage
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 || (rootSeen && root == "") {
		return "", "", usage
	}
	if !rootSeen {
		root = "."
	}
	rel = vault.NormalizeNFC(filepath.ToSlash(positionals[0]))
	if !fs.ValidPath(rel) || rel == "." || !figure.Is(rel) {
		return "", "", fmt.Errorf(figureNotAFigure, positionals[0])
	}
	return root, rel, nil
}

// approveFigure finds the figure the way the reading server does, reads it
// once, shows the person what those bytes hold, and pins their digest only if
// the person types its opening digits back. The digest recorded is the digest
// of the bytes shown, so a file changed after the read is not what was
// approved and does not run.
func approveFigure(ctx context.Context, source *vault.Reader, marks *mark.File, rel string, stdio figureIO, now func() time.Time) int {
	data, err := readServedFigure(ctx, source, rel)
	if err != nil {
		fmt.Fprintf(stdio.errOut, "yomihon: %v\n", err)
		return 2
	}
	inspection := figure.Inspect(data)
	if inspection.Refusal != figure.Served {
		fmt.Fprintf(stdio.errOut, "yomihon: "+figureRefusedFmt+"\n", rel, figureRefusalReason(&inspection))
		return 1
	}
	if !inspection.Scripted() {
		fmt.Fprintf(stdio.out, figureNoScriptFmt+"\n", rel)
		return 1
	}
	digest := hex.EncodeToString(inspection.Sum[:])
	report := figure.Describe(data, &inspection)
	fmt.Fprintf(stdio.out, figureReportFmt, rel, report.Size, digest, report.Scripts)
	printList(stdio.out, figureReportAddresses, report.Addresses)
	printList(stdio.out, figureReportNames, report.Names)
	fmt.Fprintf(stdio.out, "%s\n\n", figureReportDisclaimer)
	fmt.Fprintf(stdio.out, figureConfirmPromptFmt, confirmDigits)
	answer, err := bufio.NewReader(stdio.in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(stdio.errOut, "yomihon: read the confirmation: %v\n", err)
		return 2
	}
	if !strings.EqualFold(strings.TrimSpace(answer), digest[:confirmDigits]) {
		fmt.Fprintln(stdio.out, figureNotApproved)
		return 1
	}
	if err := marks.ApproveFigure(rel, inspection.Sum, now()); err != nil {
		fmt.Fprintf(stdio.errOut, "yomihon: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdio.out, figureApprovedFmt+"\n", rel, digest[:confirmDigits])
	return 0
}

func revokeFigure(marks *mark.File, rel string, stdio figureIO) int {
	revoked, err := marks.RevokeFigure(rel)
	if err != nil {
		fmt.Fprintf(stdio.errOut, "yomihon: %v\n", err)
		return 2
	}
	if revoked {
		fmt.Fprintf(stdio.out, figureRevokedFmt+"\n", rel)
	} else {
		fmt.Fprintf(stdio.out, figureNothingToRevoke+"\n", rel)
	}
	return 0
}

// readServedFigure selects rel exactly as the document route does — a file
// the vault's scan lists, under the contract's scan boundary, re-observed
// through the pinned root so a symlink or a replaced directory is refused —
// and reads up to one byte past the ceiling, so a file over it is known to be.
func readServedFigure(ctx context.Context, source *vault.Reader, rel string) ([]byte, error) {
	contract, err := schema.LoadReader(ctx, source)
	switch {
	case schema.ContractAbsent(err):
	case err != nil:
		return nil, fmt.Errorf("load the vault contract: %w", err)
	default:
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

func printList(out io.Writer, heading string, items []string) {
	fmt.Fprintln(out, heading)
	if len(items) == 0 {
		fmt.Fprintln(out, figureReportNone)
		return
	}
	for _, item := range items {
		fmt.Fprintf(out, "    %s\n", item)
	}
}
