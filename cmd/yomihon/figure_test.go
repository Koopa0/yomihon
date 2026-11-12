package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
)

const approvalFigure = "<!doctype html><p>x</p><script>fetch('https://outside.test/a')</script>\n"

func approvalVault(t *testing.T) (root, configDir string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "vault")
	for name, data := range map[string]string{
		"Figures/moving.html":  approvalFigure,
		"Figures/still.html":   "<p>no script</p>",
		"Figures/refused.html": "<iframe src=//outside.test/></iframe><script>x()</script>",
		"README.md":            "home\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, t.TempDir()
}

func runApproval(t *testing.T, root, configDir string, interactive bool, typed string, args ...string) (exit int, out, errOut string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exit = runFigureCommand(t.Context(), args[0], append([]string{"--root", root}, args[1:]...), &figureIO{
		in: strings.NewReader(typed), out: &stdout, errOut: &stderr, interactive: interactive,
	}, configDir, func() time.Time { return time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC) })
	return exit, stdout.String(), stderr.String()
}

func approvedHere(t *testing.T, root, configDir string) bool {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	marks, err := mark.New(configDir, resolved)
	if err != nil {
		t.Fatal(err)
	}
	ok, _, err := marks.FigureApproved("Figures/moving.html", sha256.Sum256([]byte(approvalFigure)))
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

// TestApprovalNeedsAPersonAtATerminal: a program writing the vault cannot
// approve on the owner's behalf through this command. Without a terminal on
// both ends it refuses before reading anything, and no flag skips that.
func TestApprovalNeedsAPersonAtATerminal(t *testing.T) {
	t.Parallel()
	root, configDir := approvalVault(t)
	digest := sha256.Sum256([]byte(approvalFigure))
	typed := hex.EncodeToString(digest[:])[:8] + "\n"

	exit, out, errOut := runApproval(t, root, configDir, false, typed, approveFigureCommand, "Figures/moving.html")
	if exit != 2 || !strings.Contains(errOut, "needs a person at a terminal") || out != "" {
		t.Errorf("approve without a terminal = %d, %q, %q; want 2 and the refusal alone", exit, out, errOut)
	}
	if approvedHere(t, root, configDir) {
		t.Fatal("a figure was approved without a terminal")
	}
	for _, flag := range []string{"--yes", "-y", "--force"} {
		if exit, _, _ := runApproval(t, root, configDir, false, typed, approveFigureCommand, flag, "Figures/moving.html"); exit != 2 {
			t.Errorf("approve %s = %d, want 2", flag, exit)
		}
	}
}

// TestApprovalPinsTheBytesShown walks the whole conversation: the report is
// shown, the digits typed back are the bytes' own, and the approval names
// exactly those bytes.
func TestApprovalPinsTheBytesShown(t *testing.T) {
	t.Parallel()
	root, configDir := approvalVault(t)
	digest := sha256.Sum256([]byte(approvalFigure))
	hexDigest := hex.EncodeToString(digest[:])

	exit, out, _ := runApproval(t, root, configDir, true, "deadbeef\n", approveFigureCommand, "Figures/moving.html")
	if exit != 1 || !strings.Contains(out, "nothing was approved") {
		t.Errorf("approve with the wrong digits = %d, %q; want 1 and nothing was approved", exit, out)
	}
	if approvedHere(t, root, configDir) {
		t.Fatal("the wrong digits approved the figure")
	}

	exit, out, errOut := runApproval(t, root, configDir, true, hexDigest[:8]+"\n", approveFigureCommand, "Figures/moving.html")
	if exit != 0 {
		t.Fatalf("approve with the right digits = %d, %q, %q", exit, out, errOut)
	}
	for _, want := range []string{
		hexDigest, "Scripts  1 inline", "https://outside.test/a", "fetch", "not a security boundary",
		"Script 1 of 1, 31 characters, exactly as approved:\n    | fetch('https://outside.test/a')\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not say %q:\n%s", want, out)
		}
	}
	if !approvedHere(t, root, configDir) {
		t.Fatal("the right digits did not approve the figure")
	}

	if exit, out, _ := runApproval(t, root, configDir, false, "", revokeFigureCommand, "Figures/moving.html"); exit != 0 || !strings.Contains(out, "Revoked the approval") {
		t.Errorf("revoke = %d, %q", exit, out)
	}
	if approvedHere(t, root, configDir) {
		t.Fatal("a revoked figure is still approved")
	}
	if exit, out, _ := runApproval(t, root, configDir, false, "", revokeFigureCommand, "Figures/moving.html"); exit != 0 || !strings.Contains(out, "had no approval") {
		t.Errorf("revoke of nothing = %d, %q; want 0 and had no approval", exit, out)
	}
}

func TestApprovalDeclinesWhatItCannotPin(t *testing.T) {
	t.Parallel()
	root, configDir := approvalVault(t)
	for _, tc := range []struct {
		path string
		exit int
		say  string
	}{
		{"Figures/refused.html", 1, "<iframe>"},
		{"Figures/still.html", 1, "nothing to approve"},
		{"Figures/refused.html", 1, "nothing to approve"},
		{"Figures/missing.html", 2, "not a file this vault serves"},
		{"README.md", 2, "is not a figure"},
		{"../outside.html", 2, "is not a figure"},
	} {
		exit, out, errOut := runApproval(t, root, configDir, true, "\n", approveFigureCommand, tc.path)
		if exit != tc.exit || !strings.Contains(out+errOut, tc.say) {
			t.Errorf("approve %s = %d, %q; want %d and %q", tc.path, exit, out+errOut, tc.exit, tc.say)
		}
	}
}

// TestTheReportShowsEveryScriptAsItRuns: what a person approves is the text
// they were shown, so the report prints each script whole, and a terminal is
// never handed a character that would redraw or reorder what it shows.
func TestTheReportShowsEveryScriptAsItRuns(t *testing.T) {
	t.Parallel()
	root, configDir := approvalVault(t)
	hidden := "<p>x</p><script>ok()\x1b[2K\x1b[1Ahidden()</script><script>a = '\u202e' + b</script>"
	if err := os.WriteFile(filepath.Join(root, "Figures", "hidden.html"), []byte(hidden), 0o600); err != nil {
		t.Fatal(err)
	}
	_, out, _ := runApproval(t, root, configDir, true, "no\n", approveFigureCommand, "Figures/hidden.html")
	for _, want := range []string{
		"Script 1 of 2, 20 characters, exactly as approved:\n    | ok()\\x1b[2K\\x1b[1Ahidden()\n",
		"Script 2 of 2, 11 characters, exactly as approved:\n    | a = '\\u202e' + b\n",
		"The 2 script(s) above, 31 characters in all",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not hold %q:\n%s", want, out)
		}
	}
	if strings.ContainsAny(out, "\x1b\u202e") {
		t.Errorf("the report hands the terminal a control or direction character:\n%q", out)
	}
}
