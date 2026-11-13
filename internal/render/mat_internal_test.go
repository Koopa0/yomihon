package render

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestAPictureTheVaultDoesNotHoldIsNotSetOnAMat: a picture whose file is not
// there is marked where the author put it, and that marker is what stands in
// the paragraph. A mat around a broken-image icon would present a hole as if it
// were a drawing.
func TestAPictureTheVaultDoesNotHoldIsNotSetOnAMat(t *testing.T) {
	t.Parallel()
	const written = "<p><img src=\"pic.png\" alt=\"gone\"></p>\n"
	var diags []Diagnostic
	for name, files := range map[string]Files{"held": holdsEverything{}, "missing": holdsNothing{}} {
		got := matLonePictures(resolveAssetHrefs(written, "Notes/note.md", files, wording.En, &diags))
		if matted := strings.Contains(got, `class="mat"`); matted != (name == "held") {
			t.Errorf("%s: mat present = %v:\n%s", name, matted, got)
		}
	}
}
