package note_test

import (
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestNotePageNamesTheFieldAtFault is the claim this surface was built for:
// the page tells a reader what the command already knew, and names the field
// the fault is actually in. A note whose type is not one the schema declares
// used to be answered with a sentence about its status, which was legal —
// pointing a reader at the one field that was fine.
func TestNotePageNamesTheFieldAtFault(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		rel      string
		body     string
		wantSaid []string
		wantNot  []string
	}{
		{
			name:     "a type outside the schema's list is reported against type",
			rel:      "Concepts/golang/Memo.md",
			body:     "---\ntitle: Memo\ntype: memorandum\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n",
			wantSaid: []string{"<code>type</code>", "memorandum"},
			// The status is legal for every type the schema declares, and the
			// schema declares no statuses at all for a type it does not know —
			// so nothing here can rule on it. Saying it is outside the list
			// points the reader at the one field that is fine, and it used to
			// be said twice, beside the sentence that names the real fault.
			wantNot: []string{wording.StatusOutsideList.In(wording.ZhHant)},
		},
		{
			name:     "a declared type with a status outside its list still says so",
			rel:      "Concepts/golang/Off.md",
			body:     "---\ntitle: Off\ntype: concept\ndomain: golang\nstatus: nonesuch\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n",
			wantSaid: []string{wording.StatusOutsideList.In(wording.ZhHant), "nonesuch"},
		},
		{
			name:     "a slug the schema's shape rejects is reported against slug",
			rel:      "Writing/lessons/golang/L01.md",
			body:     "---\ntitle: L01\ntype: lesson\ndomain: golang\nstatus: draft\nslug: Not Kebab\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n",
			wantSaid: []string{"<code>slug</code>", "Not Kebab"},
		},
		{
			name:     "a frontmatter key the schema does not know is named",
			rel:      "Concepts/golang/Extra.md",
			body:     "---\ntitle: Extra\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nnot_a_field: 1\n---\n\nbody\n",
			wantSaid: []string{"<code>not_a_field</code>"},
		},
		{
			name:     "a missing required field is named",
			rel:      "Concepts/golang/Bare.md",
			body:     "---\ntitle: Bare\ntype: concept\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n",
			wantSaid: []string{"<code>domain</code>"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			full := filepath.Join(root, filepath.FromSlash(tc.rel))
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(full, []byte(tc.body), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			srv := newServerWithContract(t, root, loadHomeContract(t))

			code, page := get(t, srv.Client(), srv.URL+"/notes/"+tc.rel)
			if code != http.StatusOK {
				t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
			}
			for _, want := range tc.wantSaid {
				if !strings.Contains(page, want) {
					t.Errorf("the page never says %q; a reader is left with the silence this replaced", want)
				}
			}
			for _, unwanted := range tc.wantNot {
				if strings.Contains(page, unwanted) {
					t.Errorf("the page says %q, which points at a field that is not the one at fault", unwanted)
				}
			}
		})
	}
}

// TestNotePageShowsTheFolderTheDomainRuleCompared holds the explanation to the
// first folder under the declared root, including notes nested further below it.
func TestNotePageShowsTheFolderTheDomainRuleCompared(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		root string
		rel  string
	}{
		{name: "top level", root: "Concepts", rel: "Concepts/japanese/nested/Deep.md"},
		{name: "nested", root: "Writing/lessons", rel: "Writing/lessons/japanese/nested/Deep.md"},
		{name: "renamed", root: "Writing/courses", rel: "Writing/courses/japanese/nested/Deep.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			contract := domainNoticeContract(t, root, tt.root)
			writeDomainNoticeNote(t, root, tt.rel)
			log := slog.New(slog.DiscardHandler)
			store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
			writer := openStatusWriter(t, source, contract, contract.Governance())
			mux := http.NewServeMux()
			note.New(&note.Sources{
				Source: source, Status: writer.Authority, Snapshot: store.Current,
				ObservedStatus: writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt, Continuation: noMark, Log: log,
			}).Register(mux)

			for _, language := range []struct {
				lang wording.Lang
				want string
			}{
				{lang: wording.ZhHant, want: "<code>domain</code> 寫的 <code>golang</code>與依宣告的根目錄判定的領域資料夾 <code>japanese</code> 不一致。"},
				{lang: wording.En, want: "<code>domain</code> is written as <code>golang</code>, which does not match the domain folder selected by the declared root, <code>japanese</code>."},
			} {
				t.Run(string(language.lang), func(t *testing.T) {
					t.Parallel()
					rr := httptest.NewRecorder()
					req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/"+tt.rel, http.NoBody)
					req.Header.Set("Cookie", wording.CookieName+"="+string(language.lang))
					mux.ServeHTTP(rr, req)
					if rr.Code != http.StatusOK {
						t.Fatalf("GET note status = %d, want %d", rr.Code, http.StatusOK)
					}
					if !strings.Contains(rr.Body.String(), language.want) {
						t.Errorf("GET note omitted the declared domain-folder explanation %q", language.want)
					}
				})
			}
		})
	}
}

func TestNotePageKeepsCapturedDomainRoots(t *testing.T) {
	t.Parallel()
	const rel = "Writing/lessons/japanese/nested/Deep.md"
	log := slog.New(slog.DiscardHandler)
	firstRoot := t.TempDir()
	firstContract := domainNoticeContract(t, firstRoot, "Writing/lessons")
	writeDomainNoticeNote(t, firstRoot, rel)
	firstStore, source := newSnapshotStore(t, firstRoot, log, firstContract, firstContract.Governance())
	writer := openStatusWriter(t, source, firstContract, firstContract.Governance())
	secondRoot := t.TempDir()
	secondContract := domainNoticeContract(t, secondRoot, "Writing")
	writeDomainNoticeNote(t, secondRoot, rel)
	secondStore, _ := newSnapshotStore(t, secondRoot, log, secondContract, secondContract.Governance())

	for _, tt := range []struct {
		lang wording.Lang
		want [2]string
	}{
		{lang: wording.ZhHant, want: [2]string{
			"<code>domain</code> 寫的 <code>golang</code>與依宣告的根目錄判定的領域資料夾 <code>japanese</code> 不一致。",
			"<code>domain</code> 寫的 <code>golang</code>與依宣告的根目錄判定的領域資料夾 <code>lessons</code> 不一致。",
		}},
		{lang: wording.En, want: [2]string{
			"<code>domain</code> is written as <code>golang</code>, which does not match the domain folder selected by the declared root, <code>japanese</code>.",
			"<code>domain</code> is written as <code>golang</code>, which does not match the domain folder selected by the declared root, <code>lessons</code>.",
		}},
	} {
		t.Run(string(tt.lang), func(t *testing.T) {
			t.Parallel()
			current := firstStore.Current()
			mux := http.NewServeMux()
			note.New(&note.Sources{
				Source: source, Status: writer.Authority,
				Snapshot: func() *snapshot.Generation {
					captured := current
					current = secondStore.Current()
					return captured
				},
				ObservedStatus: writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt, Continuation: noMark, Log: log,
			}).Register(mux)
			for i, want := range tt.want {
				rr := httptest.NewRecorder()
				req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/"+rel, http.NoBody)
				req.Header.Set("Cookie", wording.CookieName+"="+string(tt.lang))
				mux.ServeHTTP(rr, req)
				if rr.Code != http.StatusOK {
					t.Fatalf("GET note %d status = %d, want %d", i, rr.Code, http.StatusOK)
				}
				if !strings.Contains(rr.Body.String(), want) {
					t.Errorf("GET note %d omitted its captured domain-folder explanation %q", i, want)
				}
			}
		})
	}
}

func domainNoticeContract(t *testing.T, root, domainRoot string) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read contract fixture: %v", err)
	}
	const declaration = `domain_equals_folder_under = ["Concepts"]`
	if strings.Count(string(data), declaration) != 1 {
		t.Fatal("fixture must have exactly one domain-root declaration")
	}
	data = []byte(strings.Replace(string(data), declaration, `domain_equals_folder_under = ["`+domainRoot+`"]`, 1))
	tree, openErr := os.OpenRoot(root)
	if openErr != nil {
		t.Fatalf("open fixture root: %v", openErr)
	}
	t.Cleanup(func() {
		if closeErr := tree.Close(); closeErr != nil {
			t.Errorf("close fixture root: %v", closeErr)
		}
	})
	contractPath := filepath.FromSlash(schema.ContractRelPath)
	if mkdirErr := tree.MkdirAll(filepath.Dir(contractPath), 0o750); mkdirErr != nil {
		t.Fatalf("mkdir contract: %v", mkdirErr)
	}
	if writeErr := tree.WriteFile(contractPath, data, 0o600); writeErr != nil {
		t.Fatalf("write contract: %v", writeErr)
	}
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	return contract
}

func writeDomainNoticeNote(t *testing.T, root, rel string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir note: %v", err)
	}
	const body = "---\ntitle: Deep\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n"
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
}

// TestNotePageEscapesTheNotesOwnTextInANotice covers what a notice is made of:
// the note's own words, quoted back to a reader. A note is a file anyone can
// write, so a value that looks like markup has to arrive as the characters the
// author typed rather than as anything the page acts on.
func TestNotePageEscapesTheNotesOwnTextInANotice(t *testing.T) {
	t.Parallel()

	const rel = "Writing/lessons/golang/L02.md"
	const hostile = `<b onclick="x">y</b>`
	body := "---\ntitle: L02\ntype: lesson\ndomain: golang\nstatus: draft\nslug: '" + hostile + "'\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"

	root := t.TempDir()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "&lt;b onclick=") {
		t.Errorf("the notice does not carry the author's characters escaped; page did not contain the escaped form")
	}
	if strings.Contains(page, hostile) {
		t.Errorf("the page carries %q verbatim, so a note's own text reached the reader as markup", hostile)
	}
}

// TestNotePageReadsTogetherForAStatusThatIsNotText holds the two sentences a
// non-text status draws, because they are said side by side and a reader has
// to be able to act on the pair. One explains why no status could be read; the
// other names what was written. Before the first was completed it said the
// value was missing or not single, which is false of 123 — a number is both
// present and single — leaving the two sentences disagreeing about whether
// there was a value at all.
func TestNotePageReadsTogetherForAStatusThatIsNotText(t *testing.T) {
	t.Parallel()

	const rel = "Concepts/golang/Numeric.md"
	body := "---\ntitle: Numeric\ntype: concept\ndomain: golang\nstatus: 123\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n"

	root := t.TempDir()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "不是文字") {
		t.Error("the page does not say the value is not text, so its account of why nothing could be read omits this note's actual cause")
	}
	if !strings.Contains(page, "<code>123</code>") {
		t.Error("the page does not name what was written")
	}
}

// TestThePageNamesTheFolderTheJudgeActuallyCompared checks the command and page
// against the declared folder, with an agreeing note to also pin the comparison.
func TestThePageNamesTheFolderTheJudgeActuallyCompared(t *testing.T) {
	t.Parallel()

	const rel = "Concepts/japanese/nested/Deep.md"
	body := "---\ntitle: Deep\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n"

	root := t.TempDir()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	contractBytes, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read the contract fixture: %v", err)
	}
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if mkdirErr := os.MkdirAll(filepath.Dir(contractPath), 0o750); mkdirErr != nil {
		t.Fatalf("mkdir contract: %v", mkdirErr)
	}
	// The judging commands fail closed without an egress declaration, and the
	// shared fixture declares none, so this one is written with an empty list:
	// the question here is which folder a rule compared, not what may leave.
	contractBytes = append(contractBytes, []byte("\n[privacy]\nnever_egress_dirs = []\n")...)
	if writeErr := os.WriteFile(contractPath, contractBytes, 0o600); writeErr != nil { // #nosec G703 -- the path is this test's own t.TempDir() joined with a fixed name
		t.Fatalf("write contract: %v", writeErr)
	}

	// A second note, alike but for its domain matching the folder the rule is
	// supposed to compare. It binds the comparison as well as the naming: the
	// finding's message would still say "japanese" if the condition beside it
	// started comparing some other segment, and this note is what notices —
	// under the rule as written it draws nothing, and under a rule comparing
	// the folder the note merely sits in it draws a finding.
	const agreeing = "Concepts/japanese/nested/Agrees.md"
	agreeingBody := "---\ntitle: Agrees\ntype: concept\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n"
	if writeErr := os.WriteFile(filepath.Join(filepath.Dir(full), "Agrees.md"), []byte(agreeingBody), 0o600); writeErr != nil { // #nosec G703 -- a path under this test's own t.TempDir()
		t.Fatalf("write the agreeing note: %v", writeErr)
	}

	findings, err := judge.Check(t.Context(), root)
	if err != nil {
		t.Fatalf("judge.Check: %v", err)
	}
	for i := range findings {
		if findings[i].RuleID == "schema.domain_folder" && findings[i].Path == agreeing {
			t.Errorf("a note whose domain matches the folder the rule compares drew %q", findings[i].Message)
		}
	}
	seen := 0
	for i := range findings {
		if findings[i].RuleID != "schema.domain_folder" || findings[i].Path != rel {
			continue
		}
		seen++
		if want := `domain "golang" does not match its folder japanese`; findings[i].Message != want {
			t.Errorf("domain finding message = %q, want %q", findings[i].Message, want)
		}
	}
	if seen != 1 {
		t.Fatalf("the judge reported %d folder findings for this note; the fixture is meant to draw exactly one", seen)
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "<code>japanese</code>") {
		t.Error("the page does not name japanese, the declared domain folder")
	}
}

// TestFrontmatterSchemaNoticesQuoteTheCapturedParser keeps parser evidence on
// the note's schema face, beside the explanation rather than only in the rail.
func TestFrontmatterSchemaNoticesQuoteTheCapturedParser(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, diagnostic string
	}{
		{"invalid mapping", "---\ntitle: Bad: yaml\n---\nReadable body.\n", "frontmatter is not valid YAML: yaml: line 2: mapping values are not allowed in this context"},
		{"duplicate key", "---\ntitle: Bad\ndup: one\ndup: two\n---\nReadable body.\n", "frontmatter is not valid YAML: yaml: unmarshal errors:\n  line 4: mapping key \"dup\" already defined at line 3"},
		{"HTML-sensitive duplicate key", "---\ntitle: Bad\n'<script>&</script>': one\n'<script>&</script>': two\n---\nReadable body.\n", "frontmatter is not valid YAML: yaml: unmarshal errors:\n  line 4: mapping key \"<script>&</script>\" already defined at line 3"},
	} {
		for _, chrome := range []struct {
			lang wording.Lang
			want string
		}{
			{wording.ZhHant, "frontmatter 不是有效的 YAML。解析器指出："},
			{wording.En, "The frontmatter is not valid YAML. The parser reported: "},
		} {
			t.Run(tc.name+"/"+string(chrome.lang), func(t *testing.T) {
				t.Parallel()
				root := writeNotes(t, map[string]string{"Writing/Bad.md": tc.body})
				server := newServerWithContract(t, root, loadContract(t))
				page := frontmatterNoticePage(t, server, chrome.lang, "Writing/Bad.md")
				_, notice, ok := strings.Cut(page, `id="schema-notices"`)
				if !ok {
					t.Fatal("invalid YAML has no schema-notices panel")
				}
				notice, _, ok = strings.Cut(notice, "</div>")
				if !ok {
					t.Fatal("schema-notices panel has no closing tag")
				}
				for _, want := range []string{chrome.want, "<code>" + html.EscapeString(tc.diagnostic) + "</code>"} {
					if !strings.Contains(notice, want) {
						t.Errorf("schema notice omitted %q: %s", want, notice)
					}
				}
				for _, unwanted := range []string{"schema.frontmatter", "這個頁面還沒有它的說法", "this page has no words for it yet", "<script>"} {
					if strings.Contains(notice, unwanted) {
						t.Errorf("schema notice contains %q: %s", unwanted, notice)
					}
				}
			})
		}
	}
}

func TestFrontmatterSchemaNoticeDistinguishesMissingAndUnclosedBlocks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		contract   func(*testing.T) *schema.Contract
		hasNotice  bool
	}{
		{"required block missing", "Readable body.\n", loadContractRequiringFrontmatter, true},
		{"optional block missing", "Readable body.\n", loadContract, false},
		{"fence never closes", "---\ntitle: Unclosed\nReadable body.\n", loadContractRequiringFrontmatter, false},
	} {
		for _, chrome := range []struct {
			lang    wording.Lang
			missing string
		}{
			{wording.ZhHant, "這份筆記需要 frontmatter，但沒有 frontmatter 區塊。"},
			{wording.En, "This note requires frontmatter, but no frontmatter block is present."},
		} {
			t.Run(tc.name+"/"+string(chrome.lang), func(t *testing.T) {
				t.Parallel()
				root := writeNotes(t, map[string]string{"Writing/Bad.md": tc.body})
				server := newServerWithContract(t, root, tc.contract(t))
				page := frontmatterNoticePage(t, server, chrome.lang, "Writing/Bad.md")
				_, notice, hasNotice := strings.Cut(page, `id="schema-notices"`)
				if hasNotice != tc.hasNotice {
					t.Fatalf("schema-notices present=%v, want %v", hasNotice, tc.hasNotice)
				}
				if hasNotice {
					notice, _, _ = strings.Cut(notice, "</div>")
					if !strings.Contains(notice, chrome.missing) {
						t.Errorf("missing block notice lacks truthful explanation %q: %s", chrome.missing, notice)
					}
					for _, unwanted := range []string{"valid YAML", "有效的 YAML", "parser", "解析器", "<code>"} {
						if strings.Contains(notice, unwanted) {
							t.Errorf("missing block notice incorrectly contains %q: %s", unwanted, notice)
						}
					}
				}
				if tc.name == "fence never closes" && !strings.Contains(page, wording.FrontmatterNeverCloses.In(chrome.lang)) {
					t.Error("unclosed fence lost its existing explanation")
				}
			})
		}
	}
}

func frontmatterNoticePage(t *testing.T, server *httptest.Server, lang wording.Lang, rel string) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/notes/"+rel, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", wording.CookieName+"="+string(lang))
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("close response: %v", closeErr)
		}
	}()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("frontmatter note HTTP %d, want 200", response.StatusCode)
	}
	page := string(raw)
	if !strings.Contains(page, "Readable body.") {
		t.Error("frontmatter explanation lost the readable body")
	}
	return page
}
