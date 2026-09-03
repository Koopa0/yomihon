package judge

import (
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/vault"
)

// note is one markdown file as the diagnostics see it: its NFC path, whether
// it carries a frontmatter block, whether that block failed to parse, and every
// frontmatter key with its raw value. The typed fields below keep only genuine
// string values, because a graph reference is a name and a name is a string;
// the body references are extracted whatever the frontmatter's state.
type note struct {
	path           string
	noFrontmatter  bool
	badFrontmatter bool
	frontmatter    map[string]fmValue

	title      string
	titleEn    string
	aliases    []string
	noteType   string
	domain     string
	status     string
	sourceKind string
	slug       string
	basedOn    []string
	related    []string

	wikilinks    []wikiLink
	pathRefs     []pathRef
	plannedNames []string

	// sectionAnchors and blockAnchorLines are what this note's body answers a
	// link fragment with: the folded ids of every heading a reader could be
	// sent to, and the folded text of every line that could carry a "^name"
	// block address, collected the way the reading page collects them.
	sectionAnchors   map[string]bool
	blockAnchorLines []string

	// sequence is the note's declared course structure, read by the one grammar
	// navigation reads, so what a course lists is one answer rather than two
	// readings agreeing by luck. It is parsed for every note.
	sequence sequence.Document
}

// fmValue is a raw frontmatter value: a scalar kept as the exact text the
// author wrote, or a list of such texts. The distinction is load-bearing — an
// enum check reads a scalar and skips a list, and a required-field check treats
// an empty scalar or an empty list as absent.
type fmValue struct {
	scalar         string
	list           []string
	stringList     []string
	isList         bool
	scalarIsString bool
}

// asScalar reports the scalar text, or false when the value is a list.
func (v fmValue) asScalar() (string, bool) {
	if v.isList {
		return "", false
	}
	return v.scalar, true
}

// present reports whether the value counts as filled in: a non-empty scalar,
// or a non-empty list.
func (v fmValue) present() bool {
	if v.isList {
		return len(v.list) > 0
	}
	return v.scalar != ""
}

// stringValues returns only genuine YAML strings. Unlike scalar and list,
// these values are not coerced from numbers, booleans, or nulls.
func (v fmValue) stringValues() []string {
	if v.isList {
		return v.stringList
	}
	if v.scalarIsString && v.scalar != "" {
		return []string{v.scalar}
	}
	return nil
}

// parseNote splits a file's leading frontmatter and reads it into the note
// model. A file with no frontmatter block is legal. A block that is present but
// does not parse — or that a stricter reading rejects, such as a mapping with a
// repeated key — yields a note flagged bad, reported as one fault rather than a
// cascade of "field missing" for fields sitting above it.
func parseNote(rel string, data []byte) note {
	block, found := vault.SplitFrontmatter(data)
	body := string(block.Body)
	n := note{
		path:         rel,
		wikilinks:    extractWikilinks(body, block.BodyStartLine),
		pathRefs:     extractPathRefs(body, block.BodyStartLine),
		plannedNames: extractPlannedNames(body),
		sequence:     sequence.Parse(body, block.BodyStartLine),
	}
	n.sectionAnchors, n.blockAnchorLines = anchorSurface(body)
	if !found {
		n.noFrontmatter = true
		return n
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(block.Content, &doc); err != nil {
		n.badFrontmatter = true
		return n
	}
	if hasDuplicateKey(&doc) {
		n.badFrontmatter = true
		return n
	}
	n.frontmatter = buildFrontmatter(&doc)
	readTypedFields(&n, &doc)
	return n
}

// readTypedFields fills the note's typed graph fields from the parsed
// frontmatter. Each reads only a genuine string value: a title, alias or slug
// the core schema would resolve to a number, boolean or null is dropped.
func readTypedFields(n *note, doc *yaml.Node) {
	root := doc
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return
		}
		root = doc.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return
	}
	n.title = strField(root, "title")
	n.titleEn = strField(root, "title_en")
	n.aliases = listField(root, "aliases")
	n.noteType = strField(root, "type")
	n.domain = strField(root, "domain")
	n.status = strField(root, "status")
	n.sourceKind = strField(root, "source_kind")
	n.slug = strField(root, "slug")
	n.basedOn = listField(root, "based_on")
	n.related = listField(root, "related")
}

// mappingValue returns the value node for a top-level key, or false when the
// mapping has none. A duplicate key flags the frontmatter bad before the typed
// fields are read, so the first match is the only match.
func mappingValue(m *yaml.Node, key string) (*yaml.Node, bool) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if k := m.Content[i]; k.Kind == yaml.ScalarNode && k.Value == key {
			return m.Content[i+1], true
		}
	}
	return nil, false
}

// strField reads a key as a single genuine string, returning "" when the key is
// absent or its value is not a string.
func strField(m *yaml.Node, key string) string {
	if v, ok := mappingValue(m, key); ok {
		if s, ok := asString(v); ok {
			return s
		}
	}
	return ""
}

// listField reads a key as a list of genuine strings: a sequence keeps its
// string items and drops the rest; a lone string is a one-element list;
// anything else is empty.
func listField(m *yaml.Node, key string) []string {
	v, ok := mappingValue(m, key)
	if !ok {
		return nil
	}
	v = resolveAlias(v)
	switch v.Kind {
	case yaml.SequenceNode:
		out := make([]string, 0, len(v.Content))
		for _, item := range v.Content {
			if s, ok := asString(item); ok {
				out = append(out, s)
			}
		}
		return out
	case yaml.ScalarNode:
		if s, ok := asString(v); ok {
			return []string{s}
		}
		return nil
	default:
		return nil
	}
}

// asString reports a scalar's text when it reads as a genuine string. A quoted
// or block scalar is always a string; an explicitly tagged scalar is a string
// unless its tag is one of the non-string core tags; an unquoted scalar is a
// string unless the core schema resolves it to a number, boolean or null.
func asString(n *yaml.Node) (string, bool) {
	n = resolveAlias(n)
	if n.Kind != yaml.ScalarNode {
		return "", false
	}
	if !isPlain(n) {
		return n.Value, true
	}
	if n.Style&yaml.TaggedStyle != 0 {
		switch n.Tag {
		case "!!bool", "!!int", "!!float", "!!null":
			return "", false
		default:
			return n.Value, true
		}
	}
	if plainIsString(n.Value) {
		return n.Value, true
	}
	return "", false
}

// hasDuplicateKey reports whether any mapping anywhere in the document repeats
// a key. It is treated as a parse fault rather than resolved to the last value,
// and every mapping is checked, not only the top level.
func hasDuplicateKey(n *yaml.Node) bool {
	if n.Kind == yaml.MappingNode {
		seen := make(map[string]bool, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			if key := n.Content[i]; key.Kind == yaml.ScalarNode {
				if seen[key.Value] {
					return true
				}
				seen[key.Value] = true
			}
		}
	}
	return slices.ContainsFunc(n.Content, hasDuplicateKey)
}

// buildFrontmatter reads a parsed frontmatter document into the flat key/value
// map the checks consume. Only keys that read as strings are kept; a bare
// scalar or list at the top contributes no keys.
func buildFrontmatter(doc *yaml.Node) map[string]fmValue {
	m := make(map[string]fmValue)
	root := doc
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return m
		}
		root = doc.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return m
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if key, ok := keyText(root.Content[i]); ok {
			m[key] = nodeValue(root.Content[i+1])
		}
	}
	return m
}

// keyText reports a mapping key's text when it reads as a string, and false
// when the core schema would resolve it to a number, boolean or null. A quoted
// key is always a string.
func keyText(n *yaml.Node) (string, bool) {
	n = resolveAlias(n)
	if n.Kind != yaml.ScalarNode {
		return "", false
	}
	if !isPlain(n) {
		return n.Value, true
	}
	if n.Style&yaml.TaggedStyle != 0 {
		switch n.Tag {
		case "!!bool", "!!int", "!!float", "!!null":
			return "", false
		default:
			return n.Value, true
		}
	}
	if plainIsString(n.Value) {
		return n.Value, true
	}
	return "", false
}

// plainIsString reports whether an unquoted, untagged scalar reads as a string
// under the core schema: none of an integer in any of its spellings, a boolean
// word, a null word, or a real number.
func plainIsString(v string) bool {
	switch {
	case strings.HasPrefix(v, "0x"):
		if _, err := strconv.ParseInt(v[2:], 16, 64); err == nil {
			return false
		}
	case strings.HasPrefix(v, "0o"):
		if _, err := strconv.ParseInt(v[2:], 8, 64); err == nil {
			return false
		}
	case strings.HasPrefix(v, "+"):
		if _, err := strconv.ParseInt(v[1:], 10, 64); err == nil {
			return false
		}
	}
	switch v {
	case "", "~", "null", "true", "True", "TRUE", "false", "False", "FALSE":
		return false
	}
	if _, err := strconv.ParseInt(v, 10, 64); err == nil {
		return false
	}
	return !isCoreFloat(v)
}

// nodeValue converts one frontmatter value node into an fmValue: a sequence
// becomes a list, a scalar keeps its resolved text, and any other shape
// collapses to an empty scalar. An alias is followed to what it points at.
func nodeValue(n *yaml.Node) fmValue {
	n = resolveAlias(n)
	switch n.Kind {
	case yaml.SequenceNode:
		items := make([]string, 0, len(n.Content))
		stringItems := make([]string, 0, len(n.Content))
		for _, item := range n.Content {
			items = append(items, scalarText(item))
			if value, ok := asString(item); ok && value != "" {
				stringItems = append(stringItems, value)
			}
		}
		return fmValue{list: items, stringList: stringItems, isList: true}
	case yaml.ScalarNode:
		_, scalarIsString := asString(n)
		return fmValue{scalar: scalarText(n), scalarIsString: scalarIsString}
	default:
		return fmValue{}
	}
}

// resolveAlias follows an alias node to the node it refers to, so an aliased
// value reads as the value it points at.
func resolveAlias(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		return n.Alias
	}
	return n
}

// scalarText is a scalar value's text as the frontmatter checks read it. A
// quoted or block scalar is taken verbatim. An unquoted scalar is resolved
// under the YAML core schema — a boolean normalizes to lowercase, an integer to
// its decimal form, a null to an empty string — while every other token,
// yes/no/on/off and any real number included, keeps what the author wrote.
func scalarText(n *yaml.Node) string {
	n = resolveAlias(n)
	if n.Kind != yaml.ScalarNode {
		return ""
	}
	if !isPlain(n) {
		return n.Value
	}
	if n.Style&yaml.TaggedStyle != 0 {
		return resolveTaggedScalar(n.Tag, n.Value)
	}
	return resolvePlainScalar(n.Value)
}

// resolveTaggedScalar maps an unquoted scalar carrying an explicit core-schema
// tag to the string the wire format carries. A boolean tag admits only the six
// boolean words; an integer tag reads a plain decimal integer; a float tag
// keeps a valid real number; a null tag is always empty; any other tag keeps
// the text verbatim. A value the tag cannot read collapses to an empty string.
func resolveTaggedScalar(tag, v string) string {
	switch tag {
	case "!!bool":
		switch v {
		case "true", "True", "TRUE":
			return "true"
		case "false", "False", "FALSE":
			return "false"
		default:
			return ""
		}
	case "!!int":
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return strconv.FormatInt(i, 10)
		}
		return ""
	case "!!float":
		if isCoreFloat(v) {
			return v
		}
		return ""
	case "!!null":
		return ""
	default:
		return v
	}
}

// isCoreFloat reports whether v is a real number under the YAML core schema:
// an infinity or not-a-number spelling, or a decimal real. That real is decimal
// only, so Go's own extensions (1_000, 0x1p4) read as text here.
func isCoreFloat(v string) bool {
	switch v {
	case ".inf", ".Inf", ".INF", "+.inf", "+.Inf", "+.INF",
		"-.inf", "-.Inf", "-.INF", ".nan", ".NaN", ".NAN":
		return true
	}
	hasDigit := false
	for _, r := range v {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '.' || r == 'e' || r == 'E' || r == '+' || r == '-':
			// A decimal-real character.
		default:
			return false
		}
	}
	if !hasDigit {
		return false
	}
	_, err := strconv.ParseFloat(v, 64)
	return err == nil
}

// isPlain reports whether a scalar was written unquoted and unblocked; a
// quoted or block scalar is always its literal text.
func isPlain(n *yaml.Node) bool {
	const quotedOrBlock = yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle | yaml.LiteralStyle | yaml.FoldedStyle
	return n.Style&quotedOrBlock == 0
}

// resolvePlainScalar maps an unquoted scalar's text to the string the wire
// format carries under the YAML core schema: a hexadecimal, octal, signed or
// plain decimal integer collapses to its decimal form, the null and boolean
// words collapse as noted, and anything else — a real number, an out-of-range
// number, plain text — is kept exactly as written.
func resolvePlainScalar(v string) string {
	switch {
	case strings.HasPrefix(v, "0x"):
		if i, err := strconv.ParseInt(v[2:], 16, 64); err == nil {
			return strconv.FormatInt(i, 10)
		}
	case strings.HasPrefix(v, "0o"):
		if i, err := strconv.ParseInt(v[2:], 8, 64); err == nil {
			return strconv.FormatInt(i, 10)
		}
	case strings.HasPrefix(v, "+"):
		if i, err := strconv.ParseInt(v[1:], 10, 64); err == nil {
			return strconv.FormatInt(i, 10)
		}
	}
	switch v {
	case "", "~", "null":
		return ""
	case "true", "True", "TRUE":
		return "true"
	case "false", "False", "FALSE":
		return "false"
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return strconv.FormatInt(i, 10)
	}
	return v
}
