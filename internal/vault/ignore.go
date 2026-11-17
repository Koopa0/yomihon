package vault

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

// ignoreFile is the one rule file a scan reads, at the vault root. It is a
// dot-named file, so the walk never lists it as a note; the scan opens it by
// name through the same pinned root every other read goes through.
const ignoreFile = ".gitignore"

// maxIgnoreBytes bounds how much of the vault's .gitignore a scan reads. A rule
// file past it is somebody's generated output rather than a list of folders to
// leave out, and the rules read before the cut still apply.
const maxIgnoreBytes = 1 << 20

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// builtinSkippedDir is the one folder name every scan leaves out wherever it
// stands: a package manager's installed copy of other people's documents.
const builtinSkippedDir = "node_modules"

// venvMarker is the file a Python virtual environment's root carries. The
// folder holding it is skipped whole, whatever it is called.
const venvMarker = "pyvenv.cfg"

// ignoreRules is the vault root's .gitignore, read the way git reads it for the
// root directory: the last matching rule decides, and a leading "!" re-includes.
// A rule file nested in a subfolder is not read.
type ignoreRules struct {
	rules []ignoreRule
}

type ignoreRule struct {
	match   *regexp.Regexp
	negate  bool
	dirOnly bool
}

// readIgnoreRules reads the vault root's .gitignore. It goes through the pinned
// root with the identity checks every note read has, so a .gitignore that is a
// symbolic link is not followed: the vault has no rules rather than rules from
// wherever the link points. A vault without the file has no rules either.
func (r *Reader) readIgnoreRules(ctx context.Context) (ignoreRules, error) {
	entry, err := r.observe(ignoreFile)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, ErrSymbolicLink), errors.Is(err, ErrNotRegular):
		return ignoreRules{}, nil
	default:
		return ignoreRules{}, err
	}
	data, err := r.readPrefix(ctx, entry, maxIgnoreBytes, nil)
	if err != nil {
		return ignoreRules{}, err
	}
	if len(data) >= maxIgnoreBytes {
		// A last line cut in the middle would read as a shorter, broader rule.
		if end := bytes.LastIndexByte(data, '\n'); end >= 0 {
			data = data[:end]
		} else {
			data = nil
		}
	}
	return parseIgnoreRules(bytes.TrimPrefix(data, utf8BOM)), nil
}

// passesOver reports whether the walk does not index the entry at raw, because
// the contract's boundary excludes its folder or because a rule of the walk's
// own leaves it out. Only the second is remembered, in leftOut: the boundary
// answers for itself. The vault root is never passed over.
func (w *sourceWalk) passesOver(raw, canonical string, d fs.DirEntry) bool {
	if d.IsDir() && w.boundary.excludes(canonical) {
		return true
	}
	if !w.leavesOut(raw, canonical, d) {
		return false
	}
	w.leftOut[canonical] = struct{}{}
	return true
}

// leavesOut reports whether a rule of the walk's own names the entry at raw: a
// folder called node_modules, a Python virtual environment, or what the vault's
// .gitignore lists.
func (w *sourceWalk) leavesOut(raw, canonical string, d fs.DirEntry) bool {
	if d.IsDir() && (d.Name() == builtinSkippedDir || w.isVirtualEnv(raw)) {
		return true
	}
	return w.ignore.ignored(canonical, d.IsDir())
}

// isVirtualEnv reports whether the folder at raw carries a pyvenv.cfg, asked
// through the pinned root without following a link.
func (w *sourceWalk) isVirtualEnv(raw string) bool {
	info, err := w.root.Lstat(path.Join(raw, venvMarker))
	return err == nil && info.Mode().IsRegular()
}

// parseIgnoreRules reads the lines of a .gitignore. A line that cannot be
// compiled is dropped, the way git drops one it cannot parse, so a stray
// bracket never costs the reader their notes.
func parseIgnoreRules(data []byte) ignoreRules {
	var rules ignoreRules
	for line := range strings.Lines(strings.ReplaceAll(string(data), "\r\n", "\n")) {
		if rule, ok := parseIgnoreRule(strings.TrimSuffix(line, "\n")); ok {
			rules.rules = append(rules.rules, rule)
		}
	}
	return rules
}

func parseIgnoreRule(line string) (ignoreRule, bool) {
	line = trimIgnoreSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}
	var rule ignoreRule
	if rest, ok := strings.CutPrefix(line, "!"); ok {
		rule.negate, line = true, rest
	}
	if rest, ok := strings.CutSuffix(line, "/"); ok {
		rule.dirOnly, line = true, rest
	}
	// A slash anywhere but the end ties the rule to the vault root; without
	// one the rule matches a name at any depth.
	anchored := strings.Contains(line, "/")
	line = strings.TrimPrefix(line, "/")
	if line == "" {
		return ignoreRule{}, false
	}
	expression := globExpression(NormalizeNFC(line), anchored)
	compiled, err := regexp.Compile(expression)
	if err != nil {
		return ignoreRule{}, false
	}
	rule.match = compiled
	return rule, true
}

// trimIgnoreSpace drops the trailing spaces git ignores, keeping one a
// backslash escapes.
func trimIgnoreSpace(line string) string {
	end := len(line)
	for end > 0 && line[end-1] == ' ' {
		if end >= 2 && line[end-2] == '\\' {
			break
		}
		end--
	}
	return line[:end]
}

// globExpression translates one gitignore glob into an anchored expression
// over a canonical vault-relative path. No wildcard crosses a slash except a
// double star standing as a whole path segment.
func globExpression(glob string, anchored bool) string {
	var out strings.Builder
	out.WriteString("^")
	if !anchored {
		out.WriteString("(?:.*/)?")
	}
	runes := []rune(glob)
	for i := 0; i < len(runes); {
		piece, width := globToken(runes, i)
		out.WriteString(piece)
		i += width
	}
	out.WriteString("$")
	return out.String()
}

// globToken translates the one glob token that opens at runes[i], returning
// its expression and how many runes it spans.
func globToken(runes []rune, i int) (expression string, width int) {
	switch r := runes[i]; r {
	case '\\':
		if i+1 < len(runes) {
			return regexp.QuoteMeta(string(runes[i+1])), 2
		}
		return "", 1
	case '*':
		return starToken(runes, i)
	case '?':
		return "[^/]", 1
	case '[':
		if class, classWidth, ok := bracketClass(runes[i:]); ok {
			return class, classWidth
		}
		return `\[`, 1
	default:
		return regexp.QuoteMeta(string(r)), 1
	}
}

// starToken translates the star run that opens at runes[i]. A double star
// standing as a whole path segment spans folders: "dir/**" holds everything
// inside dir but not dir itself, so the slash before it stays literal, and
// "**/" stands for any number of leading folders. Any other run of stars is one
// star, as git reads it.
func starToken(runes []rune, i int) (expression string, width int) {
	if wholeSegmentDoubleStar(runes, i) {
		if i+2 == len(runes) {
			return ".*", 2
		}
		return "(?:.*/)?", 3
	}
	width = 1
	for i+width < len(runes) && runes[i+width] == '*' {
		width++
	}
	return "[^/]*", width
}

// wholeSegmentDoubleStar reports whether the "**" at runes[i] fills a whole
// path segment: it opens the pattern or follows a slash, and it closes the
// pattern or precedes one.
func wholeSegmentDoubleStar(runes []rune, i int) bool {
	if i+1 >= len(runes) || runes[i+1] != '*' {
		return false
	}
	opens := i == 0 || runes[i-1] == '/'
	closes := i+2 == len(runes) || runes[i+2] == '/'
	return opens && closes
}

// bracketClass translates a "[...]" set at the front of runes, returning the
// expression and how many runes it consumed. A set that never closes is not a
// set, and the bracket stands for itself. A set never matches a slash.
func bracketClass(runes []rune) (class string, width int, ok bool) {
	i := 1
	opening := "["
	if i < len(runes) && (runes[i] == '!' || runes[i] == '^') {
		opening = "[^/"
		i++
	}
	var body strings.Builder
	// A closing bracket right after the opening is a member, not the end.
	for first := true; i < len(runes); first = false {
		if runes[i] == ']' && !first {
			return opening + body.String() + "]", i + 1, true
		}
		piece, consumed := classToken(runes, i)
		body.WriteString(piece)
		i += consumed
	}
	return "", 0, false
}

// classToken translates one member of a bracket set, escaping what the
// expression language would otherwise read as structure.
func classToken(runes []rune, i int) (expression string, width int) {
	r := runes[i]
	switch {
	case r == '\\' && i+1 < len(runes):
		return regexp.QuoteMeta(string(runes[i+1])), 2
	case r == '[' || r == ']' || r == '^' || r == '\\' || r == '/':
		return regexp.QuoteMeta(string(r)), 1
	}
	return string(r), 1
}

// ignored reports whether the vault-relative canonical path is left out by the
// rules. isDir says whether it names a folder, since a rule ending in a slash
// matches folders only. The last rule that matches decides.
func (r ignoreRules) ignored(canonical string, isDir bool) bool {
	left := false
	for _, rule := range r.rules {
		if rule.dirOnly && !isDir {
			continue
		}
		if rule.match.MatchString(canonical) {
			left = !rule.negate
		}
	}
	return left
}
