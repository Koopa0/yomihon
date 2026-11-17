package judge

import (
	"context"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// noRulesChecks is what check adds to the sentence a folder with no rules file
// is told first, since the frontmatter rules are the ones that read the rules
// file and so did not run.
const noRulesChecks = egress.NoRulesFile + " The checks that need note types did not run."

// scanAuthority is the contract state that permits one agent-facing judge
// action. Every command opens its gate before reading vault notes and seals its
// payload through the same gate after constructing it.
type scanAuthority struct {
	// gate is the privacy half, nil until the rules file was read or found
	// absent, so a run that never got that far has nothing to recheck when it
	// ends.
	gate *egress.Gate
	// contract is nil for a folder with no rules file, which the contract's own
	// accessors answer for.
	contract *schema.Contract
	// scan is the observation the judge works from, set once it has run. It
	// knows what the walk left out by its own rules, which the contract's
	// boundary cannot.
	scan vault.Scan
}

// domainRoots are the folders the contract declares a note's own folder names
// its knowledge domain under. The reports group by them, and the frontmatter
// rule enforces the same declaration, so both read one key rather than each
// carrying a copy of this vault's layout.
func (a scanAuthority) domainRoots() domainRoots {
	if a.contract == nil {
		return nil
	}
	return a.contract.Domains().Roots()
}

// noRules reports whether this folder carries no rules file, which every
// payload of a command for a person says first.
func (a scanAuthority) noRules() bool {
	return a.gate.WithoutRules()
}

func loadScanAuthority(ctx context.Context, reader *vault.Reader) (scanAuthority, error) {
	gate, err := egress.Open(ctx, reader, LinkTargets)
	if err != nil {
		return scanAuthority{}, err
	}
	return scanAuthority{gate: gate, contract: gate.Rules()}, nil
}

// roles answers which note types take part in path and map behaviour. The
// judge asks the contract rather than naming a type, so a vault that calls
// its courses something else is read the same way the reader reads it.
func (a scanAuthority) roles() schema.NavigationRoles {
	return a.contract.NavigationRoles()
}

// outsideScan reports whether relPath lies where this vault's scan does not
// look: under a dot-prefixed name, a directory the contract excludes, or
// anything the walk left out by the vault's .gitignore or its built-in rules.
func (a scanAuthority) outsideScan(relPath string) bool {
	return a.contract.ScanBoundary().OutsideScan(relPath) || a.scan.OutsideScan(relPath)
}

// readable reports whether a link may be followed to relPath at all: the
// privacy policy lets it out and the scan looks there. It is the gate a path
// passes before its membership is inspected.
func (a scanAuthority) readable(relPath string) bool {
	return a.gate.Allows(relPath) && !a.outsideScan(relPath)
}

// conceptType is the note type this vault files as its distilled ideas, and
// whether the contract declares it. The name comes from the contract layer for
// the same reason the map and path roles do: a vault names its own vocabulary,
// and a judge that spelled the word itself would judge a vault that never uses
// it against a corpus it does not have.
func (a scanAuthority) conceptType() (name string, declared bool) {
	return a.contract.ConceptType()
}

// lessonType is the note type this vault files its course members as, asked of
// the contract layer for the same reason conceptType is: the word belongs to
// the vault's vocabulary, not to this package.
func (a scanAuthority) lessonType() (name string, declared bool) {
	return a.contract.LessonType()
}

// declaresType reports whether the contract lists noteType at all.
func (a scanAuthority) declaresType(noteType string) bool {
	return a.contract.DeclaresType(noteType)
}
