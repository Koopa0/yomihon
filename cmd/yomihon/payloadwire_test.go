package main_test

import (
	"bytes"
	"context"
	"io"

	"github.com/koopa0/yomihon/internal/judge"
)

// The same converters for the external test package, which cannot see the ones
// beside the command's own tests.

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

func checkBytes(ctx context.Context, o *judge.CheckOptions) (out []byte, exit int, err error) {
	payload, exit, err := judge.RunCheck(ctx, o)
	out, err = writtenBytes(payload, err)
	return out, exit, err
}

func coverageBytes(ctx context.Context, o *judge.CoverageOptions) (out []byte, exit int, err error) {
	payload, exit, err := judge.RunCoverage(ctx, o)
	out, err = writtenBytes(payload, err)
	return out, exit, err
}

func existsBytes(ctx context.Context, o *judge.ExistsOptions) (out []byte, exit int, err error) {
	payload, exit, err := judge.RunExists(ctx, o)
	out, err = writtenBytes(payload, err)
	return out, exit, err
}
