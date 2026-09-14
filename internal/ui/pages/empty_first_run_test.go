package pages

import (
	"bytes"
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

// TestEmptyPathAndMapGuideFirstRun holds the three empty signals and the one
// completable step each carries on the path and map index pages and on the desk
// blocks that narrow them.
func TestEmptyPathAndMapGuideFirstRun(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		contract ContractState
		path     wording.Phrase
		mapState wording.Phrase
		folder   wording.Phrase
		step     wording.Phrase
	}{
		{
			name:     "folder with no contract",
			contract: ContractAbsent,
			path:     wording.IndexUngoverned,
			mapState: wording.IndexUngoverned,
			folder:   wording.IndexUngoverned,
			step:     wording.IndexUngovernedNext,
		},
		{
			name:     "contract that reached the folder after yomihon started",
			contract: ContractUnloaded,
			path:     wording.IndexContractUnloaded,
			mapState: wording.IndexContractUnloaded,
			folder:   wording.IndexContractUnloaded,
			step:     wording.IndexContractUnloadedNext,
		},
		{
			name:     "contract that declares none",
			contract: ContractGoverning,
			path:     wording.PathIndexEmpty,
			mapState: wording.MapIndexEmpty,
			folder:   wording.FolderIndexEmpty,
			step:     wording.IndexDeclaredEmptyNext,
		},
	}
	for _, tt := range cases {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(tt.name+"/"+string(lang), func(t *testing.T) {
				t.Parallel()
				pathView := NewPathIndex(nil, nav.Closure{}, tt.contract, lang, nil)
				mapView := NewMapIndex(nil, nav.Closure{}, tt.contract, lang, nil)
				folderView := NewFolderIndex(&nav.Model{}, tt.contract, lang, nil)
				for _, view := range []struct {
					name  string
					state wording.Phrase
					got   string
				}{
					{name: "paths", state: tt.path, got: pathView.Shelf.Empty},
					{name: "maps", state: tt.mapState, got: mapView.Shelf.Empty},
					{name: "folders", state: tt.folder, got: folderView.Shelf.Empty},
				} {
					assertEmptyGuide(t, view.name, view.got, view.state, tt.step, lang)
				}
				blocks := NewDeskBlocks(&nav.Model{}, tt.contract, lang, nil)
				for _, block := range blocks {
					if block.Mode != pathMode && block.Mode != mapMode && block.Mode != folderMode {
						continue
					}
					state := tt.path
					switch block.Mode {
					case mapMode:
						state = tt.mapState
					case folderMode:
						state = tt.folder
					}
					assertEmptyGuide(t, "desk/"+block.Mode, block.Shelf.Empty, state, tt.step, lang)
				}
			})
		}
	}
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

func assertEmptyGuide(t *testing.T, where, got string, state, step wording.Phrase, lang wording.Lang) {
	t.Helper()
	stateText := state.In(lang)
	stepText := step.In(lang)
	var want string
	if lang == wording.En {
		want = stateText + " " + stepText
	} else {
		want = stateText + stepText
	}
	if !strings.Contains(got, want) {
		t.Errorf("%s empty guide want %q; got %q", where, want, got)
	}
	for _, other := range everyEmptyStep {
		if other == step {
			continue
		}
		if strings.Contains(got, other.In(lang)) {
			t.Errorf("%s empty guide also offers %q, which is another folder's way out: %q", where, other.In(lang), got)
		}
	}
	if strings.Contains(got, "宣告") {
		t.Errorf("%s still uses 宣告 jargon: %q", where, got)
	}
}

// TestEmptyPathAndMapIndexPagesRenderTheGuide keeps the empty slot on the mode
// index pages themselves, not only in the view the desk narrows from. Both
// states a reader without a listing can be in are drawn, because the sentence
// that only the view carries is a sentence nobody reads.
func TestEmptyPathAndMapIndexPagesRenderTheGuide(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		contract ContractState
		state    wording.Phrase
		step     wording.Phrase
	}{
		{
			name:     "no contract in the folder",
			contract: ContractAbsent,
			state:    wording.IndexUngoverned,
			step:     wording.IndexUngovernedNext,
		},
		{
			name:     "contract this process never loaded",
			contract: ContractUnloaded,
			state:    wording.IndexContractUnloaded,
			step:     wording.IndexContractUnloadedNext,
		},
	} {
		for _, mode := range []string{pathMode, mapMode, folderMode} {
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				t.Run(tt.name+"/"+mode+"/"+string(lang), func(t *testing.T) {
					t.Parallel()
					var buf bytes.Buffer
					view := emptyIndexView(mode, tt.contract, lang)
					if err := ListIndex(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
						t.Fatalf("render: %v", err)
					}
					html := buf.String()
					if !strings.Contains(html, `data-index-empty`) {
						t.Fatalf("%s index did not render the empty slot: %q", mode, html)
					}
					assertEmptyGuide(t, mode, html, tt.state, tt.step, lang)
				})
			}
		}
	}
}

// emptyIndexView builds one mode's index over a vault holding nothing of that
// kind, which is the only state in which the slot under test is drawn at all.
func emptyIndexView(mode string, contract ContractState, lang wording.Lang) ListIndexView {
	switch mode {
	case mapMode:
		return NewMapIndex(nil, nav.Closure{}, contract, lang, nil)
	case folderMode:
		return NewFolderIndex(&nav.Model{}, contract, lang, nil)
	default:
		return NewPathIndex(nil, nav.Closure{}, contract, lang, nil)
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
	view := NewPathIndex(nil, closure, ContractGoverning, wording.ZhHant, nil)
	if view.Shelf.Empty != "" {
		t.Errorf("withheld path index spoke an empty sentence: %q", view.Shelf.Empty)
	}
	if view.Kicker != "" {
		t.Errorf("withheld path index still carried a count: %q", view.Kicker)
	}
}
