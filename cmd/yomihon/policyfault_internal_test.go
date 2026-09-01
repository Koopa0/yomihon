package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// captureLogs collects every record a run writes, so a test can ask what the
// operator was told rather than what a function returned.
type captureLogs struct {
	records []slog.Record
}

func (c *captureLogs) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // hugeParam: slog.Handler fixes this signature; a pointer receiver for the record would not implement the interface
func (c *captureLogs) Handle(_ context.Context, r slog.Record) error {
	c.records = append(c.records, r.Clone())
	return nil
}
func (c *captureLogs) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *captureLogs) WithGroup(string) slog.Handler      { return c }

// said reports every record at or above level whose message or attributes
// contain want.
func (c *captureLogs) said(level slog.Level, want string) []string {
	var out []string
	for i := range c.records {
		r := &c.records[i]
		if r.Level < level {
			continue
		}
		text := r.Message
		r.Attrs(func(a slog.Attr) bool {
			text += " " + a.Key + "=" + a.Value.String()
			return true
		})
		if strings.Contains(text, want) {
			out = append(out, text)
		}
	}
	return out
}

// siteWithPrivacySection writes a vault whose contract carries the given
// privacy declaration, and starts a reading site over it.
func siteWithPrivacySection(t *testing.T, privacySection string) *captureLogs {
	t.Helper()
	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	base, err := os.ReadFile(contractPath) // #nosec G304 -- the fixture this test just wrote, under t.TempDir
	if err != nil {
		t.Fatalf("read the fixture contract: %v", err)
	}
	if err = os.WriteFile(contractPath, []byte(string(base)+"\n"+privacySection), 0o600); err != nil { // #nosec G703 -- fixed contract path under t.TempDir
		t.Fatalf("write the contract: %v", err)
	}
	logs := &captureLogs{}
	site, err := newReadingSite(t.Context(), root, slog.New(logs))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})
	return logs
}

// TestStartupSaysWhyAPolicyCouldNotBeUsed is the half of a promise the program
// makes and did not keep. The adjudication commands refuse to print the reason
// a contract could not be used — printing it would quote the vault back out
// under exactly the policy that is missing — and they name two places the
// operator can read it instead: the page, and the log at startup. The log said
// only that the contract had loaded.
//
// The reason is named in full here because this log is written on the
// operator's own machine, for the operator, about a file they wrote. The
// withholding is a rule about what a program reading the output may be shown,
// and nothing here is that output.
func TestStartupSaysWhyAPolicyCouldNotBeUsed(t *testing.T) {
	t.Parallel()

	rejected := siteWithPrivacySection(t, "[privacy]\nnever_egress_dirs = [\"/\"]\n")
	said := rejected.said(slog.LevelWarn, "never_egress_dirs")
	if len(said) == 0 {
		t.Errorf("startup said nothing about a privacy declaration it refused:\n%v", rejected.records)
	}

	// The control: a contract whose policy is usable earns no warning, or the
	// warning above would be noise the operator learns to scroll past.
	fine := siteWithPrivacySection(t, "[privacy]\nnever_egress_dirs = [\"Private\"]\n")
	if noise := fine.said(slog.LevelWarn, "privacy"); len(noise) != 0 {
		t.Errorf("a usable privacy policy was warned about: %v", noise)
	}
}
