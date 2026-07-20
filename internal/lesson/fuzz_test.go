package lesson

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func FuzzParseSidecar(f *testing.F) {
	for _, seed := range [][]byte{
		nil,
		[]byte("lesson: L01\nslug: jp-minna-l01\ntitle: はじめまして\npatterns: []\n"),
		[]byte("{"),
		[]byte("lesson: L01\nlesson: L02\n"),
		[]byte("title: |\n  line one\n  line two\n"),
		[]byte("base: &base {label_zh: subject, color: topic}\ncopy: *base\n"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		const maxInput = 32 << 10
		if len(data) > maxInput {
			return
		}

		first, firstErr := parseSidecar("fixture.yaml", data)
		second, secondErr := parseSidecar("fixture.yaml", data)
		if diff := cmp.Diff(first, second); diff != "" {
			t.Fatalf("parseSidecar() is not deterministic (-first +second):\n%s", diff)
		}
		if errorText(firstErr) != errorText(secondErr) {
			t.Fatalf("parseSidecar() error is not deterministic: first %q, second %q", firstErr, secondErr)
		}
		if firstErr != nil {
			if !strings.HasPrefix(firstErr.Error(), "parse slot sidecar ") {
				t.Fatalf("parseSidecar() error = %q, want parse error", firstErr)
			}
			return
		}

		if diff := cmp.Diff(first.Validate(), second.Validate()); diff != "" {
			t.Fatalf("Validate() is not deterministic (-first +second):\n%s", diff)
		}
	})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
