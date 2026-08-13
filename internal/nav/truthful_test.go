package nav

import (
	"fmt"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/sequence"
)

// TestAPartShowsTheLessonsItCarries holds the one number a reader compares
// across two pages. A part that only groups modules still carries their
// lessons, so Home and the part heading have to agree — "Home 2 課" beside
// "part 0 課" tells the reader one of the two is lying, and does not say which.
func TestAPartShowsTheLessonsItCarries(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/L01.md", "Writing/L02.md", "Writing/S01.md", "Writing/R01.md", "Writing/U01.md")
	body := "## Part\n\n" +
		"### Module {sequence=primary}\n\n" +
		"- [[L01]]\n" +
		"- [[L02]]\n" +
		"\t- 支線 {sequence=local}\n" +
		"\t\t- [[S01]]\n" +
		"\n### 日常 {sequence=none}\n\n- [[R01]]\n" +
		"\n### 忘了宣告\n\n- [[U01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	if p.Planned != 2 {
		t.Fatalf("Planned = %d, want 2", p.Planned)
	}
	if len(p.Groups) != 1 {
		t.Fatalf("top-level branches = %d, want 1", len(p.Groups))
	}
	part := p.Groups[0]
	if part.Planned != p.Planned {
		t.Errorf("the part shows %d 課 while Home shows %d; a structural part must carry its primary descendants",
			part.Planned, p.Planned)
	}

	// The side branch keeps its own count and does not join the part's.
	var local *PathGroup
	for _, g := range allPathGroups(p.Groups) {
		if g.Role == sequence.RoleLocal {
			local = g
		}
	}
	if local == nil || local.Planned != 1 {
		t.Errorf("side branch = %+v, want its own count of 1", local)
	}

	// A branch the course does not include has no course count to show. A
	// number beside a block declared out of the course, or one nobody
	// declared, would be a count of something that is not the course.
	for _, g := range allPathGroups(p.Groups) {
		switch g.Role {
		case sequence.RoleNone, sequence.RoleUnclassified:
			if g.Planned != 0 {
				t.Errorf("branch %q is %v yet shows %d 課", g.Name, g.Role, g.Planned)
			}
		case sequence.RolePrimary, sequence.RoleLocal, sequence.RoleStructural:
		}
	}
}

// TestPathsHandsOutNoWayBackIntoTheModel holds the snapshot's own rule. The
// model is built once and read by every request at the same time; a caller that
// can reach into a group it was handed can change what the next reader sees.
func TestPathsHandsOutNoWayBackIntoTheModel(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/L01.md", "Writing/S01.md")
	body := "## 主線 {sequence=primary}\n\n- [[L01]]\n\t- 支線 {sequence=local}\n\t\t- [[S01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))
	m := &Model{paths: []Path{p}}

	before := describePaths(m.Paths())

	handed := m.Paths()
	handed[0].Title = "mutated"
	handed[0].Planned = -1
	for _, g := range allPathGroups(handed[0].Groups) {
		g.Name = "mutated"
		g.Planned = -1
		g.Projectable = false
		for _, item := range g.Items {
			if item.Entry != nil {
				item.Entry.Text = "mutated"
				item.Entry.RelPath = "mutated"
				if len(item.Entry.Candidates) > 0 {
					item.Entry.Candidates[0] = "mutated"
				}
			}
		}
	}

	if after := describePaths(m.Paths()); after != before {
		t.Errorf("a caller changed the model through Paths():\nbefore %s\nafter  %s", before, after)
	}
}

func allPathGroups(groups []*PathGroup) []*PathGroup {
	var out []*PathGroup
	for _, g := range groups {
		out = append(out, g)
		for _, item := range g.Items {
			if item.Group != nil {
				out = append(out, allPathGroups([]*PathGroup{item.Group})...)
			}
		}
	}
	return out
}

// describePaths renders the whole projection as text, so a change anywhere in
// it shows up as a difference.
func describePaths(paths []Path) string {
	var b strings.Builder
	for i := range paths {
		p := &paths[i]
		fmt.Fprintf(&b, "%s|%s|%d\n", p.Title, p.RelPath, p.Planned)
		describeGroups(&b, p.Groups, " ")
	}
	return b.String()
}

func describeGroups(b *strings.Builder, groups []*PathGroup, indent string) {
	for _, g := range groups {
		fmt.Fprintf(b, "%s%s|%s|%d|%t|%s\n", indent, g.Name, g.Role, g.Planned, g.Projectable, g.AnchorTarget)
		for _, item := range g.Items {
			switch {
			case item.Entry != nil:
				e := item.Entry
				fmt.Fprintf(b, "%s e:%s|%s|%s|%s\n", indent, e.Text, e.Target, e.RelPath, e.Status)
				for _, c := range e.Candidates {
					fmt.Fprintf(b, "%s  c:%s\n", indent, c)
				}
			case item.Group != nil:
				describeGroups(b, []*PathGroup{item.Group}, indent+" ")
			}
		}
	}
}
