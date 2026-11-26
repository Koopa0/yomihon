// Package marktest lets a test leave the marks a reader would have left in a
// vault, through the same hand the reading server writes with, so that what the
// test then reads is what the product reads. Nothing outside tests imports it.
package marktest

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/vault"
)

// Keep leaves each mark in the vault at root, and its ledger under configDir, as
// the reading server would: every one is a new mark of this machine. A mark with
// no id is given one; one with no time is made now.
func Keep(tb testing.TB, root string, configDir confdir.ConfigDir, marks ...machinemarks.Mark) {
	tb.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		tb.Fatalf("open the vault to leave marks in: %v", err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			tb.Errorf("close the vault: %v", closeErr)
		}
	}()
	store, err := mark.Open(configDir, reader.Name())
	if err != nil {
		tb.Fatalf("name the marks: %v", err)
	}
	kept := mark.OpenKept(store, reader, slog.New(slog.DiscardHandler), mark.AllMarks)
	defer func() {
		if closeErr := kept.Close(); closeErr != nil {
			tb.Errorf("close the marks: %v", closeErr)
		}
	}()
	hand, err := kept.Marks()
	if err != nil {
		tb.Fatalf("open the marks: %v", err)
	}
	for i := range marks {
		m := marks[i]
		if m.ID == "" {
			m.ID = id()
		}
		if m.At.IsZero() {
			m.At = time.Now()
		}
		if _, err = hand.Put(tb.Context(), &m, nil, nil, mark.AllMarks); err != nil {
			tb.Fatalf("leave a mark: %v", err)
		}
	}
}

// Question is a question of a whole section.
func Question(path, anchor string, at time.Time) machinemarks.Mark {
	return machinemarks.Mark{Kind: machinemarks.Question, Path: path, Anchor: anchor, At: at}
}

// id makes the id of a mark: 32 lowercase hexadecimal digits nobody else has.
func id() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("marktest: no randomness: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
