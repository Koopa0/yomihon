package judge

import (
	"bytes"
	"context"
	"io"
)

// The three engines return a sealed payload that can only be written, which is
// the point of the type. A test reads what would be written by writing it into
// a buffer; these are that, once, so no assertion has to know the difference.

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

func checkBytes(ctx context.Context, o *CheckOptions) (out []byte, exit int, err error) {
	payload, exit, err := RunCheck(ctx, o)
	out, err = writtenBytes(payload, err)
	return out, exit, err
}

func coverageBytes(ctx context.Context, o *CoverageOptions) (out []byte, exit int, err error) {
	payload, exit, err := RunCoverage(ctx, o)
	out, err = writtenBytes(payload, err)
	return out, exit, err
}

func existsBytes(ctx context.Context, o *ExistsOptions) (out []byte, exit int, err error) {
	payload, exit, err := RunExists(ctx, o)
	out, err = writtenBytes(payload, err)
	return out, exit, err
}
