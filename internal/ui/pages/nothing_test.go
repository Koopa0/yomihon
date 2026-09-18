package pages

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// noticeKind reads back which notice each recorded page drew and in what order.
var noticeKind = regexp.MustCompile(`<div class="y-nothing(?: y-nothing--fault)?" data-nothing="([a-z]+)"`)

// faultedNoticeKind reads back only the ones drawn as faults.
var faultedNoticeKind = regexp.MustCompile(`<div class="y-nothing y-nothing--fault" data-nothing="([a-z]+)"`)

// TestEverySurfaceWithNothingToShowDrawsTheOneNotice holds the whole point of
// that component: six surfaces used to say "there is nothing here" in six
// shapes, and a reader met a different layout every time for the same kind of
// moment. The recordings are the only instrument that can see this — every one
// of those surfaces renders, its own test passes, and the six drift apart
// without a single failure.
//
// It is written per surface rather than as one tally over the whole folder. A
// count would stay right while one page went back to drawing its own, so each
// recording names the notices it owes, in the order a reader meets them, and
// names the markup it is no longer allowed to draw.
func TestEverySurfaceWithNothingToShowDrawsTheOneNotice(t *testing.T) {
	t.Parallel()

	// The shapes these surfaces used to draw for themselves. A recording that
	// carries one again is a surface that went back to its own layout, which no
	// assertion about the notice being present anywhere on the page would see.
	const (
		ownSearchShape   = `class="y-searchpage__empty"`
		ownRecoveryShape = `class="y-recovery__summary"`
		ownParagraph     = `class="y-homeempty"`
	)
	surfaces := []struct {
		recording string
		// notices are the kinds this page draws, in document order.
		notices []string
		// faulted are the ones drawn as faults rather than as absences.
		faulted []string
		// retired is markup this page may no longer draw.
		retired []string
	}{
		{"search-page-empty", []string{"search"}, nil, []string{ownSearchShape}},
		{"search-page-empty-english", []string{"search"}, nil, []string{ownSearchShape}},
		{"notfound-page", []string{"notfound"}, nil, []string{ownRecoveryShape}},
		{"notfound-page-english", []string{"notfound"}, nil, []string{ownRecoveryShape}},
		{"notfound-page-unreadable", []string{"notfound"}, []string{"notfound"}, []string{ownRecoveryShape}},
		{"health-page-clear", []string{"health"}, nil, []string{ownParagraph}},
		{"health-page-clear-english", []string{"health"}, nil, []string{ownParagraph}},
		{
			// The desk carries four at once: the two shelves a declaration has
			// not filled, and the two notices about the reading itself.
			"home-page-empty",
			[]string{"shelf", "shelf", "privacy", "degraded"},
			[]string{"privacy", "degraded"},
			[]string{ownParagraph},
		},
		{
			"home-page-empty-english",
			[]string{"shelf", "shelf", "privacy", "degraded"},
			[]string{"privacy", "degraded"},
			[]string{ownParagraph},
		},
	}

	var covered []string
	for _, tt := range surfaces {
		covered = append(covered, tt.notices...)
		t.Run(tt.recording, func(t *testing.T) {
			t.Parallel()
			page := recordedPage(t, tt.recording)
			if diff := cmp.Diff(tt.notices, noticeKinds(noticeKind, page)); diff != "" {
				t.Errorf("%s draws the wrong notices (-want +recorded):\n%s", tt.recording, diff)
			}
			if diff := cmp.Diff(tt.faulted, noticeKinds(faultedNoticeKind, page)); diff != "" {
				t.Errorf("%s marks the wrong notices as faults (-want +recorded):\n%s", tt.recording, diff)
			}
			for _, shape := range tt.retired {
				if strings.Contains(page, shape) {
					t.Errorf("%s draws %s again, so it is back to a shape of its own", tt.recording, shape)
				}
			}
		})
	}

	// The surfaces named are the whole set this component answers for. A table
	// that lost a row would go on passing over the rows it kept, and the page
	// it stopped watching is exactly the one that would drift.
	slices.Sort(covered)
	want := []string{"degraded", "health", "notfound", "privacy", "search", "shelf"}
	if diff := cmp.Diff(want, slices.Compact(covered)); diff != "" {
		t.Errorf("the recordings above cover the wrong set of surfaces (-want +covered):\n%s", diff)
	}
}

// TestTheNoticeIsDrawnInOnePlace keeps the six from growing a seventh shape
// quietly. Its markup is one component's, so the class belongs to that one
// file: a page assembling the same div itself would satisfy every assertion
// above and still be a second copy to keep in step.
func TestTheNoticeIsDrawnInOnePlace(t *testing.T) {
	t.Parallel()

	// Every hand-written source the interface is built from — both template
	// packages and the Go beside them. Generated templ output is the templates
	// again, and this test's own file names the class on purpose.
	var sources []string
	for _, pattern := range []string{"*.templ", "*.go", "../layouts/*.templ", "../layouts/*.go"} {
		matched, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("Glob(%s): %v", pattern, err)
		}
		for _, source := range matched {
			if strings.HasSuffix(source, "_templ.go") || strings.HasSuffix(source, "_test.go") {
				continue
			}
			sources = append(sources, source)
		}
	}
	if len(sources) < 30 {
		t.Fatalf("only %d sources were read, so this check looks at almost nothing", len(sources))
	}
	// The root class as an element writes it, and the marker that names the
	// surface. The first is bounded so a child class of the same family — the
	// advice line a search writes inside the notice — is not read as a second
	// copy of the notice itself.
	root := regexp.MustCompile(`class="y-nothing[ "]|data-nothing`)
	var writers []string
	for _, source := range sources {
		text, err := os.ReadFile(source) // #nosec G304 -- a source beside this test, named by the globs above
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", source, err)
		}
		if root.Match(text) {
			writers = append(writers, filepath.Base(source))
		}
	}
	// The component is two files: the template that writes the markup and the
	// type whose doc says what the marker on it means. Anything else naming
	// either is a second copy of the notice.
	if diff := cmp.Diff([]string{"nothing.templ", "nothing.go"}, writers); diff != "" {
		t.Errorf("the notice's markup is written outside its own component (-want +writing):\n%s", diff)
	}
}

// recordedPage reads one of the recorded surfaces. A recording that has gone
// missing is a check that would otherwise pass over an empty string.
func recordedPage(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", "render", name+".html")
	page, err := os.ReadFile(path) // #nosec G304 -- a recording named by this test's own table
	if err != nil {
		t.Fatalf("ReadFile(%s): %v — rerun with -update-render-bytes to record it", path, err)
	}
	if len(page) == 0 {
		t.Fatalf("%s recorded no bytes, so nothing below is looking at a page", path)
	}
	return string(page)
}

// noticeKinds names the notices one recorded page draws, in document order.
func noticeKinds(pattern *regexp.Regexp, page string) []string {
	var kinds []string
	for _, match := range pattern.FindAllStringSubmatch(page, -1) {
		kinds = append(kinds, match[1])
	}
	return kinds
}
