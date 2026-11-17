package judge

import (
	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/vault"
)

// fragmentPaths asks the renderer about multi-part heading addresses. A single
// heading keeps its older, deliberately generous check and frozen findings.
type fragmentPaths struct {
	idx    *graph.Index
	notes  fragmentNotes
	unread map[string]bool
	reader *render.Pipeline
}

func (p *fragmentPaths) finding(n *note, link *wikiLink) (Finding, bool) {
	res := p.idx.Resolve(link.target)
	if res.Kind != graph.KindUnique || !vault.IsMarkdown(res.RelPath) {
		return Finding{}, false
	}
	target := p.notes[res.RelPath]
	if target == nil {
		return Finding{}, false
	}
	if link.embed {
		if _, found := render.Excerpt(target.body, link.heading); found {
			return Finding{}, false
		}
		return sectionMissing(n, link, res.RelPath), true
	}
	if _, found := p.pipeline().HeadingPath(res.RelPath, link.heading); found {
		return Finding{}, false
	}
	for _, embedded := range target.wikilinks {
		if !embedded.embed {
			continue
		}
		place := p.idx.Resolve(embedded.target)
		if place.Kind == graph.KindUnique && vault.IsMarkdown(place.RelPath) && p.unread[place.RelPath] {
			return Finding{}, false
		}
	}
	return sectionMissing(n, link, res.RelPath), true
}

// pipeline is the reading page's own renderer over this run's captured notes,
// built the first time a link asks it something.
func (p *fragmentPaths) pipeline() *render.Pipeline {
	if p.reader == nil {
		p.reader = render.New(p.idx, p.notes, p.notes, p.notes)
	}
	return p.reader
}

// githubHeading reports whether the destination stamps a heading for a section
// name written the way GitHub spells one, which the page follows.
func (p *fragmentPaths) githubHeading(relPath, fragment string) bool {
	_, found := p.pipeline().GitHubHeading(relPath, fragment)
	return found
}

// fragmentNotes supplies only captured bodies to the outline render. Asset
// diagnostics and title-only repairs do not participate in heading identity;
// the render's diagnostics are not imported into the judge's finding stream.
type fragmentNotes map[string]*note

func (n fragmentNotes) Transclusion(path string) (string, bool) {
	if body := n[path]; body != nil {
		return body.body, true
	}
	return "", false
}

func (fragmentNotes) TitledBy(string) []string { return nil }

func (fragmentNotes) MissingFile(string) bool { return false }

func (fragmentNotes) OutsideScan(string) bool { return false }

func (fragmentNotes) IsDirectory(string) bool { return false }
