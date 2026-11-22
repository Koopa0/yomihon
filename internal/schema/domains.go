package schema

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
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

// Endpaper is the number, 1 to 12, of the endpaper domain is printed on, or 0
// when it has none: a vault with no domains, a domain the rules file does not
// list, or one beyond the twelve endpapers there are. An endpaper the table
// pinned is used as written, and the rest are given out in the order enums.domain
// lists the domains, so that no endpaper belongs to two.
func (d Domains) Endpaper(domain string) int {
	if d.table == nil {
		return 0
	}
	return d.table.endpapers[NormalizeWord(domain)]
}

// Claim reports how far the [domains] table got. It is unclaimed when the
// rules file wrote none, held when it was read cleanly, and rejected when the
// table pinned an endpaper that is not 1 to 12, one endpaper for two domains, or
// named a domain enums.domain does not list. A rejected table pins no endpaper,
// so every domain takes the one it would have had with no table; the names still
// apply.
func (d Domains) Claim() Claim {
	if d.table == nil {
		return Claim{}
	}
	return d.table.claim
}

// endpaperCount is how many endpapers there are. The design draws twelve and
// reuses none, so the thirteenth domain has no endpaper.
const endpaperCount = 12

// domainFile is one [domains.<name>] table. The endpaper is read as it was
// written, so that a value of the wrong kind is a fault in the table and not a
// refusal of the whole rules file.
type domainFile struct {
	Name     string         `toml:"name"`
	NameZh   string         `toml:"name_zh"`
	NameEn   string         `toml:"name_en"`
	Endpaper toml.Primitive `toml:"endpaper"`
}

type domainNames struct {
	every      string
	byLanguage map[string]string
}

// domainTable is what the rules file's [domains] tables came to, keyed by the
// domain value as enums.domain spells it.
type domainTable struct {
	names     map[string]domainNames
	endpapers map[string]int
	claim     Claim
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
// endpaper with none. One that lists domains has them given endpapers in that
// order whether or not it wrote a table.
//
// A fault does not refuse the rules file: the table only dresses a name and an
// endpaper, and the reader opens everything either way. It is recorded in the
// claim, which `check` reports, and the table then pins nothing.
func deriveDomainTable(order []string, tables map[string]domainFile, meta *toml.MetaData) *domainTable {
	if len(tables) == 0 && len(order) == 0 {
		return nil
	}
	table := &domainTable{names: map[string]domainNames{}, endpapers: assignEndpapers(order, nil)}
	if len(tables) == 0 {
		return table
	}

	var faults []string
	pinned := map[string]int{}
	pinnedBy := map[int]string{}
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
		if fault := pin(pinned, pinnedBy, meta, key, domain, file.Endpaper); fault != "" {
			faults = append(faults, fault)
		}
	}

	if len(faults) > 0 {
		table.claim = Rejected("invalid domains table: " + strings.Join(faults, "; "))
		pinned = nil
	} else {
		table.claim = heldClaim()
	}
	table.endpapers = assignEndpapers(order, pinned)
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

// pin records the endpaper a table asks for, and says what is wrong with it
// when it cannot be had: not a whole number from 1 to 12, or already pinned by
// another table. It says nothing for a table that pins none.
func pin(pinned map[string]int, pinnedBy map[int]string, meta *toml.MetaData, key, domain string, raw toml.Primitive) string {
	if !meta.IsDefined("domains", key, "endpaper") {
		return ""
	}
	var value any
	if err := meta.PrimitiveDecode(raw, &value); err != nil {
		return fmt.Sprintf("domains.%s.endpaper: that value is not 1 to %d", key, endpaperCount)
	}
	number, whole := value.(int64)
	if !whole || number < 1 || number > endpaperCount {
		return fmt.Sprintf("domains.%s.endpaper: %s is not 1 to %d", key, written(value), endpaperCount)
	}
	if other, taken := pinnedBy[int(number)]; taken {
		return fmt.Sprintf("domains.%s.endpaper: %d is also pinned by domains.%s", key, number, other)
	}
	pinnedBy[int(number)] = key
	pinned[domain] = int(number)
	return ""
}

// written is a value of the rules file as a fault quotes it: a string with its
// quotes, a number or a boolean as it reads, and any other kind (a list, a
// date) by what it is. A table never reaches it: its own keys are unknown ones,
// which refuse the rules file as they do anywhere else.
func written(value any) string {
	switch v := value.(type) {
	case string:
		return strconv.Quote(v)
	case int64, float64, bool:
		return fmt.Sprint(v)
	case []any:
		return "a list"
	default:
		return "a date"
	}
}

// assignEndpapers gives each domain its endpaper: the one it pinned, else the one
// at its own place in the list, the first domain's 1 and the second's 2, which is
// the design's rule. A pin takes an endpaper from the domain whose place it is,
// and that domain, and one placed past the twelfth, then take the lowest endpaper
// still free, in the order they are listed. So pinning one domain moves no other
// that kept its own place; a domain left with none once the twelve are given out
// has no entry.
func assignEndpapers(order []string, pinned map[string]int) map[string]int {
	taken := map[int]bool{}
	for _, endpaper := range pinned {
		taken[endpaper] = true
	}
	endpapers := map[string]int{}
	var displaced []string
	for place, domain := range order {
		if _, listed := endpapers[domain]; listed {
			continue
		}
		if endpaper, ok := pinned[domain]; ok {
			endpapers[domain] = endpaper
			continue
		}
		own := place + 1
		if own <= endpaperCount && !taken[own] {
			endpapers[domain] = own
			taken[own] = true
			continue
		}
		displaced = append(displaced, domain)
	}
	free := 1
	for _, domain := range displaced {
		for ; free <= endpaperCount && taken[free]; free++ {
		}
		if free > endpaperCount {
			break
		}
		endpapers[domain] = free
		taken[free] = true
	}
	return endpapers
}
