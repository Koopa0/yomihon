package note_test

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/snapshot"
)

func TestOpenThoughtsNamesShownPlacesInTheirOwnLanguage(t *testing.T) {
	t.Parallel()
	files := comparePairVault()
	files["System/slots/cutoverzh.yaml"] += "  - id: p2\n    template: \"\"\n    gloss_zh: \"\"\n    slots: {}\n"
	root := writeNotes(t, files)
	contract := loadHomeContract(t)
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
	writer := openStatusWriter(t, source, contract, contract.Governance())
	handler := note.New(&note.Sources{
		Source: source, Contract: contract, Status: writer.Authority,
		Snapshot:       func() *snapshot.Generation { return store.Current() },
		ObservedStatus: writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt,
		Continuation: noMark, Log: log,
		Uncertainties: func() ([]mark.Uncertainty, error) {
			return []mark.Uncertainty{
				{RelPath: "Writing/Cutoverzh.md", Anchor: "ledger", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
				{RelPath: "Writing/Cutoverzh.md", Anchor: "slot-pattern-1", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
				{RelPath: "Writing/Cutoverzh.md", Anchor: "slot-pattern-2", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)},
			}, nil
		},
	})
	mux := http.NewServeMux()
	handler.Register(mux)
	for _, lang := range []string{"en", "zh-Hant"} {
		fallback := `lang="en">practice card 2</span>`
		if lang == "zh-Hant" {
			fallback = `lang="zh-Hant">第 2 張練習卡</span>`
		}
		for _, path := range []string{"/", "/open-thoughts"} {
			page := lostMarkRequest(t, mux, http.MethodGet, path, "", lang)
			for _, want := range []string{
				`lang="zh-Hant">Ledger</span>`,
				`lang="ja">A</span>`,
				`href="/notes/Writing/Cutoverzh.md#ledger"`,
				`href="/notes/Writing/Cutoverzh.md#slot-pattern-1"`,
				`href="/notes/Writing/Cutoverzh.md#slot-pattern-2"`,
				fallback,
			} {
				if !strings.Contains(page, want) {
					t.Errorf("caught: %s in %s does not name its marked place with %s", path, lang, want)
				}
			}
			if strings.Contains(page, "Cutoverzh #") {
				t.Errorf("caught: %s in %s names a place by raw anchor", path, lang)
			}
		}
	}
}
