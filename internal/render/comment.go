package render

import "strings"

// stripObsidianComments removes Obsidian %% comment regions while preserving
// the delimiters and contents of fenced code blocks. An unclosed comment runs
// to the end of the body, matching Obsidian's hidden-source behavior.
func stripObsidianComments(body string) string {
	lines := strings.Split(body, "\n")
	inComment := false
	inFence := false
	var fenceByte byte

	for i, line := range lines {
		if inFence {
			if fenceCloses(line, fenceByte) {
				inFence = false
			}
			continue
		}
		if marker, _, ok := fenceOpen(line); ok {
			inFence = true
			fenceByte = marker
			continue
		}

		lines[i] = stripObsidianCommentLine(line, &inComment)
		if inComment {
			continue
		}
		if marker, _, ok := fenceOpen(lines[i]); ok {
			inFence = true
			fenceByte = marker
		}
	}
	return strings.Join(lines, "\n")
}

func stripObsidianCommentLine(line string, inComment *bool) string {
	var visible strings.Builder
	for {
		before, after, found := strings.Cut(line, "%%")
		if !*inComment {
			visible.WriteString(before)
		}
		if !found {
			return visible.String()
		}
		*inComment = !*inComment
		line = after
	}
}
