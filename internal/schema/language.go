package schema

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/text/language"
)

const articleLanguageField = "lang"

// ArticleLanguage resolves the optional BCP 47 language tag declared by the
// vault contract for authored note content. Its zero value is safe: without a
// contract declaration every article answers with no tag at all, rather than
// with one inferred from its path, domain, or text.
type ArticleLanguage struct {
	declared bool
}

// ArticleLanguage returns the contract-derived article-language resolver. The
// field is authoritative only when the universal fields.known vocabulary
// declares "lang"; a lesson-only declaration cannot describe every note.
func (c *Contract) ArticleLanguage() ArticleLanguage {
	if c == nil {
		return ArticleLanguage{}
	}
	return ArticleLanguage{declared: slices.Contains(c.definition.Fields.Known, articleLanguageField)}
}

// Resolve returns a canonical BCP 47 tag for one note frontmatter map. A tag
// comes back only where the contract gives the field authority and the note
// declares a value the tag grammar accepts. Missing authority, a missing
// field, and a value the grammar rejects all come back empty — the last with
// an error for diagnostics. The language is never guessed from a note's path,
// domain, or text.
//
// An empty answer means the caller states nothing, so a page's article
// inherits the language its chrome is written in. It is not a claim that the
// note's language is unknown: an unknown-language tag stops inheritance for
// everything under it and buys no reader anything, while inheritance is
// truthful for a vault whose prose is Traditional Chinese and a note written
// in another language says so in one frontmatter line. A note that declares
// the undetermined tag by hand still gets it back, because that is a value its
// author chose.
func (l ArticleLanguage) Resolve(frontmatter map[string]any) (string, error) {
	if !l.declared {
		return "", nil
	}
	value, ok := frontmatter[articleLanguageField]
	if !ok {
		return "", nil
	}
	raw, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("frontmatter %q must be a BCP 47 string", articleLanguageField)
	}
	tag, err := ParseLanguageTag(raw)
	if err != nil {
		return "", err
	}
	return tag, nil
}

// ParseLanguageTag validates and canonicalizes one authored BCP 47 tag.
func ParseLanguageTag(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("frontmatter %q must not be empty", articleLanguageField)
	}
	// x/text deliberately accepts common non-BCP-47 spellings such as
	// underscores. HTML's lang contract is BCP 47, so keep the parser's complete
	// subtag semantics but reject non-ASCII separators and malformed boundaries
	// before parsing.
	if raw[0] == '-' || raw[len(raw)-1] == '-' || strings.Contains(raw, "--") {
		return "", fmt.Errorf("frontmatter %q is not a valid BCP 47 language tag", articleLanguageField)
	}
	for _, r := range raw {
		if r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		return "", fmt.Errorf("frontmatter %q is not a valid BCP 47 language tag", articleLanguageField)
	}
	tag, err := language.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("frontmatter %q is not a valid BCP 47 language tag: %w", articleLanguageField, err)
	}
	return tag.String(), nil
}
