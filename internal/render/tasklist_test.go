package render_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestTaskCheckBoxesCarryTheirOwnText(t *testing.T) {
	t.Parallel()
	for _, task := range []struct {
		marker string
		input  string
	}{
		{marker: " ", input: `<input disabled="" type="checkbox">`},
		{marker: "x", input: `<input checked="" disabled="" type="checkbox">`},
		{marker: "X", input: `<input checked="" disabled="" type="checkbox">`},
		{marker: "/", input: `<input disabled="" type="checkbox" data-task="/">`},
		{marker: "-", input: `<input disabled="" type="checkbox" data-task="-">`},
		{marker: ">", input: `<input disabled="" type="checkbox" data-task="&gt;">`},
	} {
		t.Run(task.marker, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, nil, nil, nil)
			result := r.HTML("task.md", "", "- ["+task.marker+"] task **words**\n", wording.En)
			want := "<ul>\n<li><label class=\"y-task\">" + task.input + " task <strong>words</strong></label></li>\n</ul>\n"
			if result.HTML != want {
				t.Errorf("caught: HTML(task %q) = %q, want %q", task.marker, result.HTML, want)
			}
		})
	}
}

func TestTaskLabelsCloseBeforeTheNextBlock(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	result := r.HTML("task.md", "", "- [ ] outer\n  - [x] nested\n\n- [/] first paragraph\n\n  another paragraph\n\n1. [>] ordered\n", wording.En)
	for _, want := range []string{
		`<label class="y-task"><input disabled="" type="checkbox"> outer</label>`,
		`<label class="y-task"><input checked="" disabled="" type="checkbox"> nested</label>`,
		`<p><label class="y-task"><input disabled="" type="checkbox" data-task="/"> first paragraph</label></p>`,
		"<p>another paragraph</p>",
		`<li><label class="y-task"><input disabled="" type="checkbox" data-task="&gt;"> ordered</label></li>`,
	} {
		if !strings.Contains(result.HTML, want) {
			t.Errorf("caught: HTML(nested/loose/ordered) lacks %q in %q", want, result.HTML)
		}
	}
	if strings.Count(result.HTML, `<label class="y-task">`) != 4 || strings.Count(result.HTML, "</label>") != 4 {
		t.Errorf("caught: task labels must close once per first inline block: %q", result.HTML)
	}
}

func TestTaskMarkersKeepLiteralContexts(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"[/] outside a list\n", "- before [/] marker\n", "- [?] unknown marker\n",
		"- [ab] two characters\n", "- [/]word without separator\n", "- \\[/] escaped\n",
		"- `[/] code span`\n", "```\n- [/] fenced task\n```\n",
	} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			result := newRenderer(t, nil, nil, nil).HTML("task.md", "", source, wording.En)
			if strings.Contains(result.HTML, `type="checkbox"`) || strings.Contains(result.HTML, `<label`) {
				t.Errorf("caught: literal context became a task: %q", result.HTML)
			}
		})
	}
}

func TestTaskNamesSurviveIndependentReadingBodies(t *testing.T) {
	t.Parallel()
	const target = "Target.md"
	r := newRenderer(t, []graph.NoteInput{{RelPath: target}}, nil, transclusions{target: "- [/] embedded task\n"})
	result := r.HTMLIn("compare-a-", "Host.md", "", "- [ ] own task\n\n![[Target]]\n\n![[Target]]\n", wording.En)
	if strings.Count(result.HTML, `<label class="y-task">`) != 3 || strings.Count(result.HTML, "</label>") != 3 {
		t.Errorf("caught: independent bodies lost task labels: %q", result.HTML)
	}
	preview := render.StripAnchors(result.HTML)
	if strings.Count(preview, `<label class="y-task">`) != 3 || strings.Count(preview, "</label>") != 3 {
		t.Errorf("caught: anchor-free excerpt lost task names: %q", preview)
	}
}

func TestAnEmptyTaskStillHasANameInTheInterfaceLanguage(t *testing.T) {
	t.Parallel()
	for _, language := range []struct {
		lang wording.Lang
		want string
	}{
		{lang: wording.En, want: `<span lang="en">Task without text</span>`},
		{lang: wording.ZhHant, want: `<span lang="zh-Hant">沒有文字的待辦項目</span>`},
	} {
		t.Run(string(language.lang), func(t *testing.T) {
			t.Parallel()
			for _, source := range []string{"- [ ]\n", "- [x] <br>\n", "- [/] ` `\n", "- [ ] &nbsp;\n", "- [ ] &#32;\n"} {
				result := newRenderer(t, nil, nil, nil).HTML("task.md", "", source, language.lang)
				if !strings.Contains(result.HTML, `<label class="y-task">`) || !strings.Contains(result.HTML, language.want) {
					t.Errorf("caught: HTML(empty task %q) lacks named native label %q: %q", source, language.want, result.HTML)
				}
			}
		})
	}
}

func TestTaskMarkerTextAgreesWithItsRenderedBody(t *testing.T) {
	t.Parallel()
	for _, marker := range []string{" ", "x", "X", "/", "-", ">"} {
		t.Run(marker, func(t *testing.T) {
			t.Parallel()
			source := "- [" + marker + "] authored words\n"
			if got := render.PlainText(source); got != "authored words" {
				t.Errorf("caught: PlainText(task %q) = %q, want authored words", marker, got)
			}
		})
	}
}

func TestTaskBlockEmbedsKeepOnlyTheirOwnWordsInTheName(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"- [ ] before **own** ![[Target]] after [link](Target.md)\n",
		"- [ ] before **own** ![[Target]] after [link](Target.md)\n\n  another paragraph\n",
		"- [ ] before **own** ![[Target]] ![[Target]] after [link](Target.md)\n",
	} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": "- [x] inner task\n"})
			result := r.HTMLIn("compare-a-", "Host.md", "", source, wording.En)
			for _, body := range []string{result.HTML, render.StripAnchors(result.HTML)} {
				want := `<label class="y-task"><input disabled="" type="checkbox"> <span class="y-offscreen">before own after link</span></label>`
				if !strings.Contains(body, want) {
					t.Errorf("caught: block task lost its own words: %q", body)
				}
				labels := regexp.MustCompile(`(?s)<label class="y-task">.*?</label>`).FindAllString(body, -1)
				for _, label := range labels {
					if strings.Contains(label, "<div") || strings.Count(label, "<label") != 1 || strings.Count(label, "<input") != 1 {
						t.Errorf("caught: task label contains expanded blocks or controls: %q", label)
					}
				}
				if !strings.Contains(body, `<a href="/notes/Target.md">link</a>`) {
					t.Errorf("caught: authored link no longer remains independent: %q", body)
				}
				if strings.ContainsAny(body, "\ue000\ue001\ue002\ue003") {
					t.Errorf("caught: task name exposed private placeholders: %q", body)
				}
			}
		})
	}
}

func TestTaskBlockEmbedNameUsesInlineProjectionAndLanguage(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": "- [x] inner task\n"})
	for _, test := range []struct {
		source string
		lang   wording.Lang
		want   string
	}{
		{"- [ ] [[Target|own alias]] ![[Target]] & <label>\n", wording.En, `<span class="y-offscreen">own alias &amp; &lt;label&gt;</span>`},
		{"- [ ] ![own image](data:image/png;base64,aGVsbG8=) ![[Target]]\n", wording.En, `<span class="y-offscreen">own image</span>`},
		{"- [ ] ![[Target]]\n", wording.En, `<span class="y-offscreen" lang="en">Task without text</span>`},
		{"- [ ] ![[Target]]\n", wording.ZhHant, `<span class="y-offscreen" lang="zh-Hant">沒有文字的待辦項目</span>`},
	} {
		t.Run(test.source+string(test.lang), func(t *testing.T) {
			t.Parallel()
			result := r.HTML("Host.md", "", test.source, test.lang)
			if !strings.Contains(result.HTML, test.want) {
				t.Errorf("caught: block task projection/language = %q, want %q", result.HTML, test.want)
			}
		})
	}
}

func TestPhrasingTaskKeepsAnAuthoredLink(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, nil)
	result := r.HTML("Host.md", "", "- [ ] [[Target|own link]]\n", wording.En)
	const want = `<label class="y-task"><input disabled="" type="checkbox"> <a href="/notes/Target.md" class="wikilink">own link</a></label>`
	if !strings.Contains(result.HTML, want) {
		t.Errorf("caught: phrasing task altered its independent authored link: %q", result.HTML)
	}
}

func TestTaskCodeEntitiesRemainAuthoredWords(t *testing.T) {
	t.Parallel()
	result := newRenderer(t, nil, nil, nil).HTML("Host.md", "", "- [ ] `&nbsp;`\n", wording.En)
	const want = `<label class="y-task"><input disabled="" type="checkbox"> <code>&amp;nbsp;</code></label>`
	if !strings.Contains(result.HTML, want) {
		t.Errorf("caught: literal code entity was mistaken for empty task text: %q", result.HTML)
	}
}

func TestHeadingTasksStayOutsideFollowingLabels(t *testing.T) {
	heading := regexp.MustCompile(`(?s)<h[1-6]\b[^>]*>(.*?)</h[1-6]>`)
	labels := regexp.MustCompile(`(?s)<label class="y-task">.*?</label>`)
	for _, task := range []struct {
		marker string
		input  string
	}{
		{marker: " ", input: `<input disabled="" type="checkbox">`},
		{marker: "x", input: `<input checked="" disabled="" type="checkbox">`},
		{marker: "X", input: `<input checked="" disabled="" type="checkbox">`},
		{marker: "/", input: `<input disabled="" type="checkbox" data-task="/">`},
		{marker: "-", input: `<input disabled="" type="checkbox" data-task="-">`},
		{marker: ">", input: `<input disabled="" type="checkbox" data-task="&gt;">`},
	} {
		marker := task.marker
		for _, second := range []string{"Before words after", "Before ![[Child]] after"} {
			for _, lang := range []wording.Lang{wording.En, wording.ZhHant} {
				for _, region := range []string{"", "compare-a-"} {
					name := marker + second + string(lang) + region
					t.Run(name, func(t *testing.T) {
						r := newRenderer(t, []graph.NoteInput{{RelPath: "Child.md"}}, nil, transclusions{"Child.md": "- [x] Child task\n"})
						source := "- # [" + marker + "] heading\n- [ ] " + second + "\n"
						result := r.HTMLIn(region, "Host.md", "", source, lang)
						for _, body := range []string{result.HTML, render.StripAnchors(result.HTML)} {
							t.Logf("invoked: heading-task-boundary marker=%q second=%q lang=%q region=%q html=%q", marker, second, lang, region, body)
							match := heading.FindStringSubmatch(body)
							if len(match) != 2 || !strings.Contains(match[1], task.input) || !strings.Contains(match[1], "heading") {
								t.Fatalf("caught: heading lost authored words/control fields: %q", body)
							}
							if strings.Contains(match[1], "<label") {
								t.Errorf("caught: heading opened a label whose block cannot close it: %q", match[1])
							}
							want := `<label class="y-task"><input disabled="" type="checkbox"> Before words after</label>`
							if strings.Contains(second, "![[") {
								want = `<label class="y-task"><input disabled="" type="checkbox"> <span class="y-offscreen">Before after</span></label>`
							}
							if !strings.Contains(body, want) {
								t.Errorf("caught: following task lost its own name: %q, want %q", body, want)
							}
							if strings.Count(body, `<label class="y-task">`) != strings.Count(body, "</label>") {
								t.Errorf("caught: task labels are unbalanced across item boundary: %q", body)
							}
							for _, label := range labels.FindAllString(body, -1) {
								if strings.Contains(label, "</li>") || strings.Contains(label, "<div") || strings.Count(label, "<label") != 1 || strings.Count(label, "<input") != 1 {
									t.Errorf("caught: task label crossed item/block/control boundary: %q", label)
								}
							}
						}
					})
				}
			}
		}
	}
}
