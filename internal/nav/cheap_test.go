package nav

import (
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// A note page asks the model ten questions about its paths and maps and only
// two of them are "walk these": the rest ask a count or a name. Answering
// those through the copying accessors cloned every branch tree of every course
// eight times per render, to learn a length. The cheap questions have to
// actually be cheap, so the count is asserted at zero and the copy at more than
// zero in the same table: an optimizer that ate both would fail the second row
// rather than pass the first.
//
// Not parallel: testing.AllocsPerRun is unreliable while other tests run.
func TestTheCheapQuestionsCopyNothing(t *testing.T) {
	model := immutableModelFixture()

	tests := []struct {
		name      string
		run       func()
		wantAlloc bool
	}{
		{name: "PathCount", run: func() { _ = model.PathCount() }},
		{name: "MapCount", run: func() { _ = model.MapCount() }},
		{name: "IsPath", run: func() { _ = model.IsPath("Maps/Path.md") }},
		{name: "IsMap", run: func() { _ = model.IsMap("Maps/Map.md") }},
		{name: "Paths", run: func() { _ = model.Paths() }, wantAlloc: true},
		{name: "Maps", run: func() { _ = model.Maps() }, wantAlloc: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allocs := testing.AllocsPerRun(100, tt.run)
			if got := allocs > 0; got != tt.wantAlloc {
				t.Errorf("%s allocates %v per call, want allocating = %t", tt.name, allocs, tt.wantAlloc)
			}
		})
	}
}

// The cheap questions and the copying accessors have to answer the same thing,
// including for the three models that are not an ordinary populated one: the
// nil receiver every accessor here is safe for, a zero model, and the
// request-local model whose instance projections were withheld.
func TestTheCheapQuestionsAgreeWithTheCopies(t *testing.T) {
	t.Parallel()

	withheld := immutableModelFixture().WithoutInstanceProjections(Close(schema.Rejected("artifact unavailable")))
	models := map[string]*Model{
		"nil":       nil,
		"zero":      {},
		"populated": immutableModelFixture(),
		"withheld":  withheld,
	}
	for name, model := range models {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got, want := model.PathCount(), len(model.Paths()); got != want {
				t.Errorf("PathCount() = %d, len(Paths()) = %d", got, want)
			}
			if got, want := model.MapCount(), len(model.Maps()); got != want {
				t.Errorf("MapCount() = %d, len(Maps()) = %d", got, want)
			}
			for _, rel := range []string{"Maps/Path.md", "Maps/Map.md", "Writing/Root.md", ""} {
				wantPath := false
				for _, p := range model.Paths() {
					wantPath = wantPath || p.RelPath == rel
				}
				if got := model.IsPath(rel); got != wantPath {
					t.Errorf("IsPath(%q) = %t, Paths() says %t", rel, got, wantPath)
				}
				wantMap := false
				for _, x := range model.Maps() {
					wantMap = wantMap || x.RelPath == rel
				}
				if got := model.IsMap(rel); got != wantMap {
					t.Errorf("IsMap(%q) = %t, Maps() says %t", rel, got, wantMap)
				}
			}
		})
	}
}
