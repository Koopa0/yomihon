package pages

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

// TestTwoColumnShellsUseOnlyTheirDeclaredColumns keeps a spare rail from
// becoming a second grid row. The source inventory makes every page using
// this shell participate, including recovery pages reached without a write.
func TestTwoColumnShellsUseOnlyTheirDeclaredColumns(t *testing.T) {
	t.Parallel()
	root, model := buildVault(t)
	course := newRecordedCourse(t, root, model)
	cases := []struct {
		name     string
		page     func(layouts.Chrome) templ.Component
		children []string
	}{
		{"contractchanged.templ", ContractChanged, []string{"main"}},
		{"health.templ", func(c layouts.Chrome) templ.Component { return Health(recordedHealthView(model), c) }, []string{"left", "main"}},
		{"listen.templ", func(c layouts.Chrome) templ.Component { return Listen(recordedListenView(), c) }, []string{"main"}},
		{"notfound.templ", func(c layouts.Chrome) templ.Component {
			return NotFound(NotFoundView{Asked: "/notes/absent.md", Sidebar: NewSidebar(nav.Shell{Nav: model}, "")}, c)
		}, []string{"left", "main"}},
		{"recovery.templ", func(c layouts.Chrome) templ.Component { return StatusRecovery(recordedRecoveryView(model), c) }, []string{"left", "main"}},
		{"report.templ", func(c layouts.Chrome) templ.Component {
			return Report(ReportView{Name: "brief.html", Title: "Brief", Label: "Brief", ReadingRail: NewReportReadingRail(recordedShell(model), "System/reports/daily-briefing/brief.html")}, c)
		}, []string{"left", "main"}},
		{"search.templ", func(c layouts.Chrome) templ.Component { return Search(recordedSearchView(model, c.Lang), c) }, []string{"left", "main"}},
		{"syllabus.templ", func(c layouts.Chrome) templ.Component { return Syllabus(course.view(c.Lang, ""), c) }, []string{"left", "main"}},
	}
	var wantSources []string
	for _, tc := range cases {
		wantSources = append(wantSources, "pages/"+tc.name)
	}
	var gotSources []string
	classAttribute := regexp.MustCompile(`class="([^"]*)"`)
	sources, err := os.OpenRoot(templSourceRoot)
	if err != nil {
		t.Fatalf("OpenRoot(%q): %v", templSourceRoot, err)
	}
	t.Cleanup(func() {
		if closeErr := sources.Close(); closeErr != nil {
			t.Errorf("Close(template sources): %v", closeErr)
		}
	})
	err = fs.WalkDir(sources.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".templ" {
			return nil
		}
		data, readErr := sources.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, attr := range classAttribute.FindAllSubmatch(data, -1) {
			if slices.Contains(strings.Fields(string(attr[1])), "y-shell2") {
				gotSources = append(gotSources, filepath.ToSlash(path))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan templates: %v", err)
	}
	t.Log("SHELL_CATALOG_INVOKED: template source scan completed")
	if diff := cmp.Diff(wantSources, gotSources); diff != "" {
		t.Fatalf("caught: two-column-source-catalog (-covered +declared):\n%s", diff)
	}
	for _, tc := range cases {
		for _, chrome := range []layouts.Chrome{recordedChrome(), recordedEnglishChrome()} {
			t.Run(tc.name+"/"+chrome.Lang.Tag(), func(t *testing.T) {
				t.Parallel()
				var output bytes.Buffer
				if renderErr := tc.page(chrome).Render(t.Context(), &output); renderErr != nil {
					t.Fatalf("Render(%s): %v", tc.name, renderErr)
				}
				t.Logf("SHELL_RENDER_INVOKED: %s/%s", tc.name, chrome.Lang.Tag())
				document, parseErr := html.Parse(&output)
				if parseErr != nil {
					t.Fatalf("Parse(%s): %v", tc.name, parseErr)
				}
				var shells []*html.Node
				var visit func(*html.Node)
				visit = func(node *html.Node) {
					if shellHasClass(node, "y-shell2") {
						shells = append(shells, node)
					}
					for child := node.FirstChild; child != nil; child = child.NextSibling {
						visit(child)
					}
				}
				visit(document)
				if len(shells) != 1 {
					t.Fatalf("Render(%s) shell count = %d, want 1", tc.name, len(shells))
				}
				var got []string
				for child := shells[0].FirstChild; child != nil; child = child.NextSibling {
					if child.Type != html.ElementNode || child.Data == "script" {
						continue
					}
					switch {
					case child.Data == "aside" && shellHasClass(child, "y-rail-left"):
						got = append(got, "left")
					case child.Data == "main" && shellHasClass(child, "y-main"):
						got = append(got, "main")
					default:
						got = append(got, child.Data+" (unexpected grid child)")
					}
				}
				if diff := cmp.Diff(tc.children, got); diff != "" {
					t.Errorf("caught: two-column-grid-child-set Render(%s) (-want +got):\n%s", tc.name, diff)
				}
			})
		}
	}
}

func shellHasClass(node *html.Node, class string) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" {
			return slices.Contains(strings.Fields(attr.Val), class)
		}
	}
	return false
}
