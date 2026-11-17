package schema_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// The fixture lists three domains, golang, japanese and meta, in that order.
func fixtureText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read the fixture: %v", err)
	}
	return string(data)
}

func loadWithDomains(t *testing.T, tables string) schema.Domains {
	t.Helper()
	return loadText(t, fixtureText(t)+"\n"+tables).Domains()
}

func patterns(d schema.Domains, domains ...string) []string {
	out := make([]string, len(domains))
	for i, domain := range domains {
		out[i] = d.Pattern(domain)
	}
	return out
}

func TestDomainsHaveAnEndpaperInTheOrderTheRulesFileListsThem(t *testing.T) {
	t.Parallel()

	domains := loadFixture(t).Domains()
	got := patterns(domains, "golang", "japanese", "meta", "rust", "")
	want := []string{"endpaper-01", "endpaper-02", "endpaper-03", "", ""}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Pattern() = %v, want %v", got, want)
	}
	if domains.Claim().Claimed() {
		t.Error("Claim().Claimed() = true for a rules file with no [domains] table")
	}
	if name := domains.Name("golang", "en"); name != "golang" {
		t.Errorf("Name(golang) = %q, want the domain's own spelling", name)
	}
}

func TestADomainWithNoRulesFileHasNeitherNameNorPattern(t *testing.T) {
	t.Parallel()

	var contract *schema.Contract
	domains := contract.Domains()
	if got := domains.Name("golang", "en"); got != "golang" {
		t.Errorf("Name() = %q, want the domain's own spelling", got)
	}
	if got := domains.Pattern("golang"); got != "" {
		t.Errorf("Pattern() = %q, want none", got)
	}
}

func TestADomainIsCalledWhatTheRulesFileSaysInTheLanguageAsked(t *testing.T) {
	t.Parallel()

	domains := loadWithDomains(t, `
[domains.golang]
name = "Go"
name_zh = "Go 語言"

[domains.japanese]
name_en = "Japanese"
`)
	for _, tt := range []struct{ domain, lang, want string }{
		{"golang", "zh-Hant", "Go 語言"},
		{"golang", "ZH-hant", "Go 語言"},
		{"golang", "zh", "Go 語言"},
		{"golang", "en", "Go"},
		{"golang", "ja", "Go"},
		{"golang", "", "Go"},
		{"japanese", "en", "Japanese"},
		{"japanese", "zh-Hant", "japanese"},
		{"meta", "en", "meta"},
		{"rust", "en", "rust"},
	} {
		if got := domains.Name(tt.domain, tt.lang); got != tt.want {
			t.Errorf("Name(%q, %q) = %q, want %q", tt.domain, tt.lang, got, tt.want)
		}
	}
	if !domains.Claim().Trustworthy() || domains.Claim().Diagnostic() != "" {
		t.Errorf("Claim() = %+v, want a table read cleanly", domains.Claim())
	}
}

func TestAPinnedPatternIsUsedAndTheRestTakeTheLowestFree(t *testing.T) {
	t.Parallel()

	domains := loadWithDomains(t, `
[domains.meta]
pattern = "endpaper-01"

[domains.japanese]
pattern = "endpaper-03"
`)
	got := patterns(domains, "golang", "japanese", "meta")
	want := []string{"endpaper-02", "endpaper-03", "endpaper-01"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Pattern() = %v, want %v", got, want)
	}
	if !domains.Claim().Trustworthy() {
		t.Errorf("Claim() = %+v, want a table read cleanly", domains.Claim())
	}
}

// A table with a fault pins nothing, so each domain has the pattern it would
// have had with no table, and the names are untouched.
func TestATableWithAFaultPinsNothingAndKeepsItsNames(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		tables string
		reason string
	}{
		{"two domains pin one pattern", `
[domains.golang]
name = "Go"
pattern = "endpaper-05"
[domains.japanese]
pattern = "endpaper-05"
`, "domains.japanese.pattern"},
		{"a pattern past the last", `
[domains.golang]
name = "Go"
pattern = "endpaper-13"
`, "domains.golang.pattern"},
		{"a pattern before the first", `
[domains.golang]
name = "Go"
pattern = "endpaper-00"
`, "domains.golang.pattern"},
		{"a pattern that is not one", `
[domains.golang]
name = "Go"
pattern = "stripes"
`, "domains.golang.pattern"},
		{"a pattern spelled another way", `
[domains.golang]
name = "Go"
pattern = "Endpaper-01"
`, "domains.golang.pattern"},
		{"a table for a domain the rules file does not list", `
[domains.golang]
name = "Go"
[domains.rust]
pattern = "endpaper-04"
`, "domains.rust"},
		{"a pin beside a good one", `
[domains.golang]
name = "Go"
pattern = "endpaper-09"
[domains.meta]
pattern = "endpaper-10x"
`, "domains.meta.pattern"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			domains := loadWithDomains(t, tt.tables)
			claim := domains.Claim()
			if claim.Trustworthy() || claim.Diagnostic() == "" {
				t.Fatalf("Claim() = %+v, want a rejected table with a diagnostic", claim)
			}
			if !strings.Contains(claim.Diagnostic(), tt.reason) {
				t.Errorf("Diagnostic() = %q, want it to name %s", claim.Diagnostic(), tt.reason)
			}
			got := patterns(domains, "golang", "japanese", "meta")
			want := []string{"endpaper-01", "endpaper-02", "endpaper-03"}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("Pattern() = %v, want %v: a faulty table pins nothing", got, want)
			}
			if name := domains.Name("golang", "en"); name != "Go" {
				t.Errorf("Name(golang) = %q, want the table's name", name)
			}
		})
	}
}

func TestOnlyTwelveDomainsHaveAnEndpaper(t *testing.T) {
	t.Parallel()

	var listed, want []string
	for i := 1; i <= 13; i++ {
		listed = append(listed, fmt.Sprintf("%q", fmt.Sprintf("d%02d", i)))
		if i <= 12 {
			want = append(want, fmt.Sprintf("endpaper-%02d", i))
		}
	}
	want = append(want, "")
	text := strings.Replace(fixtureText(t), `domain = ["golang", "japanese", "meta"]`, "domain = ["+strings.Join(listed, ", ")+"]", 1)
	domains := loadText(t, text).Domains()
	var asked []string
	for i := 1; i <= 13; i++ {
		asked = append(asked, fmt.Sprintf("d%02d", i))
	}
	if got := patterns(domains, asked...); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Pattern() = %v, want %v", got, want)
	}
}

// The domain a note has is the name of a folder, which a filesystem hands back
// in another composition than an editor wrote, so a name found by the bytes
// would be lost.
func TestADomainIsFoundWhateverCompositionItIsSpelledIn(t *testing.T) {
	t.Parallel()

	text := strings.Replace(fixtureText(t), `domain = ["golang", "japanese", "meta"]`, `domain = ["café"]`, 1)
	domains := loadText(t, text+"\n[domains.\"café\"]\nname = \"Café\"\npattern = \"endpaper-07\"\n").Domains()
	for _, spelling := range []string{"café", "café"} {
		if got := domains.Name(spelling, "en"); got != "Café" {
			t.Errorf("Name(%q) = %q, want Café", spelling, got)
		}
		if got := domains.Pattern(spelling); got != "endpaper-07" {
			t.Errorf("Pattern(%q) = %q, want endpaper-07", spelling, got)
		}
	}
}

func TestADomainTableRefusesAKeyItDoesNotDefine(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	text := fixtureText(t) + "\n[domains.golang]\ncolour = \"red\"\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := schema.LoadFile(path); err == nil || !strings.Contains(err.Error(), "unknown keys") {
		t.Fatalf("LoadFile() error = %v, want the unknown key refused as everywhere else", err)
	}
}
