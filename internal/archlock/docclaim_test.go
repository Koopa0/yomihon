package archlock

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// oldVaultClaims are the sentences the documentation used while yomihon wrote
// nothing into the vault. The reader's marks now live in one machine file there
// (reader-notes/.yomihon/marks/), so each of these is false, in English and in
// Traditional Chinese. They match the phrasings the docs actually used, not the
// idea: a rewrite that says the same thing in other words is for the reviewer,
// and a guard that tried to read meaning would be a second prose reader.
var oldVaultClaims = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bnever writes? (?:in)?to (?:a |the |your )?(?:vault|notes)\b`),
	regexp.MustCompile(`(?i)\b(?:does not|doesn't|do not|don't) write (?:in)?to (?:a |the |your )?vault\b`),
	regexp.MustCompile(`(?i)\bwrites? nothing (?:in)?to (?:a |the |your )?vault\b`),
	regexp.MustCompile(`(?i)\bwrites nothing into it\b`),
	regexp.MustCompile(`(?i)\bno (?:endpoint|command) writes into the vault\b`),
	regexp.MustCompile(`(?i)\bnothing writes into a vault\b`),
	regexp.MustCompile(`(?i)\bread-only toward the vault\b`),
	regexp.MustCompile(`(?i)\bnever written: yomihon opens no vault file for writing\b`),
	regexp.MustCompile(`(?i)\bnever writes, renames or deletes one\b`),
	regexp.MustCompile(`(?:從不|絕不|不會|不)寫(?:入|進|回)\s*(?:書庫|vault)`),
	regexp.MustCompile(`對\s*(?:書庫|vault)\s*唯讀`),
}

// allowedClaimQuotes lists the documents and lines that quote an old claim on
// purpose, as "path:text of the line". It is empty: a document that needs to
// say what yomihon used to promise says it without the old sentence.
var allowedClaimQuotes = map[string]bool{}

// trackedDocs lists the tracked prose that describes what yomihon does to a
// vault. docs/ also holds the maintainer's untracked working material, so the
// list comes from git rather than from a walk.
func trackedDocs(t *testing.T) []string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git is not available: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), "git", "-c", "core.quotepath=off", "ls-files", "-z",
		"--", "README*.md", "ARCHITECTURE*.md", "CONTRIBUTING*.md", ".github/SECURITY*.md",
		"docs", "skills/yomihon", "examples")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("no git work tree to list: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	var docs []string
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" && path.Ext(p) == ".md" {
			docs = append(docs, p)
		}
	}
	if len(docs) == 0 {
		t.Fatal("no tracked document was listed, so the check over them passes for the wrong reason")
	}
	return docs
}

// staleVaultClaims returns the lines of text that carry an old claim.
func staleVaultClaims(file, text string) []site {
	var found []site
	for i, line := range strings.Split(text, "\n") {
		for _, claim := range oldVaultClaims {
			if claim.MatchString(line) && !allowedClaimQuotes[file+":"+strings.TrimSpace(line)] {
				found = append(found, site{path: file, line: i + 1, text: strings.TrimSpace(line)})
				break
			}
		}
	}
	return found
}

// TestNoDocSaysYomihonNeverWritesIntoTheVault keeps the documentation from
// promising what is no longer true. yomihon writes one thing into a vault, the
// reader's marks in reader-notes/.yomihon/marks/, and nothing else; a page that
// says it writes nothing there sends a reader looking for the wrong evidence
// when a mark appears in `git status`.
func TestNoDocSaysYomihonNeverWritesIntoTheVault(t *testing.T) {
	t.Parallel()

	var found []site
	for _, file := range trackedDocs(t) {
		data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(file))) // #nosec G304 -- a path git listed under the repository root
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		found = append(found, staleVaultClaims(file, string(data))...)
	}
	report(t, "yomihon writes the reader's marks into reader-notes/.yomihon/marks/; say what it writes instead of saying it writes nothing", found)
}

// TestTheOldVaultClaimsAreCaughtInBothLanguages locks the patterns. The check
// above is green when it finds nothing and green when its patterns can no
// longer match anything, so each old phrasing is shown to be caught, and the
// sentences that stay true are shown to pass.
func TestTheOldVaultClaimsAreCaughtInBothLanguages(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		line string
		want bool
	}{
		{"never writes into the vault", "yomihon never writes into the vault.", true},
		{"never writes to your vault", "It never writes to your vault.", true},
		{"does not write into the vault", "yomihon does not write into the vault", true},
		{"writes nothing into the vault", "its server listens on 127.0.0.1 and writes nothing into the vault", true},
		{"reads and writes nothing into it", "it reads a vault and writes nothing into it", true},
		{"no endpoint writes", "No endpoint and no command writes into the vault.", true},
		{"read-only toward the vault", "## 6. Read-only toward the vault", true},
		{"the heading of the Chinese draft note", "## 不會寫回書庫", true},
		{"never writes in Chinese, library", "yomihon 從不寫入書庫。", true},
		{"never writes in Chinese, vault", "閱讀標記另外儲存，從不寫入 vault。", true},
		{"read-only in Chinese", "## 6. 對 vault 唯讀", true},
		{"the draft field is still never written", "yomihon 絕不寫這個欄位：它只在作者的編輯器裡改。", false},
		{"a note is still never edited", "yomihon never edits, creates or deletes a note.", false},
		{"the one thing it writes", "yomihon writes one file per computer under reader-notes/.yomihon/marks/.", false},
		{"commands that read only", "`check` opens the vault as an independent read-only action.", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := len(staleVaultClaims("x.md", tt.line)) == 1; got != tt.want {
				t.Errorf("claim in %q = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}
