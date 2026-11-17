package judge

import (
	"context"
	"errors"
	"regexp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// ErrPrivacyAuthorityUnavailable reports a rules file that exists and could not
// be honoured. The reason is deliberately not carried: a decoder's account of an
// existing rules file's keys would quote vault content back out under exactly
// the policy that is missing. The paragraph a person is owed is the binary's.
var ErrPrivacyAuthorityUnavailable = errors.New("privacy authority unavailable; agent-facing output disabled")

// ErrRulesFileNearby reports a folder with no rules file of its own that sits
// inside a vault or holds one. What that rules file withholds is not known to
// this run, so nothing is read; the repair names no vault content.
var ErrRulesFileNearby = errors.New("a rules file lies above or inside this folder; run yomihon from that vault's own folder, or pass it as --root")

// ErrRulesFileUnknown reports a folder whose surroundings could not be looked at
// to see whether a rules file governs it. It names no path.
var ErrRulesFileUnknown = errors.New("yomihon could not tell whether a rules file lies above or inside this folder, because a folder could not be listed")

// NoRulesFile is what a command says, at the top of what it prints for a
// person, about a folder that has no rules file. It is not a refusal: a folder
// that declared nothing withholds nothing, and the commands read it as they read
// any other.
const NoRulesFile = "No rules file in this folder: nothing is withheld."

// noRulesChecks is what check adds to it, since the frontmatter rules are the
// ones that read the rules file and so did not run.
const noRulesChecks = NoRulesFile + " The checks that need note types did not run."

// RulesVersionError is a contract written for another schema version. It is
// the one contract that could not be used whose cause is stated, because what
// is stated cannot carry the vault's own words: the declared version only when
// it is a plain number, and the line it is on. It is still a refusal of the
// privacy authority, so everything that asks for that refusal gets it.
type RulesVersionError struct {
	// Declared is the version the file declares, empty when it is not a plain
	// number or the file declares none.
	Declared string
	// Line is the line schema_version is written on, zero when it was not found.
	Line int
	// Missing is whether the file declares no version at all.
	Missing bool
	// Supported is the version this binary reads.
	Supported string
}

func (e *RulesVersionError) Error() string { return ErrPrivacyAuthorityUnavailable.Error() }

// Is makes the refusal for another version a case of the refusal for an
// unusable contract.
func (e *RulesVersionError) Is(target error) bool { return target == ErrPrivacyAuthorityUnavailable }

// plainVersion is what a declared version has to look like to be repeated.
var plainVersion = regexp.MustCompile(`^\d{1,6}(?:\.\d{1,6}){0,2}$`)

// RefusalFor is the refusal owed for a rules file that did not load, so every
// command that loads one answers the same way. Only a rules file for another
// schema version says more than that it could not be used, and it is a refusal
// all the same: it is never taken for a folder with no rules file. A folder
// with none never reaches this, because the loader grants it a policy of its own.
func RefusalFor(err error) error {
	if mismatch, ok := errors.AsType[*schema.VersionError](err); ok {
		declared := ""
		if plainVersion.MatchString(mismatch.Declared) {
			declared = mismatch.Declared
		}
		return &RulesVersionError{Declared: declared, Line: mismatch.Line, Missing: mismatch.Missing, Supported: mismatch.Supported}
	}
	// The decoder's own words are deliberately dropped. A parse failure
	// names keys from the contract, and this face exists to hand its output
	// to an agent, so explaining the fault here would send vault content
	// out under exactly the policy that is missing. The operator reads the
	// cause where reading is the point: the server states it on the page
	// and logs it at startup.
	return ErrPrivacyAuthorityUnavailable
}

// scanAuthority is the contract state that permits one agent-facing judge
// action. Every command loads it before reading vault notes and validates the
// same source again after constructing its payload.
type scanAuthority struct {
	// loaded is set once the rules file was read or found absent, so a run that
	// never got that far has nothing to recheck when it ends.
	loaded bool
	// contract is nil for a folder with no rules file, which the contract's own
	// accessors answer for.
	contract *schema.Contract
	privacy  schema.PrivacyPolicy
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
	return a.privacy.WithoutRules()
}

func loadScanAuthority(ctx context.Context, reader *vault.Reader) (scanAuthority, error) {
	contract, privacy, err := schema.LoadForAgents(ctx, reader)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// A stopped command is not a fault in the rules file, and a reader who
		// interrupted it is owed their own words rather than the privacy
		// refusal.
		return scanAuthority{}, err
	}
	if errors.Is(err, schema.ErrRulesFileNearby) {
		return scanAuthority{}, ErrRulesFileNearby
	}
	if errors.Is(err, schema.ErrRulesFileUnknown) {
		return scanAuthority{}, ErrRulesFileUnknown
	}
	if err != nil {
		return scanAuthority{}, RefusalFor(err)
	}
	authority := scanAuthority{
		loaded:   true,
		contract: contract,
		privacy:  privacy,
	}
	if err := authority.validate(); err != nil {
		return scanAuthority{}, err
	}
	return authority, nil
}

// roles answers which note types take part in path and map behaviour. The
// judge asks the contract rather than naming a type, so a vault that calls
// its courses something else is read the same way the reader reads it.
func (a scanAuthority) roles() schema.NavigationRoles {
	return a.contract.NavigationRoles()
}

func (a scanAuthority) validate() error {
	privacy := a.privacy.ValidateSource()
	if privacy.RulesFileNearby() {
		return ErrRulesFileNearby
	}
	if !privacy.Available() {
		return ErrPrivacyAuthorityUnavailable
	}
	return nil
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
	return a.egressAllowed(relPath) && !a.outsideScan(relPath)
}

func (a scanAuthority) egressAllowed(relPath string) bool {
	return a.privacy.EgressAllowed(relPath)
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
