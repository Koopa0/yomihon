package schema

import (
	"fmt"

	"github.com/koopa0/yomihon/internal/vault"
)

// JournalDir is the vault directory whose markdown files populate the journal
// shelf. The zero value is unclaimed: nothing was declared, the projection is
// empty, and that is the true answer for a vault that has no journal.
type JournalDir struct {
	dir   string
	claim Claim
}

// Claim reports how far the journal declaration got.
func (j JournalDir) Claim() Claim {
	return j.claim
}

// Available reports whether the contract declared a valid journal directory.
func (j JournalDir) Available() bool {
	return j.claim.held()
}

// Trustworthy reports whether a projection over the journal directory may be
// answered: true when it was read cleanly or never declared, false when a
// declaration was made and could not be honoured.
func (j JournalDir) Trustworthy() bool {
	return j.claim.Trustworthy()
}

// Diagnostic explains why the journal directory could not be honoured. It is
// empty when the directory was read cleanly and empty when nothing declared one.
func (j JournalDir) Diagnostic() string {
	return j.claim.Diagnostic()
}

// Contains reports whether rel is equal to or below the declared journal
// directory, comparing whole components through the folded identity every vault
// directory scope shares. An unclaimed or unresolved directory contains
// nothing: inventing a journal folder a vault never named is a rule its owner
// did not write.
func (j JournalDir) Contains(rel string) bool {
	if !j.Available() {
		return false
	}
	return pathHasFoldedPrefix(vault.NormalizeNFC(rel), j.dir)
}

func deriveJournalDir(section *navigationSection, journalDefined bool) JournalDir {
	if section == nil || !journalDefined {
		return JournalDir{}
	}
	normalized, ok := normalizeDeclaredDir(section.JournalDir)
	if !ok {
		return invalidJournalDir(section.JournalDir)
	}
	return JournalDir{dir: normalized, claim: heldClaim()}
}

func invalidJournalDir(value string) JournalDir {
	return JournalDir{claim: Rejected(fmt.Sprintf("invalid journal directory: journal_dir contains %q", value))}
}

func journalTypeError(key string) JournalDir {
	return JournalDir{claim: Rejected(fmt.Sprintf("invalid journal directory: key %q has incompatible TOML type", key))}
}
