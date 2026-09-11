package nav

import (
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

func TestTeachingPath(t *testing.T) {
	t.Parallel()

	goBody := "## Data {sequence=primary}\n\n" +
		"- [[L01 Slices]]\n" +
		"- [[L03 Maps]]\n"
	jpBody := "## 初級 {sequence=primary}\n\n" +
		"- [[L01 て形]]\n" +
		"- [[L01 Slices]]\n"
	secondGoBody := "## 導讀 {sequence=primary}\n\n" +
		"- [[L01 Slices]]\n"

	idx := resolver(t,
		"Writing/lessons/golang/L01 Slices.md",
		"Writing/lessons/golang/L03 Maps.md",
		"Writing/lessons/japanese/L01 て形.md",
	)
	policy := testArtifactPolicy(t)
	goPath := buildPath(domainPathNote("Maps/go.md", "Go 課綱", "golang", goBody), idx, nil, nil, policy)
	jpPath := buildPath(domainPathNote("Maps/jp.md", "日本語 學習路徑", "japanese", jpBody), idx, nil, nil, policy)
	secondGo := buildPath(domainPathNote("Maps/second.md", "Second Go", "golang", secondGoBody), idx, nil, nil, policy)

	twoCourse := &Model{paths: []Path{goPath, jpPath}}
	threeCourse := &Model{paths: []Path{goPath, jpPath, secondGo}}
	tie := &Model{paths: []Path{goPath, secondGo}}

	if path := twoCourse.TeachingPath("Maps/go.md", "golang"); path == nil || path.RelPath != "Maps/go.md" {
		t.Errorf("a study-path note answers as its own book, got %v", path)
	}

	tests := []struct {
		name       string
		model      *Model
		relPath    string
		noteDomain string
		want       string
	}{
		{
			name:       "one path teaches the note",
			model:      twoCourse,
			relPath:    "Writing/lessons/golang/L03 Maps.md",
			noteDomain: "golang",
			want:       "Maps/go.md",
		},
		{
			name:       "no path teaches the note",
			model:      twoCourse,
			relPath:    "Concepts/plain.md",
			noteDomain: "golang",
			want:       "",
		},
		{
			name:       "domain picks one course among two domains",
			model:      twoCourse,
			relPath:    "Writing/lessons/golang/L01 Slices.md",
			noteDomain: "golang",
			want:       "Maps/go.md",
		},
		{
			name:       "another domain picks the other course",
			model:      twoCourse,
			relPath:    "Writing/lessons/golang/L01 Slices.md",
			noteDomain: "japanese",
			want:       "Maps/jp.md",
		},
		{
			name:       "no domain on a multi-path note falls back",
			model:      twoCourse,
			relPath:    "Writing/lessons/golang/L01 Slices.md",
			noteDomain: "",
			want:       "",
		},
		{
			name:       "a tie in the same domain falls back",
			model:      tie,
			relPath:    "Writing/lessons/golang/L01 Slices.md",
			noteDomain: "golang",
			want:       "",
		},
		{
			name:       "three courses with two in-domain is still a tie",
			model:      threeCourse,
			relPath:    "Writing/lessons/golang/L01 Slices.md",
			noteDomain: "golang",
			want:       "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.model.TeachingPath(tt.relPath, tt.noteDomain)
			var rel string
			if got != nil {
				rel = got.RelPath
			}
			if rel != tt.want {
				t.Errorf("TeachingPath(%q, %q) = %q, want %q", tt.relPath, tt.noteDomain, rel, tt.want)
			}
		})
	}
}

func domainPathNote(rel, title, domain, body string) *vault.Note {
	return &vault.Note{
		RelPath:     rel,
		Frontmatter: map[string]any{"title": title, "type": "study-path", "domain": domain},
		Body:        body,
		BodyLine:    1,
	}
}
