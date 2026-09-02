package status_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// Two spellings of one Japanese status word. NFC composes the dakuten into が;
// NFD leaves か followed by the combining mark. A filesystem hands over the
// decomposed form and an editor composes it, so both reach this code from the
// same vault, and nothing about the note the reader sees distinguishes them.
const (
	statusNFC = "進行中が"  // 進行中が
	statusNFD = "進行中が" // 進行中か + ◌゙
)

func loadContractWithComposedStatus(t *testing.T) *schema.Contract {
	t.Helper()
	return loadContractDeclaring(t, statusNFC)
}

// loadContractDeclaring builds a contract whose only substantive status is the
// spelling given, so the fold can be tested from both sides: a contract that
// composes meeting a note that decomposes, and the reverse.
func loadContractDeclaring(t *testing.T, declared string) *schema.Contract {
	t.Helper()
	statusNFC := declared
	contractText := `schema_version = "1"

[enums]
type = ["doc"]

[enums.status]
note = []
doc = ["` + statusNFC + `", "archived"]

[fields.status_group]
doc = ["doc"]

[artifacts]
non_instance_dirs = []

[[lifecycle]]
status = "` + statusNFC + `"
applies_to = ["doc"]
from = ["archived"]
owner = ["koopa"]

[[lifecycle]]
status = "archived"
applies_to = ["doc"]
from = ["` + statusNFC + `"]
owner = ["koopa"]
`
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(path, []byte(contractText), 0o600); err != nil { // #nosec G703 -- a fixed basename under t.TempDir
		t.Fatalf("write test contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	return contract
}

// TestStatusVerdictsDoNotDependOnHowTheWordIsSpelled holds that one note gets
// one answer. The two spellings below are the same word and the reader cannot
// see the difference, but they are different bytes — and the surfaces reach
// this contract from different sources: the reading page compares the note's
// own line as it sits on disk, while the whole-folder page compares the value
// the search index stored, which is normalized on the way in. Byte comparison
// therefore had the two pages disagree about the same note, one calling its
// status legal and the other calling it outside the schema.
func TestStatusVerdictsDoNotDependOnHowTheWordIsSpelled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContractWithComposedStatus(t))
	view := writer.Authority()

	if !view.KnownStatus("doc", statusNFC) {
		t.Fatalf("the contract's own spelling is not recognised; the fixture is wrong")
	}
	if !view.KnownStatus("doc", statusNFD) {
		t.Errorf("KnownStatus rejected the decomposed spelling of a declared status; the page reading the file and the page reading the index will disagree about the same note")
	}

	const rel = "Notes/n.md"
	composed := view.Transitions(rel, "doc", statusNFC)
	decomposed := view.Transitions(rel, "doc", statusNFD)
	if len(composed) == 0 {
		t.Fatalf("the fixture offers no transition from the declared status")
	}
	if len(decomposed) != len(composed) {
		t.Errorf("Transitions from the decomposed spelling = %v, from the composed one = %v; the same note offers different controls depending on how its word is stored", decomposed, composed)
	}

	if view.CanReturn("doc", statusNFC, "archived") != view.CanReturn("doc", statusNFD, "archived") {
		t.Errorf("CanReturn disagrees across the two spellings of the origin status")
	}
	if !view.CanReturn("doc", statusNFD, statusNFC) {
		t.Errorf("CanReturn does not read the two spellings as the same stop: a move that stays put reads as a one-way door")
	}

	// The control: a word that is genuinely not declared stays undeclared, or
	// the fold above would be excusing every value.
	if view.KnownStatus("doc", "進行") {
		t.Errorf("a status the contract never declares was accepted")
	}
}

// TestADecomposedContractAnswersAComposedNote is the mirror of the test above
// and the one that holds the fold applied when the contract is read. A vault
// whose toml spells its status decomposed is as ordinary as a note that does:
// the same editor and the same filesystem produced both files. Without the
// fold on the declared side the two simply swap places — the contract becomes
// the odd spelling and every note in the vault is judged outside its own
// schema.
func TestADecomposedContractAnswersAComposedNote(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContractDeclaring(t, statusNFD))
	view := writer.Authority()

	if !view.KnownStatus("doc", statusNFD) {
		t.Fatalf("the contract's own spelling is not recognised; the fixture is wrong")
	}
	if !view.KnownStatus("doc", statusNFC) {
		t.Errorf("a contract that spells its status decomposed rejects the composed spelling of the same word")
	}
	// A transition offered from the composed spelling is the same set as from
	// the declared one, so the write face does not close on an ordinary note.
	const rel = "Notes/n.md"
	if got := view.Transitions(rel, "doc", statusNFC); len(got) == 0 {
		t.Errorf("a note whose status composes gets no transition from a contract that decomposes")
	}
	if view.KnownStatus("doc", "進行") {
		t.Errorf("a status the contract never declares was accepted")
	}
}
