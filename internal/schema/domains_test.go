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

func TestAPinnedPatternIsUsedAndTheDomainWhosePlaceItWasTakesTheLowestFree(t *testing.T) {
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

// domainsListed is a rules file listing the domains d01, d02, … and the
// patterns they have once the tables in tables are read.
func domainsListed(t *testing.T, count int, tables string) (domains schema.Domains, names []string) {
	t.Helper()
	var listed []string
	for i := 1; i <= count; i++ {
		names = append(names, fmt.Sprintf("d%02d", i))
		listed = append(listed, fmt.Sprintf("%q", names[i-1]))
	}
	text := strings.Replace(fixtureText(t), `domain = ["golang", "japanese", "meta"]`, "domain = ["+strings.Join(listed, ", ")+"]", 1)
	return loadText(t, text+"\n"+tables).Domains(), names
}

// A pin moves no domain but the one whose place it took: a reader would see the
// cover of a course they did not touch change if it did.
func TestPinningOneDomainMovesNoOtherThatKeptItsPlace(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		count  int
		tables string
		want   []string
	}{
		{"the third pinned past the others", 6, "[domains.d03]\npattern = \"endpaper-09\"\n",
			[]string{"endpaper-01", "endpaper-02", "endpaper-09", "endpaper-04", "endpaper-05", "endpaper-06"}},
		{"the third pinned, and the ninth's place taken", 10, "[domains.d03]\npattern = \"endpaper-09\"\n",
			[]string{"endpaper-01", "endpaper-02", "endpaper-09", "endpaper-04", "endpaper-05", "endpaper-06", "endpaper-07", "endpaper-08", "endpaper-03", "endpaper-10"}},
		{"the fifth pinned to the first's place", 6, "[domains.d05]\npattern = \"endpaper-01\"\n",
			[]string{"endpaper-05", "endpaper-02", "endpaper-03", "endpaper-04", "endpaper-01", "endpaper-06"}},
		{"two displaced take the lowest free in order", 6, "[domains.d05]\npattern = \"endpaper-01\"\n\n[domains.d06]\npattern = \"endpaper-02\"\n",
			[]string{"endpaper-05", "endpaper-06", "endpaper-03", "endpaper-04", "endpaper-01", "endpaper-02"}},
		{"nothing pinned", 4, "[domains.d02]\nname = \"Two\"\n",
			[]string{"endpaper-01", "endpaper-02", "endpaper-03", "endpaper-04"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			domains, names := domainsListed(t, tt.count, tt.tables)
			if !domains.Claim().Trustworthy() {
				t.Fatalf("Claim() = %+v, want a table read cleanly", domains.Claim())
			}
			if got := patterns(domains, names...); fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("Pattern() = %v, want %v", got, tt.want)
			}
		})
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

// A thirteenth domain has none even when a pin has freed a pattern its place
// would have wanted: the twelve are the twelve, and a pin only moves them.
func TestAPinDoesNotGiveAThirteenthDomainAnEndpaper(t *testing.T) {
	t.Parallel()

	domains, names := domainsListed(t, 13, "[domains.d02]\npattern = \"endpaper-12\"\n")
	got := patterns(domains, names...)
	want := []string{"endpaper-01", "endpaper-12", "endpaper-03", "endpaper-04", "endpaper-05", "endpaper-06", "endpaper-07", "endpaper-08", "endpaper-09", "endpaper-10", "endpaper-11", "endpaper-02", ""}
	if fmt.Sprint(got) != fmt.Sprint(want) {
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
