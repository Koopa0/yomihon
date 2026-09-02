package status

import (
	"testing"
	"time"
)

// receiptWriter is a Writer with only what the receipt ledger touches. The
// ledger is guarded by its own mutex and never reads the root, so a zero
// Writer exercises exactly the code a live one runs.
func receiptWriter() *Writer {
	return &Writer{}
}

func TestConsumeReceiptSpendsAMintedReceiptExactlyOnce(t *testing.T) {
	t.Parallel()
	w := receiptWriter()
	w.vouchReceipt("Writing/a.md", "draft")

	if !w.ConsumeReceipt("Writing/a.md", "draft") {
		t.Fatal(`ConsumeReceipt("Writing/a.md", "draft") = false right after the mint, want true`)
	}
	if w.ConsumeReceipt("Writing/a.md", "draft") {
		t.Error("a second consume of the same receipt = true, want false: the first must spend it")
	}
}

func TestConsumeReceiptAnswersFalseWithoutAMint(t *testing.T) {
	t.Parallel()
	w := receiptWriter()
	if w.ConsumeReceipt("Writing/a.md", "draft") {
		t.Error("ConsumeReceipt with no mint = true, want false")
	}
}

func TestConsumeReceiptMismatchedOriginSpendsNothing(t *testing.T) {
	t.Parallel()
	w := receiptWriter()
	w.vouchReceipt("Writing/a.md", "draft")

	if w.ConsumeReceipt("Writing/a.md", "archived") {
		t.Fatal("a mismatched origin = true, want false")
	}
	if !w.ConsumeReceipt("Writing/a.md", "draft") {
		t.Error("the matching consume after a mismatch = false, want true: the mismatch must not spend the receipt")
	}
}

func TestConsumeReceiptDistinguishesNotes(t *testing.T) {
	t.Parallel()
	w := receiptWriter()
	w.vouchReceipt("Writing/a.md", "draft")

	if w.ConsumeReceipt("Writing/b.md", "draft") {
		t.Fatal("another note's path = true, want false")
	}
	if !w.ConsumeReceipt("Writing/a.md", "draft") {
		t.Error("the minted note's consume after another path asked = false, want true")
	}
}

func TestConsumeReceiptNormalizesThePathLikeTheFlipThatMintedIt(t *testing.T) {
	t.Parallel()
	w := receiptWriter()
	// The mint key is the flip's normalized slash path; a reading page asking
	// with an unnormalized spelling of the same note must find it.
	w.vouchReceipt("Writing/a.md", "draft")
	if !w.ConsumeReceipt("./Writing/a.md", "draft") {
		t.Error(`ConsumeReceipt("./Writing/a.md", ...) = false for a receipt minted under "Writing/a.md", want true`)
	}

	w.vouchReceipt("Writing/a.md", "draft")
	if w.ConsumeReceipt("../Writing/a.md", "draft") {
		t.Error("a non-local path = true, want false")
	}
	if w.ConsumeReceipt("", "draft") {
		t.Error("an empty path = true, want false")
	}
	if w.ConsumeReceipt("Writing/a.md", "") {
		t.Error("an empty origin = true, want false")
	}
}

func TestConsumeReceiptExpiresAndDropsAnUncollectedReceipt(t *testing.T) {
	t.Parallel()
	w := receiptWriter()
	w.vouchReceipt("Writing/a.md", "draft")
	w.receiptMu.Lock()
	w.receipts["Writing/a.md"] = receipt{from: "draft", at: time.Now().Add(-receiptTTL - time.Second)}
	w.receiptMu.Unlock()

	if w.ConsumeReceipt("Writing/a.md", "draft") {
		t.Fatal("an expired receipt = true, want false")
	}
	w.receiptMu.Lock()
	_, still := w.receipts["Writing/a.md"]
	w.receiptMu.Unlock()
	if still {
		t.Error("the expired receipt is still in the ledger after the refusing consume")
	}
}

func TestVouchReceiptReplacesTheUnreadReceiptForTheSameNote(t *testing.T) {
	t.Parallel()
	w := receiptWriter()
	w.vouchReceipt("Writing/a.md", "draft")
	w.vouchReceipt("Writing/a.md", "ready")

	if w.ConsumeReceipt("Writing/a.md", "draft") {
		t.Fatal("the replaced origin = true, want false: only the latest flip's receipt stands")
	}
	if !w.ConsumeReceipt("Writing/a.md", "ready") {
		t.Error("the latest origin = false, want true")
	}
}

func TestVouchReceiptSweepsExpiredReceiptsOfOtherNotes(t *testing.T) {
	t.Parallel()
	w := receiptWriter()
	w.vouchReceipt("Writing/a.md", "draft")
	w.receiptMu.Lock()
	w.receipts["Writing/a.md"] = receipt{from: "draft", at: time.Now().Add(-receiptTTL - time.Second)}
	w.receiptMu.Unlock()

	w.vouchReceipt("Writing/b.md", "draft")

	w.receiptMu.Lock()
	_, expiredStays := w.receipts["Writing/a.md"]
	size := len(w.receipts)
	w.receiptMu.Unlock()
	if expiredStays || size != 1 {
		t.Errorf("after a later mint the ledger holds %d receipts (expired one kept: %t), want exactly the fresh one", size, expiredStays)
	}
}

func TestConsumeReceiptOnANilWriterAnswersFalse(t *testing.T) {
	t.Parallel()
	var w *Writer
	if w.ConsumeReceipt("Writing/a.md", "draft") {
		t.Error("a nil Writer = true, want false")
	}
}
