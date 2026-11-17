package schema

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Domains is where a vault's notes get their domain: the first folder below a
// root the contract names under folders.domain_from. A note never writes its
// domain, and a note outside every root has none. The zero value names no root,
// so no note has a domain.
//
// It also answers what a domain is called and which endpaper it is printed on,
// from the optional [domains] table. Both have an answer with no table at all.
type Domains struct {
	roots []string
	table *domainTable
}

// Domains returns the contract's domain roots as a capability a face can hold
// without holding the contract.
func (c *Contract) Domains() Domains {
	if c == nil {
		return Domains{}
	}
	return Domains{roots: slices.Clone(c.folders.DomainFrom), table: c.domainTable}
}

// Of returns the domain of the note at relPath, empty when the path sits
// outside every root or directly under one.
func (d Domains) Of(relPath string) string {
	folder, _ := DomainFolder(d.roots, relPath)
	return folder
}

// Roots returns the declared roots, cloned so a caller cannot change them.
func (d Domains) Roots() []string {
	return slices.Clone(d.roots)
}

// Name is what a face calls domain in the language lang, a BCP 47 tag of which
// only the first subtag matters. A name written for that language wins, then the
// name written for every language, then the domain's own spelling, which is what
// a vault with no [domains] table shows.
func (d Domains) Name(domain, lang string) string {
	if d.table == nil {
		return domain
	}
	names, ok := d.table.names[NormalizeWord(domain)]
	if !ok {
		return domain
	}
	primary, _, _ := strings.Cut(strings.ToLower(lang), "-")
	for _, name := range []string{names.byLanguage[primary], names.every} {
		if name != "" {
			return name
		}
	}
	return domain
}

// Pattern is the endpaper domain is printed on, "endpaper-01" to "endpaper-12",
// or empty when it has none: a vault with no domains, a domain the rules file does
// not list, or one beyond the twelve patterns there are. A pattern the table
// pinned is used as written, and the rest are given out in the order enums.domain
// lists the domains, so that no pattern belongs to two.
func (d Domains) Pattern(domain string) string {
	if d.table == nil {
		return ""
	}
	return d.table.patterns[NormalizeWord(domain)]
}

// Claim reports how far the [domains] table got. It is unclaimed when the
// rules file wrote none, held when it was read cleanly, and rejected when the
// table named a pattern that does not exist, a pattern for two domains, or a
// domain enums.domain does not list. A rejected table pins no pattern, so every
// domain takes the one it would have had with no table; the names still apply.
func (d Domains) Claim() Claim {
	if d.table == nil {
		return Claim{}
	}
	return d.table.claim
}

// patternCount is how many endpapers there are. The design draws twelve and
// reuses none, so the thirteenth domain has no endpaper.
const patternCount = 12

var patternName = regexp.MustCompile(`^endpaper-(0[1-9]|1[012])$`)

type domainFile struct {
	Name    string `toml:"name"`
	NameZh  string `toml:"name_zh"`
	NameEn  string `toml:"name_en"`
	Pattern string `toml:"pattern"`
}

type domainNames struct {
	every      string
	byLanguage map[string]string
}

// domainTable is what the rules file's [domains] tables came to, keyed by the
// domain value as enums.domain spells it.
type domainTable struct {
	names    map[string]domainNames
	patterns map[string]string
	claim    Claim
}

// enumValues is the vocabulary a rules file lists for field, in the order it was
// written, or nil when it lists none.
func (c *Contract) enumValues(field string) []string {
	for _, enum := range c.enums {
		if enum.field == field {
			return enum.values
		}
	}
	return nil
}

// deriveDomainTable reads the [domains] tables against the domains enums.domain
// lists, in the order it lists them. A rules file that lists no domain and wrote
// no table gets nil, which answers every name with its own spelling and every
// pattern with none. One that lists domains has them patterned in that order
// whether or not it wrote a table.
//
// A fault does not refuse the rules file: the table only dresses a name and a
// pattern, and the reader opens everything either way. It is recorded in the
// claim, which `check` reports, and the table then pins nothing.
func deriveDomainTable(order []string, tables map[string]domainFile) *domainTable {
	if len(tables) == 0 && len(order) == 0 {
		return nil
	}
	table := &domainTable{names: map[string]domainNames{}, patterns: assignPatterns(order, nil)}
	if len(tables) == 0 {
		return table
	}

	var faults []string
	pinned := map[string]string{}
	pinnedBy := map[string]string{}
	for _, key := range slices.Sorted(maps.Keys(tables)) {
		file := tables[key]
		domain := NormalizeWord(key)
		if _, seen := table.names[domain]; seen {
			faults = append(faults, fmt.Sprintf("domains.%s: names the same domain as another table", key))
			continue
		}
		if !slices.Contains(order, domain) {
			faults = append(faults, fmt.Sprintf("domains.%s: %q is not listed in enums.domain", key, domain))
		}
		table.names[domain] = namesOf(file)
		if fault := pin(pinned, pinnedBy, key, domain, file.Pattern); fault != "" {
			faults = append(faults, fault)
		}
	}

	if len(faults) > 0 {
		table.claim = Rejected("invalid domains table: " + strings.Join(faults, "; "))
		pinned = nil
	} else {
		table.claim = heldClaim()
	}
	table.patterns = assignPatterns(order, pinned)
	return table
}

// namesOf is what a table calls its domain, with the blank ones left out so
// that a language with no name of its own falls back to the one for every
// language.
func namesOf(file domainFile) domainNames {
	names := domainNames{every: strings.TrimSpace(file.Name), byLanguage: map[string]string{}}
	for language, name := range map[string]string{"zh": file.NameZh, "en": file.NameEn} {
		if name = strings.TrimSpace(name); name != "" {
			names.byLanguage[language] = name
		}
	}
	return names
}

// pin records the pattern a table asks for, and says what is wrong with it
// when it cannot be had: not one of the twelve, or already pinned by another
// table. It says nothing for a table that pins none.
func pin(pinned, pinnedBy map[string]string, key, domain, pattern string) string {
	if pattern == "" {
		return ""
	}
	if !patternName.MatchString(pattern) {
		return fmt.Sprintf("domains.%s.pattern: %s is not one of endpaper-01 to endpaper-12", key, strconv.Quote(pattern))
	}
	if other, taken := pinnedBy[pattern]; taken {
		return fmt.Sprintf("domains.%s.pattern: %s is also pinned by domains.%s", key, pattern, other)
	}
	pinnedBy[pattern] = key
	pinned[domain] = pattern
	return ""
}

// assignPatterns gives each domain its endpaper: the one it pinned, else the one
// at its own place in the list, the first domain's endpaper-01 and the second's
// endpaper-02, which is the design's rule. A pin takes a pattern from the domain
// whose place it is, and that domain, and one placed past the twelfth, then take
// the lowest pattern still free, in the order they are listed. So pinning one
// domain moves no other that kept its own place; a domain left with none once
// the twelve are given out has no entry.
func assignPatterns(order []string, pinned map[string]string) map[string]string {
	taken := map[string]bool{}
	for _, pattern := range pinned {
		taken[pattern] = true
	}
	patterns := map[string]string{}
	var displaced []string
	for place, domain := range order {
		if _, listed := patterns[domain]; listed {
			continue
		}
		if pattern, ok := pinned[domain]; ok {
			patterns[domain] = pattern
			continue
		}
		own := fmt.Sprintf("endpaper-%02d", place+1)
		if place < patternCount && !taken[own] {
			patterns[domain] = own
			taken[own] = true
			continue
		}
		displaced = append(displaced, domain)
	}
	free := 1
	for _, domain := range displaced {
		for ; free <= patternCount && taken[fmt.Sprintf("endpaper-%02d", free)]; free++ {
		}
		if free > patternCount {
			break
		}
		patterns[domain] = fmt.Sprintf("endpaper-%02d", free)
		taken[patterns[domain]] = true
	}
	return patterns
}
