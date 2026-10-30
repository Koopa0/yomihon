package judge

import (
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/vault"
)

// The image rule reports a picture the reading page would show from the vault
// and cannot, because nothing is at the path it names. It asks the page's own
// question with the page's own resolution, so check and the page name the same
// files: a Markdown image resolves against its note's directory, and an embed
// naming a file is judged beside the other wikilinks.

// checkImages reports each Markdown image whose resolved vault path the scan
// did not find. The scan's membership is complete before any note is read, so
// the verdict stands on a run that could not read some file. A path under
// ground the contract withholds is never looked up, the way the disk-reference
// rule declines it, and a path with a hidden segment is where this reader does
// not look rather than a file anybody lost.
func checkImages(notes []note, scan vault.Scan, authority scanAuthority) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		if !authority.egressAllowed(n.path) {
			continue
		}
		for _, image := range n.images {
			resolved, ok := render.ImagePath(image.destination, n.path)
			if !ok {
				continue
			}
			canonical := vault.NormalizeNFC(resolved)
			if !authority.egressAllowed(canonical) || authority.outsideScan(canonical) || scan.Contains(canonical) {
				continue
			}
			out = append(out, missingImage(n, image.line, resolved))
		}
	}
	return out
}

// missingImage is a picture whose target names no file in the vault. The
// target is the path the page reports, so the two can be compared line for
// line.
func missingImage(n *note, line int, target string) Finding {
	return Finding{
		RuleID:          "link.broken.image",
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(line),
		Message:         "image to " + target + " resolves to no file",
		Evidence:        "no file matches the image target",
		SuggestedAction: "restore the target file, change the image to an existing file, or remove it",
		SourceRule:      sourceYomihon,
		Target:          new(target),
		Fingerprint:     fingerprint("link.broken.image", n.path, target),
	}
}
