package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/koopa0/yomihon/internal/judge"
)

// The engines return a sealed payload that can only be written. A test reads
// what would be written by writing it into a buffer, once, here.

func writtenBytes(payload io.WriterTo, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if _, err := payload.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func existsBytes(ctx context.Context, o *judge.ExistsOptions) (out []byte, exit int, err error) {
	payload, exit, err := judge.RunExists(ctx, o)
	out, err = writtenBytes(payload, err)
	return out, exit, err
}

// checkedFinding is the part of one check finding these tests read.
type checkedFinding struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// checkFindings runs check over root and decodes its JSONL payload.
func checkFindings(t *testing.T, root string) []checkedFinding {
	t.Helper()
	payload, _, err := judge.RunCheck(t.Context(), &judge.CheckOptions{Root: root, Format: judge.FormatJSON})
	out, err := writtenBytes(payload, err)
	if err != nil {
		t.Fatalf("check %s: %v", root, err)
	}
	var findings []checkedFinding
	for line := range bytes.Lines(out) {
		var f checkedFinding
		if err := json.Unmarshal(line, &f); err != nil {
			t.Fatalf("check %s wrote a line that is not a finding: %q: %v", root, line, err)
		}
		findings = append(findings, f)
	}
	return findings
}
