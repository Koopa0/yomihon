package origin

import (
	"errors"
	"log/slog"
	"net/http"
	"syscall"
)

// WriteFailureLevel is how loudly a response that could not be written should
// be reported. It decides the level and logs nothing itself, because the
// operational event belongs to the handler that owns the response.
//
// A reader who left is not a fault yomihon made. A browser asks for an icon at
// a fixed address on nearly every fresh page, gets a whole page instead, and
// drops the connection the moment it sees what it is — which used to be the
// most frequent loud line in the log, on a site where the loud lines are the
// operator's only instrument. The same happens to anyone who presses Back
// while a long page is still arriving.
//
// Two questions, because either can answer first: the request's own context is
// cancelled when the connection closes, and the write itself fails with the
// operating system's broken-pipe or reset answer. Which arrives first depends
// on where the response had got to.
func WriteFailureLevel(r *http.Request, err error) slog.Level {
	if r.Context().Err() != nil ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) {
		return slog.LevelDebug
	}
	return slog.LevelError
}
