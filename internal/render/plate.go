package render

import (
	"html"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// A picture alone in its paragraph, or a diagram, is a plate: a figure on the
// mat's ground, with a caption under it only where the author gave it words.
//
// A picture's words are its alt text, which the author wrote as ![words](…) or
// ![[…|words]]: an image alone in a paragraph with alt text is a figure
// captioned by it, which is how Pandoc reads the same markdown. The file's own
// name, which an embed with no words is given for its alt, is not a caption.
// A diagram's words are its accTitle, Mermaid's own name for a diagram, which
// it keeps out of the drawing. The caption repeats what the picture's alt or
// the diagram's title already says to a screen reader, so it is hidden from
// one, as Pandoc hides it.
//
// The mat takes the light or dark of what it holds. An SVG's own stylesheet
// asks the browser for prefers-color-scheme, which answers with the system, so
// its mat does too. A raster picture is the same in either, and a diagram is
// drawn again in the page's theme whenever that changes, so their mats are in
// the page's scheme (mat--page).

// plateInset is the mat's inset either side of what it holds, which a picture's
// room at each measure is counted without.
const plateInset = 2 * 28 // components.css .y-prose .mat --mat-side, 28px either side

// lonePicture matches a paragraph that holds one picture the vault serves and
// nothing else, which is how the renderer writes a picture on a line of its own
// whether the author wrote ![](…) or ![[…]]. It reads what this package wrote,
// as localImageSrc does, so it relies on the same two facts: the tag is emitted
// with src first, and no ">" appears inside an attribute value. A picture the
// vault does not hold arrives wrapped in its own marker by resolveAssetHrefs
// and is not matched: there is nothing to set down.
var lonePicture = regexp.MustCompile(`<p>(<img src="/raw/([^"?#]*)(?:[?#][^"]*)?"([^>]*)>)</p>`)

var (
	pictureAlt   = regexp.MustCompile(` alt="([^"]*)"`)
	pictureWidth = regexp.MustCompile(` width="(\d+)"`)
)

// plateLonePictures sets every picture that stands alone in its paragraph on a
// plate. A picture inside a sentence is part of the sentence and is left as it
// is.
func plateLonePictures(htmlOut string) string {
	return lonePicture.ReplaceAllStringFunc(htmlOut, func(match string) string {
		parts := lonePicture.FindStringSubmatch(match)
		tag, src, attributes := parts[1], parts[2], parts[3]
		class := "mat mat--page"
		if strings.EqualFold(path.Ext(src), ".svg") {
			class = "mat"
		}
		var caption string
		if alt := pictureAlt.FindStringSubmatch(attributes); len(alt) == 2 && authoredCaption(html.UnescapeString(alt[1]), src) {
			caption = alt[1]
		}
		wide := ""
		if width := pictureWidth.FindStringSubmatch(attributes); len(width) == 2 {
			wide = wideForWidth(width[1])
		}
		return plate(class, wide, tag, caption)
	})
}

// authoredCaption reports whether alt is words the author wrote rather than
// nothing, or the name of the file at src, which is what an embed with no words
// of its own is given.
func authoredCaption(alt, src string) bool {
	alt = strings.TrimSpace(alt)
	if alt == "" {
		return false
	}
	name, err := url.PathUnescape(path.Base(src))
	return err != nil || alt != name
}

// wideForWidth names every measure a picture the author gave this many pixels
// does not fit at, inside the mat's inset.
func wideForWidth(pixels string) string {
	width, err := strconv.Atoi(pixels)
	if err != nil {
		return ""
	}
	return wideFor(func(m readingMeasure) bool { return width > m.width-plateInset })
}

// plate is one figure on a mat: what it holds, and a caption only when there
// is one. caption is HTML-escaped already.
func plate(class, wide, content, caption string) string {
	var b strings.Builder
	b.WriteString(`<figure class="` + class + `"` + wideAttribute(wide) + `>`)
	b.WriteString(content)
	if caption != "" {
		b.WriteString(`<figcaption aria-hidden="true">` + caption + `</figcaption>`)
	}
	b.WriteString(`</figure>`)
	return b.String()
}

// diagramPlate is a diagram's placeholder on its plate, captioned by its
// accTitle and set wide unless its layout runs down the page.
func diagramPlate(source, placeholder string) string {
	wide := ""
	if !diagramRunsDown(source) {
		wide = wideFor(func(readingMeasure) bool { return true })
	}
	return plate("mat mat--page", wide, placeholder, html.EscapeString(diagramTitle(source)))
}

// diagramTitle is the accTitle a diagram's source gives it, or nothing.
func diagramTitle(source string) string {
	for line := range strings.Lines(source) {
		key, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if found && strings.TrimSpace(key) == "accTitle" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// diagramRunsDown reports whether a diagram's layout runs down the page rather
// than across it, which is the one thing about its width the server can know.
// A diagram's width is only known once the browser has drawn it, and Mermaid
// draws it no wider than it is and narrower where it must, which takes its
// words below a readable size. So the safe answer for a diagram that may run
// across is the wide measure, where a narrow one is only a smaller drawing on a
// wider mat; a diagram that runs down, or is drawn to a size of its own that
// fits the text, keeps the text's width. A flowchart runs down unless it says
// LR or RL; a state, class, entity or requirement diagram unless it turns
// itself with a direction statement; a pie and a quadrant chart are drawn to
// their own size; a git graph runs across unless it says TB or BT. Everything
// else, a sequence, a timeline, a Gantt chart, a mind map or a kind added
// after this was written, runs across.
func diagramRunsDown(source string) bool {
	kind, direction := diagramHeader(source)
	switch kind {
	case "graph", "flowchart":
		return direction != "LR" && direction != "RL"
	case "stateDiagram", "stateDiagram-v2", "classDiagram", "classDiagram-v2", "erDiagram", "requirementDiagram":
		for line := range strings.Lines(source) {
			if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "direction" && (fields[1] == "LR" || fields[1] == "RL") {
				return false
			}
		}
		return true
	case "pie", "quadrantChart":
		return true
	case "gitGraph":
		return direction == "TB" || direction == "BT"
	default:
		return false
	}
}

// diagramHeader is the kind a diagram's first statement names and the
// direction after it, if any, past a front matter block, directives and
// comments.
func diagramHeader(source string) (kind, direction string) {
	inFrontMatter := false
	for i, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case i == 0 && line == "---":
			inFrontMatter = true
			continue
		case inFrontMatter:
			inFrontMatter = line != "---"
			continue
		case line == "" || strings.HasPrefix(line, "%%"):
			continue
		}
		fields := strings.Fields(strings.NewReplacer(";", " ", ":", " ").Replace(line))
		if len(fields) > 1 {
			direction = fields[1]
		}
		return fields[0], direction
	}
	return "", ""
}
