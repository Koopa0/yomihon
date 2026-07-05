package judge

import (
	"bytes"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// note is one markdown file as the diagnostics see it: its vault-relative
// NFC path, whether it carries a frontmatter block at all, whether that
// block was present but failed to parse, and — when it parsed — every
// frontmatter key with its raw value. The frontmatter checks need the whole
// key set to flag unknown fields and the raw scalar text to validate enums,
// beyond the handful of typed fields the reader surfaces.
//
// The typed fields below and the extracted body references drive the graph
// rules. Unlike the frontmatter map — which keeps a coerced scalar for every
// key so the schema checks can read a value written as a number or boolean —
// the typed fields keep only genuine string values, dropping a title or alias
// that reads as a number, boolean, or null, because a graph reference is a name
// and a name is a string. The body references are extracted whatever the
// frontmatter's state, so a note with broken frontmatter still contributes its
// links.
type note struct {
	path           string
	noFrontmatter  bool
	badFrontmatter bool
	frontmatter    map[string]fmValue

	title                string
	titleEn              string
	aliases              []string
	noteType             string
	domain               string
	status               string
	sourceKind           string
	slug                 string
	basedOn              []string
	related              []string
	evolutionPredecessor string
	evolutionSuccessors  []string

	wikilinks    []wikiLink
	pathRefs     []pathRef
	plannedNames []string
}

// fmValue is a raw frontmatter value: either a scalar kept as the exact text
// the author wrote, or a list of such texts. The distinction is load-bearing
// for the checks — an enum check reads a scalar and skips a value written as
// a list, and a required-field check treats an empty scalar or an empty list
// as absent.
type fmValue struct {
	scalar string
	list   []string
	isList bool
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

// collectNotes walks the vault and parses every markdown file into a note,
// discarding the non-note resources the full walk also finds. A ".md" extension
// is the boundary between a note and a linkable resource; only notes are
// returned. It shares collectVault's single walk, so the scan boundary is the
// resolver's.
func collectNotes(root string) ([]note, error) {
	notes, _, err := collectVault(root)
	return notes, err
}

// parseNote splits a file's leading frontmatter and reads it into the note
// model. A file with no frontmatter block is legal (a raw transcript, scanned
// but never faulted). A block that is present but does not parse — or that a
// stricter reading rejects, such as a mapping with a repeated key, which is
// ill-formed even though this parser tolerates it — yields a note flagged bad,
// which the frontmatter check reports as a single fault rather than a cascade
// of "field missing" for fields that may sit above the fault.
func parseNote(rel string, data []byte) note {
	fm, bodyBytes, bodyLine, found := splitFrontmatter(data)
	body := string(bodyBytes)
	n := note{
		path:         rel,
		wikilinks:    extractWikilinks(body, bodyLine),
		pathRefs:     extractPathRefs(body, bodyLine),
		plannedNames: extractPlannedNames(body),
	}
	if !found {
		n.noFrontmatter = true
		return n
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(fm, &doc); err != nil {
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
// frontmatter. Each reads only a genuine string value (or list of them),
// matching how the vault's linker reads a reference: a title, alias, or slug
// that the core schema would resolve to a number, boolean, or null is dropped,
// because a name is a string.
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
	n.evolutionPredecessor = strField(root, "evolution_predecessor")
	n.evolutionSuccessors = listField(root, "evolution_successors")
}

// mappingValue returns the value node for a top-level key, or false when the
// mapping has no such key. Duplicate keys never reach here — they flag the
// frontmatter bad before the typed fields are read — so the first match is the
// only match.
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

// asString reports a scalar's text when it reads as a genuine string, and false
// otherwise. A quoted or block scalar is always a string; an explicitly tagged
// scalar is a string unless its tag is one of the non-string core tags
// (boolean, integer, float, null), which is how a mapping key is read too; an
// unquoted scalar is a string unless the core schema resolves it to a number,
// boolean, or null.
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

// splitFrontmatter separates a leading frontmatter block from the body. It
// returns the block's bytes, the body's bytes, the 1-based file line the body
// starts on, and whether a block was present. A block is recognized only at the
// very start of the file, opened by a "---" line and closed by the next "---"
// or "..." line; without a closing fence there is no frontmatter and the whole
// file is the body starting at line 1. Both line endings are accepted. The
// returned block keeps the newline that ends its last line — the byte before
// the closing fence — because a trailing newline is part of a block scalar's
// value, and dropping it would change what the author wrote. Line numbers count
// the opening and closing fence lines, so the body of an N-line block starts on
// line N+1. This reads the block on the diagnostics' own terms rather than
// reusing the renderer's split, whose fence handling is shaped for display, not
// for the frozen wire format.
func splitFrontmatter(data []byte) (fm, body []byte, bodyStartLine int, found bool) {
	rest, ok := bytes.CutPrefix(data, []byte("---\n"))
	if !ok {
		if rest, ok = bytes.CutPrefix(data, []byte("---\r\n")); !ok {
			return nil, data, 1, false
		}
	}
	line := 1 // the opening "---"
	for offset := 0; offset < len(rest); {
		raw := rest[offset:]
		advance := len(raw)
		if nl := bytes.IndexByte(raw, '\n'); nl >= 0 {
			advance = nl + 1
		}
		line++
		switch string(bytes.TrimRight(raw[:advance], "\r\n")) {
		case "---", "...":
			return rest[:offset], rest[offset+advance:], line + 1, true
		}
		offset += advance
	}
	return nil, data, 1, false
}

// hasDuplicateKey reports whether any mapping anywhere in the document repeats a
// key. A repeated key is ill-formed YAML that the stricter reference reading
// rejects outright, so it is treated as a parse fault rather than silently
// resolved to the last value. Every mapping is checked — nested in a value, an
// item of a sequence, or the top level alike — to match where the reference
// reading raises the fault.
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
// map the checks consume. Only keys that read as strings are kept, matching how
// the author's mapping is read: a key the core schema would resolve to a
// number, boolean, or null is dropped, while an ordinary word or a token like
// the merge indicator is kept as itself. A non-mapping document (a bare scalar
// or list at the top) contributes no keys.
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
// when the core schema would resolve it to a number, boolean, or null (which
// the reference reading does not admit as a key). A quoted key is always a
// string; an unquoted key is a string unless it resolves to one of those other
// scalar types.
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
// under the core schema — that is, it is none of an integer (in any of the
// hexadecimal, octal, signed, or decimal spellings), a boolean word, a null
// word, or a real number.
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
// becomes a list, a scalar keeps its resolved text, and any other shape (a
// nested mapping) collapses to an empty scalar, so the checks see the same
// shape the resolver does. An alias is followed to the value it points at.
func nodeValue(n *yaml.Node) fmValue {
	n = resolveAlias(n)
	switch n.Kind {
	case yaml.SequenceNode:
		items := make([]string, 0, len(n.Content))
		for _, item := range n.Content {
			items = append(items, scalarText(item))
		}
		return fmValue{list: items, isList: true}
	case yaml.ScalarNode:
		return fmValue{scalar: scalarText(n)}
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
// under the YAML core schema — independent of how the parser happens to tag
// the token — so a boolean normalizes to lowercase, an integer to its decimal
// form, and a null (empty, "~", or "null") to an empty string, while every
// other token (including the 1.1 boolean words yes/no/on/off, an out-of-range
// number, and any real number) keeps exactly what the author wrote.
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
// tag to the string the wire format carries for it. The tag directs the
// reading: a boolean tag admits only the six boolean words and drops anything
// else; an integer tag reads a plain decimal integer and drops the rest; a
// float tag keeps a valid real number as written; a null tag is always empty;
// a string tag (and any tag outside the core schema) keeps the text verbatim.
// A value the tag cannot read collapses to an empty string, which the checks
// treat as absent.
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
// one of the infinity or not-a-number spellings, or a decimal real. The core
// schema's real is decimal only — digits, a point, an e-exponent, and a sign —
// so a token carrying any other character is text, even though Go's float
// parser would accept its own extensions such as digit-separating underscores
// (1_000) and hexadecimal floats (0x1p4).
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
// format carries for it under the YAML core schema: a hexadecimal (0x…),
// octal (0o…), explicitly-signed, or plain decimal integer collapses to its
// decimal form; the null and boolean words collapse as noted; anything the
// core schema does not read as an integer, boolean, or null — a real number,
// a number too large for a signed 64-bit integer, or plain text — is kept
// exactly as written.
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
