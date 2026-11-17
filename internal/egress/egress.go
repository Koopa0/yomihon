// Package egress is the one place the rules file's privacy half is read for an
// output that leaves this process, and the only way such an output gets out.
//
// Every agent-facing command (check, coverage, exists, marks) opens a Gate
// before it reads a note, asks it before it says a path or a title, has it
// mask the links in a body it prints, and seals what it assembled through it.
// What the rules file's [privacy] section withholds reaches none of them, and
// that is a property of this package rather than of each command remembering
// it: a payload can only be written once the gate has looked at the rules file
// again, and the archlock walk holds every caller to the same extractor and the
// same writer.
package egress

import (
	"context"
	"errors"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// ErrUnavailable reports a rules file that exists and could not be honoured.
// The reason is deliberately not carried: a decoder's account of an existing
// rules file's keys would quote vault content back out under exactly the
// policy that is missing. The paragraph a person is owed is the binary's.
var ErrUnavailable = errors.New("privacy authority unavailable; agent-facing output disabled")

// ErrUndeclared is the refusal for a rules file that exists and leaves the
// [privacy] section out. It prints as ErrUnavailable does and is a case of it;
// it is separate because it is the one refusal a command may explain in full,
// since its repair quotes nothing the vault wrote.
var ErrUndeclared error = undeclared{}

type undeclared struct{}

func (undeclared) Error() string { return ErrUnavailable.Error() }

func (undeclared) Is(target error) bool { return target == ErrUnavailable }

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

// Link is one link a piece of text writes: the name or path it addresses, and
// the bytes it occupies, an embed's "!" and the display text included. Start and
// End are byte offsets into the text it was read from.
type Link struct {
	Target     string
	Start, End int
}

// Targets is the name or path each of links addresses, in order.
func Targets(links []Link) []string {
	targets := make([]string, len(links))
	for i, link := range links {
		targets[i] = link.Target
	}
	return targets
}

// LinkTargets reads the links a piece of text writes, in every syntax a page
// renders as a link, with where each sits. It is a named type, and not a bare
// function at the call site, so that what Open is handed says what it has to
// be: an extractor that reads fewer links than the page does lets a withheld
// name through. The archlock walk holds every production caller to the judge's.
type LinkTargets func(text string) []Link

// Gate is the privacy authority one agent-facing output is made under: the
// rules file as it stood when the command began, and the means to ask it again.
type Gate struct {
	rules   *schema.Contract
	privacy schema.PrivacyPolicy
	links   LinkTargets
	// withheld is the declared directories, read once so that matching a title
	// does not copy the list for each one.
	withheld []string
}

// Open reads the rules file for reader and looks at it for the first time. A
// folder with no rules file gets a gate that withholds nothing, once it has been
// looked at above and inside; every other way a rules file can be missing or
// unusable is a refusal, because that folder may withhold ground this process
// never learned. links is what Spells and Body read links out of text with.
func Open(ctx context.Context, reader *vault.Reader, links LinkTargets) (*Gate, error) {
	if links == nil {
		panic("egress: Open needs the link extractor")
	}
	rules, privacy, err := schema.LoadForAgents(ctx, reader)
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		// A stopped command is not a fault in the rules file, and a reader who
		// interrupted it is owed their own words rather than the privacy
		// refusal.
		return nil, err
	case errors.Is(err, schema.ErrRulesFileNearby):
		return nil, ErrRulesFileNearby
	case errors.Is(err, schema.ErrRulesFileUnknown):
		return nil, ErrRulesFileUnknown
	case err != nil:
		// The decoder's own words are deliberately dropped. A parse failure
		// names keys from the rules file, and this face exists to hand its
		// output to an agent, so explaining the fault here would send vault
		// content out under exactly the policy that is missing. The operator
		// reads the cause where reading is the point: the server states it on
		// the page and logs it at startup.
		return nil, ErrUnavailable
	}
	gate := &Gate{rules: rules, privacy: privacy, links: links, withheld: privacy.WithheldDirs()}
	if err := gate.Check(); err != nil {
		return nil, err
	}
	return gate, nil
}

// Allowed reports whether the rules file of the folder reader reads lets rel be
// said in a place a program reads, for a caller that has no output of its own to
// seal. A folder with no rules file withholds nothing; one whose rules file
// cannot be honoured, or that sits inside or above another's, answers no, as
// Open refuses it, because it may withhold ground this process never learned.
func Allowed(ctx context.Context, reader *vault.Reader, rel string) bool {
	_, privacy, err := schema.LoadForAgents(ctx, reader)
	return err == nil && privacy.EgressAllowed(rel)
}

// Rules is the rules file's other half, read in the same load: the scan
// boundary, the roles and the governance a command needs besides the privacy
// declaration. It is nil for a folder with no rules file, which its own
// accessors answer for.
func (g *Gate) Rules() *schema.Contract {
	return g.rules
}

// Allows reports whether rel may be said in an output: it is a valid
// vault-relative path outside every withheld directory. Permission is positive
// authority, so an unavailable gate allows nothing.
func (g *Gate) Allows(rel string) bool {
	return g.privacy.EgressAllowed(rel)
}

// WithoutRules reports whether this folder carries no rules file, which every
// output for a person says first.
func (g *Gate) WithoutRules() bool {
	return g.privacy.WithoutRules()
}

// Check looks at the rules file again and refuses if it is no longer the one
// this gate was opened on. It is what Seal asks after an output exists, and
// what a run that fails asks before it says what failed.
func (g *Gate) Check() error {
	privacy := g.privacy.ValidateSource()
	switch {
	case privacy.RulesFileNearby():
		return ErrRulesFileNearby
	case privacy.Available():
		return nil
	case privacy.Undeclared():
		return ErrUndeclared
	default:
		return ErrUnavailable
	}
}
