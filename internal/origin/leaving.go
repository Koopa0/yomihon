package origin

import (
	"errors"
	"log/slog"
	"net/http"
	"syscall"
)

// WriteFailureLevel is how loudly a response that could not be written should
// be reported: debug when the reader left, error otherwise. It logs nothing
// itself. Either the request's context or the write's own broken-pipe or reset
// error can answer first, depending on how far the response had got.
func WriteFailureLevel(r *http.Request, err error) slog.Level {
	if r.Context().Err() != nil ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) {
		return slog.LevelDebug
	}
	return slog.LevelError
}
