package render

import (
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

func TestTaskNamingStopsAtMalformedBoundaries(t *testing.T) {
	t.Parallel()
	const input = `<input disabled="" type="checkbox">`
	const opener = `<label class="y-task">`
	for _, prefix := range []struct {
		name string
		body string
	}{
		{name: "another label", body: opener + input + " unfinished "},
		{name: "list item", body: opener + input + " unfinished \ue0020\ue003</li><li>orphan </label></li>"},
	} {
		for _, label := range []struct {
			name string
			body string
			want string
		}{
			{name: "empty", body: opener + input + " </label>", want: opener + input + ` <span lang="en">Task without text</span></label>`},
			{name: "block", body: opener + input + " own \ue0020\ue003</label>", want: opener + input + ` <span class="y-offscreen">own</span></label>` + " own \ue0020\ue003"},
		} {
			t.Run(prefix.name+"/"+label.name, func(t *testing.T) {
				t.Parallel()
				body := prefix.body + label.body
				t.Logf("invoked: task-naming-boundary %q", body)
				got := nameTaskLabels(body, nil, wording.En)
				want := prefix.body + label.want
				if got != want {
					t.Errorf("caught: task naming crossed %s or consumed the following %s label: got %q, want %q", prefix.name, label.name, got, want)
				}
			})
		}
	}
}

func TestTaskNamingPreservesCompletePhrasingLabels(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`<label class="y-task"><input disabled="" type="checkbox"> words <em>emphasis</em> <ruby>字<rt>じ</rt></ruby> <img src="image.png" alt="image"> <a href="Target.md">link</a> &amp; <code>code</code></label>`,
		`<label class="y-task"><input disabled="" type="checkbox"> words</label><label class="y-task"><input disabled="" type="checkbox"> next</label>`,
		`<label class="y-task"><input disabled="" type="checkbox"> unfinished`,
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			t.Logf("invoked: task-naming-positive %q", body)
			if got := nameTaskLabels(body, nil, wording.En); got != body {
				t.Errorf("caught: task naming changed complete phrasing or an unmatched suffix: got %q, want %q", got, body)
			}
		})
	}
}
