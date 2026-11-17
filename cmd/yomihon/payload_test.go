package main

import (
	"bytes"
	"context"
	"io"

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
