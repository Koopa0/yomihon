package pages

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEmptyPathAndMapGuideFirstRun holds the four empty signals the path, map
// and folder shelves carry, and the one completable step each of them is: a
// folder with no contract, a contract this process never read, a contract that
// declares what fills the two declared shelves, and one that declares nothing
// for them at all.
func TestEmptyPathAndMapGuideFirstRun(t *testing.T) {
	t.Parallel()

	// The words the declared shelves answer for are this test's, written into
	// a contract and read back through it, so a word the interface spelled for
	// itself cannot pass for one a vault declared.
	declared := declaringContract(t, "trail-guide", "atlas", "chart")
	folderGuide := guide(wording.FolderIndexEmpty, wording.IndexDeclaredEmptyNext)
	cases := []struct {
		name     string
		contract ContractState
		roles    schema.NavigationRoles
		path     func(lang wording.Lang) string
		mapState func(lang wording.Lang) string
		folder   func(lang wording.Lang) string
	}{
		{
			name:     "folder with no contract",
			contract: ContractAbsent,
			roles:    declared,
			path:     guide(wording.IndexUngoverned, wording.IndexUngovernedNext),
			mapState: guide(wording.IndexUngoverned, wording.IndexUngovernedNext),
			folder:   guide(wording.IndexUngoverned, wording.IndexUngovernedNext),
		},
		{
			name:     "contract that reached the folder after yomihon started",
			contract: ContractUnloaded,
			roles:    declared,
			path:     guide(wording.IndexContractUnloaded, wording.IndexContractUnloadedNext),
			mapState: guide(wording.IndexContractUnloaded, wording.IndexContractUnloadedNext),
			folder:   guide(wording.IndexContractUnloaded, wording.IndexContractUnloadedNext),
		},
		{
			name:     "contract that declares what fills each shelf",
			contract: ContractGoverning,
			roles:    declared,
			path:     declaredGuide("trail-guide"),
			mapState: declaredGuide("atlas", "chart"),
			folder:   folderGuide,
		},
		{
			// Nothing can be declared onto these two shelves, so there is no
			// edit to offer and the page says nothing rather than name one
			// that would leave the reader where they started. The folder shelf
			// is unaffected: a file added to it is on it.
			name:     "contract that declares nothing for either shelf",
			contract: ContractGoverning,
			roles:    schema.NavigationRoles{},
			path:     silent,
			mapState: silent,
			folder:   folderGuide,
		},
	}
	for _, tt := range cases {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(tt.name+"/"+string(lang), func(t *testing.T) {
				t.Parallel()
				pathView := NewPathIndex(nil, tt.roles, nav.Closure{}, tt.contract, lang, nil)
				mapView := NewMapIndex(nil, tt.roles, nav.Closure{}, tt.contract, lang, nil)
				folderView := NewFolderIndex(&nav.Model{}, tt.contract, lang, nil)
				for _, view := range []struct {
					name string
					want string
					got  string
				}{
					{name: "paths", want: tt.path(lang), got: pathView.Shelf.Empty},
					{name: "maps", want: tt.mapState(lang), got: mapView.Shelf.Empty},
					{name: "folders", want: tt.folder(lang), got: folderView.Shelf.Empty},
				} {
					assertEmptyGuide(t, view.name, view.got, view.want, lang)
				}
				blocks := NewDeskBlocks(&nav.Model{}, tt.roles, tt.contract, lang, nil)
				for _, block := range blocks {
					if block.Mode != pathMode && block.Mode != mapMode && block.Mode != folderMode {
						continue
					}
					want := tt.path(lang)
					switch block.Mode {
					case mapMode:
						want = tt.mapState(lang)
					case folderMode:
						want = tt.folder(lang)
					}
					assertEmptyGuide(t, "desk/"+block.Mode, block.Shelf.Empty, want, lang)
				}
			})
		}
	}
}

// guide is what a shelf says in a contract state that has nothing to do with a
// declaration: the state it is in, and the one step out of it.
func guide(state, step wording.Phrase) func(wording.Lang) string {
	return func(lang wording.Lang) string { return wording.JoinGuide(state, step, lang) }
}

// declaredGuide is what a shelf filled by a declaration says while nothing has
// been declared onto it. The words are the caller's, which is the half of this
// expectation the page cannot supply to itself: they reach the page through a
// contract file and reach this string directly.
func declaredGuide(types ...string) func(wording.Lang) string {
	return func(lang wording.Lang) string {
		format := wording.NoDeclaredTypeEmptyFmt
		if len(types) > 1 {
			format = wording.NoDeclaredTypesEmptyFmt
		}
		return fmt.Sprintf(format.In(lang), strings.Join(types, wording.ListSeparator.In(lang)))
	}
}

// silent is a shelf with nothing to say and no step to offer.
func silent(wording.Lang) string { return "" }

// declaringContract writes a contract declaring the given navigation
// vocabulary and reads the roles back out of it, so every word under test
// travels the way a vault's own words travel.
func declaringContract(t *testing.T, pathType string, mapTypes ...string) schema.NavigationRoles {
	t.Helper()
	declared := append([]string{pathType}, mapTypes...)
	quoted := func(words []string) string { return `"` + strings.Join(words, `", "`) + `"` }
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	text := `schema_version = "1"

[enums]
type = [` + quoted(declared) + `]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type"]

[rules]

[scan]
knowledge_dirs = ["Notes"]

[navigation]
path_types = [` + quoted([]string{pathType}) + `]
map_types = [` + quoted(mapTypes) + `]

[artifacts]
non_instance_dirs = ["System/templates"]

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
`
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("schema.LoadFile = %v", err)
	}
	roles := contract.NavigationRoles()
	if got := roles.PathTypes(); len(got) != 1 || got[0] != pathType {
		t.Fatalf("the fixture contract declares path types %q, want %q", got, pathType)
	}
	return roles
}

// everyEmptyStep is every next step this slot can hold. Carrying the right one
// is only half the claim: a slot that also carried another would hand the
// reader a recovery that is not theirs, and each of these three costs something
// different to follow.
var everyEmptyStep = []wording.Phrase{
	wording.IndexUngovernedNext,
	wording.IndexContractUnloadedNext,
	wording.IndexDeclaredEmptyNext,
}

func assertEmptyGuide(t *testing.T, where, got, want string, lang wording.Lang) {
	t.Helper()
	if want == "" {
		if got != "" {
			t.Errorf("%s spoke where it has no step to offer: %q", where, got)
		}
		return
	}
	if !strings.Contains(got, want) {
		t.Errorf("%s empty guide want %q; got %q", where, want, got)
	}
	for _, other := range everyEmptyStep {
		step := other.In(lang)
		if strings.Contains(want, step) || !strings.Contains(got, step) {
			continue
		}
		t.Errorf("%s empty guide also offers %q, which is another shelf's way out: %q", where, step, got)
	}
	if strings.Contains(got, "宣告") {
		t.Errorf("%s still uses 宣告 jargon: %q", where, got)
	}
}

// TestEmptyPathAndMapIndexPagesRenderTheGuide keeps the empty slot on the mode
// index pages themselves, not only in the view the desk narrows from. Every
// state a reader without a listing can be in is drawn, because the sentence
// that only the view carries is a sentence nobody reads.
func TestEmptyPathAndMapIndexPagesRenderTheGuide(t *testing.T) {
	t.Parallel()

	declared := declaringContract(t, "trail-guide", "atlas", "chart")
	for _, tt := range []struct {
		name     string
		contract ContractState
		path     func(lang wording.Lang) string
		mapState func(lang wording.Lang) string
		folder   func(lang wording.Lang) string
	}{
		{
			name:     "no contract in the folder",
			contract: ContractAbsent,
			path:     guide(wording.IndexUngoverned, wording.IndexUngovernedNext),
			mapState: guide(wording.IndexUngoverned, wording.IndexUngovernedNext),
			folder:   guide(wording.IndexUngoverned, wording.IndexUngovernedNext),
		},
		{
			name:     "contract this process never loaded",
			contract: ContractUnloaded,
			path:     guide(wording.IndexContractUnloaded, wording.IndexContractUnloadedNext),
			mapState: guide(wording.IndexContractUnloaded, wording.IndexContractUnloadedNext),
			folder:   guide(wording.IndexContractUnloaded, wording.IndexContractUnloadedNext),
		},
		{
			name:     "contract that declares what fills each shelf",
			contract: ContractGoverning,
			path:     declaredGuide("trail-guide"),
			mapState: declaredGuide("atlas", "chart"),
			folder:   guide(wording.FolderIndexEmpty, wording.IndexDeclaredEmptyNext),
		},
	} {
		for _, mode := range []string{pathMode, mapMode, folderMode} {
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				t.Run(tt.name+"/"+mode+"/"+string(lang), func(t *testing.T) {
					t.Parallel()
					var buf bytes.Buffer
					view := emptyIndexView(mode, declared, tt.contract, lang)
					if err := ListIndex(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
						t.Fatalf("render: %v", err)
					}
					html := buf.String()
					if !strings.Contains(html, `data-index-empty`) {
						t.Fatalf("%s index did not render the empty slot: %q", mode, html)
					}
					want := tt.path(lang)
					switch mode {
					case mapMode:
						want = tt.mapState(lang)
					case folderMode:
						want = tt.folder(lang)
					}
					assertEmptyGuide(t, mode, html, want, lang)
				})
			}
		}
	}
}

// emptyIndexView builds one mode's index over a vault holding nothing of that
// kind, which is the only state in which the slot under test is drawn at all.
func emptyIndexView(mode string, roles schema.NavigationRoles, contract ContractState, lang wording.Lang) ListIndexView {
	switch mode {
	case mapMode:
		return NewMapIndex(nil, roles, nav.Closure{}, contract, lang, nil)
	case folderMode:
		return NewFolderIndex(&nav.Model{}, contract, lang, nil)
	default:
		return NewPathIndex(nil, roles, nav.Closure{}, contract, lang, nil)
	}
}

// TestContractStateReadsTheFileOffTheScanThePageListsFrom is the half a
// hand-written state cannot hold. The page's claim that the contract is in the
// folder is the file list of the same reading it lists notes from, so a scan
// that stopped carrying the contract file would quietly send this reader back
// to the sentence saying the folder has none — with every test above still
// green, because each of them states the answer instead of finding it.
func TestContractStateReadsTheFileOffTheScanThePageListsFrom(t *testing.T) {
	t.Parallel()

	contract, err := os.ReadFile(filepath.Join("..", "..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	for _, tt := range []struct {
		name  string
		files map[string]string
		want  ContractState
	}{
		{
			name:  "folder with no contract",
			files: map[string]string{"Notes/A.md": "# A\n"},
			want:  ContractAbsent,
		},
		{
			name: "contract this process never loaded",
			files: map[string]string{
				"Notes/A.md":           "# A\n",
				schema.ContractRelPath: string(contract),
			},
			want: ContractUnloaded,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			snap := ungovernedSnapshot(t, tt.files)
			if got := ContractStateFrom(false, snap); got != tt.want {
				t.Errorf("ContractStateFrom(false, snap) = %s, want %s", got, tt.want)
			}
			// A folder something claims authority over is that, whatever is on
			// its shelf: the sentence about a file nobody loaded belongs only
			// to the reader who has no authority behind the page.
			if got := ContractStateFrom(true, snap); got != ContractGoverning {
				t.Errorf("ContractStateFrom(true, snap) = %s, want %s", got, ContractGoverning)
			}
		})
	}
}

// TestAContractStateNamesItself pins the word each state answers with. The
// word reaches a diagnostic and a log line, and in both a number is a lookup
// the reader has to perform against a constant block they do not have open.
func TestAContractStateNamesItself(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		state ContractState
		want  string
	}{
		{ContractGoverning, "governing"},
		{ContractAbsent, "absent"},
		{ContractUnloaded, "unloaded"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()

			if got := tc.state.String(); got != tc.want {
				t.Errorf("ContractState.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAContractStateRefusesToNameAValueItDoesNotDeclare holds the answer for
// the member somebody adds to the constant block later: a name borrowed from
// one of the three would report a folder state the reader is not in, so the
// value itself is all there is to say.
func TestAContractStateRefusesToNameAValueItDoesNotDeclare(t *testing.T) {
	t.Parallel()

	defer func() {
		recovered := recover()
		text, isText := recovered.(string)
		if !isText || !strings.Contains(text, "99") {
			t.Errorf("panic = %v, want a message naming the value 99", recovered)
		}
	}()
	_ = ContractState(99).String()
	t.Error("ContractState(99).String() returned instead of panicking")
}

// ungovernedSnapshot reads one temporary folder the way the server does, with
// no contract handed to the scanner, and returns the published generation.
func ungovernedSnapshot(t *testing.T, files map[string]string) *snapshot.Generation {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("MkdirAll for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", rel, err)
		}
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("reader.Close: %v", closeErr)
		}
	})
	store, err := snapshot.New(t.Context(), reader, slog.New(slog.DiscardHandler), nil, schema.Ungoverned())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	return store.Current().Capture()
}

// TestWithheldNavigationLeavesTheEmptySlotSilent is the fourth state: a contract
// that loaded but left navigation out withholds the listing and states the
// reason once, without inventing another empty sentence.
func TestWithheldNavigationLeavesTheEmptySlotSilent(t *testing.T) {
	t.Parallel()

	closure := nav.Close(schema.Rejected("contract declares no navigation roles; Paths and Maps disabled until it does"))
	view := NewPathIndex(nil, declaringContract(t, "trail-guide", "atlas"), closure, ContractGoverning, wording.ZhHant, nil)
	if view.Shelf.Empty != "" {
		t.Errorf("withheld path index spoke an empty sentence: %q", view.Shelf.Empty)
	}
	if view.Kicker != "" {
		t.Errorf("withheld path index still carried a count: %q", view.Kicker)
	}
}
