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

func endpapers(d schema.Domains, domains ...string) []int {
	out := make([]int, len(domains))
	for i, domain := range domains {
		out[i] = d.Endpaper(domain)
	}
	return out
}

func TestDomainsHaveAnEndpaperInTheOrderTheRulesFileListsThem(t *testing.T) {
	t.Parallel()

	domains := loadFixture(t).Domains()
	got := endpapers(domains, "golang", "japanese", "meta", "rust", "")
	want := []int{1, 2, 3, 0, 0}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Endpaper() = %v, want %v", got, want)
	}
	if domains.Claim().Claimed() {
		t.Error("Claim().Claimed() = true for a rules file with no [domains] table")
	}
	if name := domains.Name("golang", "en"); name != "golang" {
		t.Errorf("Name(golang) = %q, want the domain's own spelling", name)
	}
}

func TestADomainWithNoRulesFileHasNeitherNameNorEndpaper(t *testing.T) {
	t.Parallel()

	var contract *schema.Contract
	domains := contract.Domains()
	if got := domains.Name("golang", "en"); got != "golang" {
		t.Errorf("Name() = %q, want the domain's own spelling", got)
	}
	if got := domains.Endpaper("golang"); got != 0 {
		t.Errorf("Endpaper() = %d, want none", got)
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

func TestAPinnedEndpaperIsUsedAndTheDomainWhosePlaceItWasTakesTheLowestFree(t *testing.T) {
	t.Parallel()

	domains := loadWithDomains(t, `
[domains.meta]
endpaper = 1

[domains.japanese]
endpaper = 3
`)
	got := endpapers(domains, "golang", "japanese", "meta")
	want := []int{2, 3, 1}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Endpaper() = %v, want %v", got, want)
	}
	if !domains.Claim().Trustworthy() {
		t.Errorf("Claim() = %+v, want a table read cleanly", domains.Claim())
	}
}

// domainsListed is a rules file listing the domains d01, d02, … and the
// endpapers they have once the tables in tables are read.
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
		want   []int
	}{
		{"the third pinned past the others", 6, "[domains.d03]\nendpaper = 9\n",
			[]int{1, 2, 9, 4, 5, 6}},
		{"the third pinned, and the ninth's place taken", 10, "[domains.d03]\nendpaper = 9\n",
			[]int{1, 2, 9, 4, 5, 6, 7, 8, 3, 10}},
		{"the fifth pinned to the first's place", 6, "[domains.d05]\nendpaper = 1\n",
			[]int{5, 2, 3, 4, 1, 6}},
		{"two displaced take the lowest free in order", 6, "[domains.d05]\nendpaper = 1\n\n[domains.d06]\nendpaper = 2\n",
			[]int{5, 6, 3, 4, 1, 2}},
		{"nothing pinned", 4, "[domains.d02]\nname = \"Two\"\n",
			[]int{1, 2, 3, 4}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			domains, names := domainsListed(t, tt.count, tt.tables)
			if !domains.Claim().Trustworthy() {
				t.Fatalf("Claim() = %+v, want a table read cleanly", domains.Claim())
			}
			if got := endpapers(domains, names...); fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("Endpaper() = %v, want %v", got, tt.want)
			}
		})
	}
}

// A table with a fault pins nothing, so each domain has the endpaper it would
// have had with no table, and the names are untouched.
func TestATableWithAFaultPinsNothingAndKeepsItsNames(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		tables string
		reason string
	}{
		{"two domains pin one endpaper", `
[domains.golang]
name = "Go"
endpaper = 5
[domains.japanese]
endpaper = 5
`, "domains.japanese.endpaper: 5 is also pinned by domains.golang"},
		{"an endpaper past the last", `
[domains.golang]
name = "Go"
endpaper = 13
`, "domains.golang.endpaper: 13 is not 1 to 12"},
		{"an endpaper before the first", `
[domains.golang]
name = "Go"
endpaper = 0
`, "domains.golang.endpaper: 0 is not 1 to 12"},
		{"an endpaper below nothing", `
[domains.golang]
name = "Go"
endpaper = -3
`, "domains.golang.endpaper: -3 is not 1 to 12"},
		{"an endpaper written as a string", `
[domains.golang]
name = "Go"
endpaper = "3"
`, `domains.golang.endpaper: "3" is not 1 to 12`},
		{"an endpaper written as a file name", `
[domains.golang]
name = "Go"
endpaper = "endpaper-03"
`, `domains.golang.endpaper: "endpaper-03" is not 1 to 12`},
		{"an endpaper written as a fraction", `
[domains.golang]
name = "Go"
endpaper = 2.5
`, "domains.golang.endpaper: 2.5 is not 1 to 12"},
		{"an endpaper written as a boolean", `
[domains.golang]
name = "Go"
endpaper = true
`, "domains.golang.endpaper: true is not 1 to 12"},
		{"an endpaper written as a list", `
[domains.golang]
name = "Go"
endpaper = [1, 2]
`, "domains.golang.endpaper: a list is not 1 to 12"},
		{"a table for a domain the rules file does not list", `
[domains.golang]
name = "Go"
[domains.rust]
endpaper = 4
`, "domains.rust"},
		{"a pin beside a good one", `
[domains.golang]
name = "Go"
endpaper = 9
[domains.meta]
endpaper = 10.5
`, "domains.meta.endpaper"},
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
			got := endpapers(domains, "golang", "japanese", "meta")
			want := []int{1, 2, 3}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("Endpaper() = %v, want %v: a faulty table pins nothing", got, want)
			}
			if name := domains.Name("golang", "en"); name != "Go" {
				t.Errorf("Name(golang) = %q, want the table's name", name)
			}
		})
	}
}

func TestOnlyTwelveDomainsHaveAnEndpaper(t *testing.T) {
	t.Parallel()

	var listed []string
	var want []int
	for i := 1; i <= 13; i++ {
		listed = append(listed, fmt.Sprintf("%q", fmt.Sprintf("d%02d", i)))
		if i <= 12 {
			want = append(want, i)
		}
	}
	want = append(want, 0)
	text := strings.Replace(fixtureText(t), `domain = ["golang", "japanese", "meta"]`, "domain = ["+strings.Join(listed, ", ")+"]", 1)
	domains := loadText(t, text).Domains()
	var asked []string
	for i := 1; i <= 13; i++ {
		asked = append(asked, fmt.Sprintf("d%02d", i))
	}
	if got := endpapers(domains, asked...); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Endpaper() = %v, want %v", got, want)
	}
}

// A thirteenth domain has none even when a pin has freed an endpaper its place
// would have wanted: the twelve are the twelve, and a pin only moves them.
func TestAPinDoesNotGiveAThirteenthDomainAnEndpaper(t *testing.T) {
	t.Parallel()

	domains, names := domainsListed(t, 13, "[domains.d02]\nendpaper = 12\n")
	got := endpapers(domains, names...)
	want := []int{1, 12, 3, 4, 5, 6, 7, 8, 9, 10, 11, 2, 0}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Endpaper() = %v, want %v", got, want)
	}
}

// The domain a note has is the name of a folder, which a filesystem hands back
// in another composition than an editor wrote, so a name found by the bytes
// would be lost.
func TestADomainIsFoundWhateverCompositionItIsSpelledIn(t *testing.T) {
	t.Parallel()

	text := strings.Replace(fixtureText(t), `domain = ["golang", "japanese", "meta"]`, `domain = ["café"]`, 1)
	domains := loadText(t, text+"\n[domains.\"café\"]\nname = \"Café\"\nendpaper = 7\n").Domains()
	for _, spelling := range []string{"café", "café"} {
		if got := domains.Name(spelling, "en"); got != "Café" {
			t.Errorf("Name(%q) = %q, want Café", spelling, got)
		}
		if got := domains.Endpaper(spelling); got != 7 {
			t.Errorf("Endpaper(%q) = %d, want 7", spelling, got)
		}
	}
}

// A name is written only for a language the interface speaks, and the key the
// first design used for an endpaper is not one a table has: either would be
// ignored if it were not refused.
func TestADomainTableRefusesAKeyItDoesNotDefine(t *testing.T) {
	t.Parallel()

	for _, key := range []string{`colour = "red"`, `name_fr = "Aller"`, `pattern = "endpaper-03"`} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "vault-schema.toml")
			text := fixtureText(t) + "\n[domains.golang]\n" + key + "\n"
			if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := schema.LoadFile(path); err == nil || !strings.Contains(err.Error(), "unknown keys") {
				t.Fatalf("LoadFile() error = %v, want the unknown key refused as everywhere else", err)
			}
		})
	}
}
