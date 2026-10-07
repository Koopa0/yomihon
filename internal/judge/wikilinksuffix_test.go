package judge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

func suffixExistsVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "Notes/Projects/Atlas/README.md", "---\ntitle: Atlas page\n---\nbody\n")
	write(t, root, "Notes/Other/README.md", "---\ntitle: Other page\n---\nbody\n")
	write(t, root, "README.md", "---\ntitle: Root page\n---\nbody\n")
	write(t, root, "Diary/Private/Secret.md", "---\ntitle: Kept private\n---\nbody\n")
	write(t, root, schema.ContractRelPath, contractFixture(t, []string{"Diary"}, [2]string{`skip_basenames = ["README.md"]`, "skip_basenames = []"}))
	return root
}

func TestRunExistsPathSuffixKeepsMetadataAndPrivacy(t *testing.T) {
	t.Parallel()
	root := suffixExistsVault(t)
	write(t, root, "Notes/Alias.md", "---\ntitle: Alias holder\naliases: [Atlas/README]\n---\nbody\n")
	write(t, root, "Notes/Title.md", "---\ntitle: Atlas/README\n---\nbody\n")
	write(t, root, "Notes/Mixed/Secret.md", "---\ntitle: Public secret\n---\nbody\n")
	write(t, root, "Diary/Mixed/Secret.md", "---\ntitle: Private secret\n---\nbody\n")
	write(t, root, "Notes/Near.md", "---\ntitle: Ｗｉｄｅ/Ｎａｍｅ\n---\nbody\n")
	write(t, root, "Diary/Ｗｉｄｅ/Ｎａｍｅ.md", "---\ntitle: Hidden width\n---\nbody\n")
	write(t, root, "Writing/English.md", "---\ntype: lesson\ntitle: English page\ntitle_en: Lessons/English\n---\nbody\n")
	tests := []struct {
		query, want string
		exit        int
	}{
		{query: "Atlas/README", want: `{"query":"Atlas/README","matches":[{"path":"Notes/Alias.md","field":"alias","value":"Atlas/README"},{"path":"Notes/Title.md","field":"title","value":"Atlas/README"}]}`, exit: 0},
		{query: "Mixed/Secret", want: `{"query":"Mixed/Secret","matches":[{"path":"Notes/Mixed/Secret.md","field":"path","value":"Notes/Mixed/Secret.md"}],"withheld":true}`, exit: 0},
		{query: "wide/name", want: `{"query":"wide/name","matches":[],"near_matches":[{"path":"Notes/Near.md","field":"title","value":"Ｗｉｄｅ/Ｎａｍｅ"}]}`, exit: 1},
		{query: "Lessons/English", want: `{"query":"Lessons/English","matches":[{"path":"Writing/English.md","field":"title_en","value":"Lessons/English"}]}`, exit: 0},
	}
	// A width-fold-only location is not an exact name; it may not create the
	// private-name bit even though lexical matching finds another public title.
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()
			got, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: tt.query, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunExists() error = %v", err)
			}
			t.Log("invoked: actual suffix resolution contract")
			if exit != tt.exit || string(got) != tt.want+"\n" {
				t.Errorf("caught: exists metadata or privacy changed: RunExists(%q) = (%q, %d), want (%q, %d)", tt.query, got, exit, tt.want+"\n", tt.exit)
			}
		})
	}
}

func TestRunExistsPathSuffixSelectionIncludesUnprojectableIdentities(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		skipped, panicNote bool
	}{
		{name: "unreadable exact claimant", panicNote: true},
		{name: "skipped exact claimant", skipped: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, "Notes/Atlas/README.md", "---\ntitle: Exact claimant\n---\nbody\n")
			write(t, root, "Extra/Notes/Atlas/README.md", "---\ntitle: Deeper claimant\n---\nbody\n")
			replacements := [][2]string{{`skip_basenames = ["README.md"]`, "skip_basenames = []"}}
			if tt.skipped {
				// The basename skip removes both README notes from the readable
				// set; the ordinary note keeps a readable corpus alongside them.
				write(t, root, "Notes/Public.md", "---\ntitle: Public\n---\nbody\n")
				replacements = nil
			}
			write(t, root, schema.ContractRelPath, contractFixture(t, nil, replacements...))
			hooks := actionHooks{}
			if tt.panicNote {
				hooks.parseNote = func(rel string, data []byte, marks plannedMarks) note {
					if rel == "Notes/Atlas/README.md" {
						panic("controlled parse refusal")
					}
					return parseNoteWithMarks(rel, data, marks)
				}
			}
			prepared, err := prepareExistsWithHooks(t.Context(), &ExistsOptions{Root: root, Name: "Notes/Atlas/README.md", Format: FormatJSON}, hooks)
			t.Log("invoked: actual suffix resolution contract")
			if tt.panicNote && err == nil {
				if finishErr := prepared.finish(); finishErr != nil {
					t.Fatalf("finish unexpected positive observation: %v", finishErr)
				}
				t.Fatalf("caught: unreadable exact claimant discarded: got payload %q", prepared.stdout)
			}
			if tt.panicNote {
				if len(prepared.stdout) != 0 || !strings.Contains(err.Error(), "controlled parse refusal") {
					t.Fatalf("partial corpus refusal = (%q, %v)", prepared.stdout, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("prepareExists() error = %v", err)
			}
			if err := prepared.finish(); err != nil {
				t.Fatal(err)
			}
			want := "{\"query\":\"Notes/Atlas/README.md\",\"matches\":[]}\n"
			if string(prepared.stdout) != want || prepared.exit != 1 {
				t.Errorf("caught: resource became a note answer: (%q, %d)", prepared.stdout, prepared.exit)
			}
		})
	}
}

func TestRunExistsPathSuffixFinalAuthorityAndCancellation(t *testing.T) {
	t.Parallel()
	t.Run("changed authority", func(t *testing.T) {
		t.Parallel()
		root := suffixExistsVault(t)
		got, err := runPrepared(t.Context(), t, "exists", root, "Atlas/README", actionHooks{}, func() {
			file := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
			data, readErr := os.ReadFile(file) // #nosec G304 -- file is the schema path inside suffixExistsVault testing.TempDir
			if readErr != nil {
				t.Fatal(readErr)
			}
			changed := strings.Replace(string(data), `never_egress_dirs = ["Diary"]`, `never_egress_dirs = ["Diary", "Notes"]`, 1)
			if changed == string(data) {
				t.Fatal("policy stimulus not applied")
			}
			if writeErr := os.WriteFile(file, []byte(changed), 0o600); writeErr != nil { // #nosec G703 -- only the private suffixExistsVault testing.TempDir contract is rewritten
				t.Fatal(writeErr)
			}
		})
		t.Log("invoked: actual suffix resolution contract")
		if err == nil || got != nil || !strings.Contains(err.Error(), "privacy") {
			t.Errorf("caught: stale exists authority emitted: (%q, %v)", got, err)
		}
	})
	t.Run("cancelled public read", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		write(t, root, "Notes/Atlas/A.md", "---\ntitle: Public\n---\nbody\n")
		write(t, root, schema.ContractRelPath, contractFixture(t, nil))
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		got, err := runPrepared(ctx, t, "exists", root, "Atlas/A", actionHooks{afterScan: cancel}, nil)
		if got != nil || !errors.Is(err, context.Canceled) {
			t.Errorf("caught: cancelled exists emitted: (%q, %v)", got, err)
		}
	})
	t.Run("deadline during public read", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		write(t, root, "Notes/Atlas/A.md", "---\ntitle: Public\n---\nbody\n")
		write(t, root, schema.ContractRelPath, contractFixture(t, nil))
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			got, err := runPrepared(ctx, t, "exists", root, "Atlas/A", actionHooks{afterScan: func() { time.Sleep(2 * time.Second) }}, nil)
			if got != nil || !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("caught: expired exists emitted: (%q, %v)", got, err)
			}
		})
	})
}

func TestRunExistsPathSuffix(t *testing.T) {
	t.Parallel()
	root := suffixExistsVault(t)
	tests := []struct {
		name   string
		format Format
		want   string
		exit   int
	}{
		{name: "Atlas/README", format: FormatJSON, want: `{"query":"Atlas/README","matches":[{"path":"Notes/Projects/Atlas/README.md","field":"path","value":"Notes/Projects/Atlas/README.md"}]}`, exit: 0},
		{name: "Atlas/README.md", format: FormatJSON, want: `{"query":"Atlas/README.md","matches":[{"path":"Notes/Projects/Atlas/README.md","field":"path","value":"Notes/Projects/Atlas/README.md"}]}`, exit: 0},
		{name: "Notes/Projects/Atlas/README", format: FormatJSON, want: `{"query":"Notes/Projects/Atlas/README","matches":[{"path":"Notes/Projects/Atlas/README.md","field":"path","value":"Notes/Projects/Atlas/README.md"}]}`, exit: 0},
		{name: "Nope/README", format: FormatJSON, want: `{"query":"Nope/README","matches":[]}`, exit: 1},
		{name: "Private/Secret", format: FormatJSON, want: `{"query":"Private/Secret","matches":[],"withheld":true}`, exit: 0},
		{name: "Atlas/README", format: FormatHuman, want: "\"Atlas/README\" exists in 1 note(s):\n  Notes/Projects/Atlas/README.md (matched path)\n", exit: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/"+strconv.Itoa(int(tt.format)), func(t *testing.T) {
			t.Parallel()
			want := tt.want
			if tt.format == FormatJSON {
				want += "\n"
			}
			got, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: tt.name, Format: tt.format})
			if err != nil {
				t.Fatalf("RunExists() error = %v", err)
			}
			t.Log("invoked: actual suffix resolution contract")
			if diff := cmp.Diff(struct {
				Body string
				Exit int
			}{Body: want, Exit: tt.exit}, struct {
				Body string
				Exit int
			}{Body: string(got), Exit: exit}); diff != "" {
				t.Errorf("caught: exists path answer missing: RunExists(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if strings.Contains(string(got), "Diary/") {
				t.Errorf("caught: denied exists path disclosed: %s", got)
			}
		})
	}
}

func TestRunExistsPathSuffixResourcePriority(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Assets/chart.svg", "resource")
	write(t, root, "Notes/Assets/chart.svg.md", "---\ntitle: Competing note\n---\nbody\n")
	write(t, root, schema.ContractRelPath, contractFixture(t, nil))
	got, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: "Assets/chart.svg", Format: FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("invoked: actual suffix resolution contract")
	if exit != 1 || string(got) != "{\"query\":\"Assets/chart.svg\",\"matches\":[]}\n" {
		t.Errorf("caught: exact resource identity discarded: (%q, %d)", got, exit)
	}
}

func TestRunCheckPathSuffixConsumers(t *testing.T) {
	t.Parallel()
	root := suffixExistsVault(t)
	write(t, root, "Concepts/Source.md", "---\nbased_on: ['[[Atlas/README]]', '[[Nope/Source]]']\n---\n[[Atlas/README#Part]]\n[[Atlas/README#Absent]]\n[[Atlas/README#^absent]]\n![[Atlas/README#Absent]]\n[[Shared/Twin]]\n[[Nope/README]]\n[[Container/Page#Part]]\n")
	write(t, root, "Notes/Projects/Atlas/README.md", "## Part\n\nBlock payload. ^piece\n")
	write(t, root, "A/Shared/Twin.md", "body\n")
	write(t, root, "B/Shared/Twin.md", "body\n")
	write(t, root, "Notes/Container/Page.md", "![[Atlas/README#Part]]\n")
	got, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, All: true, Format: FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	if exit != 0 {
		t.Fatalf("check exit = %d", exit)
	}
	var rows []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(got)), "\n") {
		var f struct {
			RuleID     string  `json:"rule_id"`
			Target     *string `json:"target"`
			Field      *string `json:"field"`
			Evidence   string  `json:"evidence"`
			ResolvedTo *string `json:"resolved_to"`
		}
		if err := json.Unmarshal([]byte(line), &f); err != nil {
			t.Fatalf("decode actual JSONL: %v", err)
		}
		switch f.RuleID {
		case "link.broken", "link.section_missing", "link.block_missing", "embed.section_missing", "provenance.unresolved":
			target := ""
			if f.Target != nil {
				target = *f.Target
			}
			if f.Field != nil {
				target = *f.Field + ":" + f.Evidence
			}
			resolved := ""
			if f.ResolvedTo != nil {
				resolved = *f.ResolvedTo
			}
			rows = append(rows, f.RuleID+"@"+target+"->"+resolved)
		}
	}
	t.Log("invoked: actual suffix resolution contract")
	want := []string{
		"embed.section_missing@Atlas/README#Absent->Notes/Projects/Atlas/README.md",
		"link.block_missing@Atlas/README#^absent->Notes/Projects/Atlas/README.md",
		"link.broken@Nope/README->",
		"link.broken@Shared/Twin->",
		"link.section_missing@Atlas/README#Absent->Notes/Projects/Atlas/README.md",
		"provenance.unresolved@based_on:no note, alias, or lesson slug matches the reference->",
	}
	slices.Sort(want)
	slices.Sort(rows)
	if diff := cmp.Diff(want, rows); diff != "" {
		t.Errorf("caught: suffix check projections changed: %s", diff)
	}
}

func TestRunCoveragePathSuffix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, atlasContract)
	write(t, root, "Concepts/Atlas/Page.md", "---\ntitle: Page\ntype: concept\n---\nbody\n")
	write(t, root, "Atlases/Map.md", "---\ntitle: Map\ntype: atlas\nbased_on: '[[Atlas/Page]]'\n---\n[[Shared/Twin]]\n")
	write(t, root, "Concepts/A/Shared/Twin.md", "---\ntitle: A\ntype: concept\n---\nbody\n")
	write(t, root, "Concepts/B/Shared/Twin.md", "---\ntitle: B\ntype: concept\n---\nbody\n")
	got, exit, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"total_concepts":3,"domains":[{"domain":"(none)","concepts":3,"mounted":3,"pending_mount":0,"orphan":0}],"pending_mount":[],"orphans":[],"unrouted":[]}` + "\n"
	t.Log("invoked: actual suffix resolution contract")
	if exit != 0 || string(got) != want {
		t.Errorf("caught: suffix coverage mount missing: (%q,%d)", got, exit)
	}
}

func TestRunCheckPathSuffixSupersession(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	data, err := os.ReadFile(filepath.Join("testdata", "vault-supersession", filepath.FromSlash(schema.ContractRelPath)))
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, schema.ContractRelPath, string(data))
	write(t, root, "Writing/Atlas/Old.md", "---\ntitle: Old\ntype: lesson\nstatus: archived\nslug: old\n---\nbody\n")
	write(t, root, "Maps/Map.md", "---\ntitle: Map\ntype: topic-map\nstatus: ready\n---\n[[Atlas/Old]]\n")
	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	var targets []string
	for _, f := range findings {
		if f.RuleID == archivedNavigationRule && f.Target != nil && f.ResolvedTo != nil {
			targets = append(targets, f.Path+"@"+*f.Target+"->"+*f.ResolvedTo)
		}
	}
	t.Log("invoked: actual suffix resolution contract")
	if diff := cmp.Diff([]string{"Maps/Map.md@Atlas/Old->Writing/Atlas/Old.md"}, targets); diff != "" {
		t.Errorf("caught: suffix supersession missing: %s", diff)
	}
}

func TestRunCheckPathSuffixCourseDisk(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil))
	write(t, root, "Writing/Atlas/Lesson.md", "---\ntitle: Lesson\ntype: lesson\nstatus: ready\ndomain: golang\nslug: lesson\n---\nbody\n")
	write(t, root, "Maps/Course.md", "---\ntitle: Course\ntype: study-path\nstatus: ready\ndomain: golang\n---\n## Main {sequence=primary}\n- [[Atlas/Lesson]]\n- [[Nope/Lesson]]\n")
	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, f := range findings {
		if (f.RuleID == "map.disk_mismatch" || f.RuleID == "map.disk_unlisted") && f.Target != nil {
			rows = append(rows, string(f.RuleID)+"@"+*f.Target)
		}
	}
	t.Log("invoked: actual suffix resolution contract")
	if diff := cmp.Diff([]string{"map.disk_mismatch@Nope/Lesson"}, rows); diff != "" {
		t.Errorf("caught: suffix course disk projections changed: %s", diff)
	}
}
