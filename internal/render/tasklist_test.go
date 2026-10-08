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

func TestTaskSequenceRolesLeaveOnlyAuthoredNames(t *testing.T) {
	t.Parallel()
	for _, task := range []struct{ marker, input string }{
		{marker: " ", input: `<input disabled="" type="checkbox">`},
		{marker: "x", input: `<input checked="" disabled="" type="checkbox">`},
		{marker: "X", input: `<input checked="" disabled="" type="checkbox">`},
		{marker: "/", input: `<input disabled="" type="checkbox" data-task="/">`},
		{marker: "-", input: `<input disabled="" type="checkbox" data-task="-">`},
		{marker: ">", input: `<input disabled="" type="checkbox" data-task="&gt;">`},
	} {
		for _, role := range []string{"primary", "local", "none"} {
			t.Run(task.marker+role, func(t *testing.T) {
				t.Parallel()
				source := "- [" + task.marker + "] Task {sequence=" + role + "}\n- Plain row {sequence=" + role + "}\n"
				want := "<ul>\n<li><label class=\"y-task\">" + task.input + " Task</label></li>\n<li>Plain row</li>\n</ul>\n"
				got := newRenderer(t, nil, nil, nil).HTML("Task.md", "", source, wording.En).HTML
				t.Logf("invoked: task-role-state marker=%q role=%q", task.marker, role)
				if got != want {
					t.Errorf("caught: task role HTML = %q, want %q", got, want)
				}
				if strings.Contains(got, "{sequence=") {
					t.Errorf("caught: task name retains declaration in %q", got)
				}
			})
		}
	}
}

func TestTaskSequenceRolesRespectInlineBlockBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, source, want string }{
		{
			name:   "loose",
			source: "- [x] Task {sequence=local}\n\n- Plain row {sequence=local}\n",
			want:   "<ul>\n<li>\n<p><label class=\"y-task\"><input checked=\"\" disabled=\"\" type=\"checkbox\"> Task</label></p>\n</li>\n<li>\n<p>Plain row</p>\n</li>\n</ul>\n",
		},
		{
			name:   "nested",
			source: "- [ ] Parent {sequence=primary}\n  - [x] Child {sequence=local}\n",
			want:   "<ul>\n<li><label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Parent</label>\n<ul>\n<li><label class=\"y-task\"><input checked=\"\" disabled=\"\" type=\"checkbox\"> Child</label></li>\n</ul>\n</li>\n</ul>\n",
		},
		{
			name:   "ordered",
			source: "1. [x] First {sequence=none}\n2. [ ] Second {sequence=local}\n",
			want:   "<ol>\n<li><label class=\"y-task\"><input checked=\"\" disabled=\"\" type=\"checkbox\"> First</label></li>\n<li><label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Second</label></li>\n</ol>\n",
		},
		{
			name:   "second paragraph",
			source: "- [x] Own {sequence=local}\n\n  Another paragraph {sequence=local}\n",
			want:   "<ul>\n<li>\n<p><label class=\"y-task\"><input checked=\"\" disabled=\"\" type=\"checkbox\"> Own</label></p>\n<p>Another paragraph {sequence=local}</p>\n</li>\n</ul>\n",
		},
		{
			name:   "soft continuation",
			source: "- [ ] Own\n  continuation {sequence=local}\n",
			want:   "<ul>\n<li><label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Own\ncontinuation {sequence=local}</label></li>\n</ul>\n",
		},
		{
			name:   "inline formatting retained",
			source: "- [x] **Own** {sequence=local}\n",
			want:   "<ul>\n<li><label class=\"y-task\"><input checked=\"\" disabled=\"\" type=\"checkbox\"> <strong>Own</strong></label></li>\n</ul>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := newRenderer(t, nil, nil, nil).HTML("Task.md", "", tt.source, wording.En).HTML
			if got != tt.want {
				t.Errorf("caught: task own-block HTML = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBlockTaskSequenceRoleIsAbsentFromItsEarlyName(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "Target.md"}}, nil, transclusions{"Target.md": "- [ ] Embedded {sequence=local}\n"})
	result := r.HTMLIn("compare-a-", "Host.md", "", "- [x] Task ![[Target]] {sequence=local}\n", wording.En)
	for _, body := range []string{result.HTML, render.StripAnchors(result.HTML)} {
		t.Log("invoked: early-block-task-role-name")
		const label = `<label class="y-task"><input checked="" disabled="" type="checkbox"> <span class="y-offscreen">Task</span></label>`
		if !strings.Contains(body, label) {
			t.Errorf("caught: early block task name = %q, want %q", body, label)
		}
		const visible = label + ` Task <div class="embed">`
		if !strings.Contains(body, visible) {
			t.Errorf("caught: block task visible own wording = %q, want %q", body, visible)
		}
		const child = `<label class="y-task"><input disabled="" type="checkbox"> Embedded</label>`
		if !strings.Contains(body, child) || strings.Index(body, child) < strings.Index(body, label)+len(label) {
			t.Errorf("caught: embedded child task crossed parent name boundary: %q", body)
		}
		if strings.Contains(body, "{sequence=") || strings.ContainsAny(body, "\ue000\ue001\ue002\ue003") {
			t.Errorf("caught: block task display leaked role or placeholder: %q", body)
		}
		if strings.Count(body, `<label class="y-task">`) != 2 || strings.Count(body, "</label>") != 2 || strings.Count(body, `type="checkbox"`) != 2 {
			t.Errorf("caught: block task labels/control counts = %q, want two independent tasks", body)
		}
	}
}

func TestTaskSequenceRoleQuotationAndInvalidDeclarationsStayVisible(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, source, own string }{
		{name: "code", source: "Task `{sequence=local}`", own: "Task <code>{sequence=local}</code>"},
		{name: "emphasis", source: "*Task {sequence=local}*", own: "<em>Task {sequence=local}</em>"},
		{name: "link", source: "[Task {sequence=local}](https://example.test/)", own: `<a href="https://example.test/" target="_blank" rel="external noopener noreferrer" referrerpolicy="no-referrer">Task {sequence=local}<span class="y-offscreen"> (opens in a new tab)</span></a>`},
		{name: "unknown", source: "Task {sequence=supplementary}", own: "Task {sequence=supplementary}"},
		{name: "duplicate", source: "Task {sequence=primary} {sequence=local}", own: "Task {sequence=primary} {sequence=local}"},
		{name: "incomplete", source: "Task {sequence=local", own: "Task {sequence=local"},
		{name: "nonterminal", source: "Task {sequence=local} tail", own: "Task {sequence=local} tail"},
		{name: "marker only", source: "{sequence=local}", own: "{sequence=local}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			source := "- [x] " + tt.source + "\n- " + tt.source + "\n"
			want := "<ul>\n<li><label class=\"y-task\"><input checked=\"\" disabled=\"\" type=\"checkbox\"> " + tt.own + "</label></li>\n<li>" + tt.own + "</li>\n</ul>\n"
			got := newRenderer(t, nil, nil, nil).HTML("Task.md", "", source, wording.En).HTML
			if got != want {
				t.Errorf("caught: literal task/plain role HTML = %q, want %q", got, want)
			}
		})
	}
}
