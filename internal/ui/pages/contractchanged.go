package pages

import (
	"context"
	"net/http"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// ContractChangedSentence is what a page says, in the reader's language, when
// the contract's bytes moved after yomihon read them: the cause and the one step
// that follows. Every surface that stands in for a withheld projection under
// that cause says these words, so a reader who meets them on the desk and again
// on a course page is told one thing.
func ContractChangedSentence(lang wording.Lang) string {
	return wording.JoinGuide(wording.ContractChanged, wording.RestartYomihon, lang)
}

// FaultOf divides why a projection was withheld into the two things a page can
// say about it: the operator's sentence, quoted as the contract's own, and the
// reader's sentence for a cause the dictionary has words of its own for. A
// cause is one or the other, never both, so a page does not say it twice. It
// takes the two values rather than a nav.Closure, so a schema.Claim can feed it
// too, and every surface that states why something is missing decides here.
func FaultOf(diagnostic string, reason schema.Reason, lang wording.Lang) (quoted, said string) {
	if reason == schema.ReasonContractChanged {
		return "", ContractChangedSentence(lang)
	}
	return diagnostic, ""
}

// WriteContractChanged answers a request for something yomihon can no longer
// serve because the contract it read at startup is not the one on disk: the
// content type, the 503 status, and the page, in that order.
//
// It is not a not-found page and must not be answered as one. The course a
// reader asked for was there a moment ago, so the address is not what is wrong,
// and a 404 saying so sends them looking for a typo. The status is the one the
// write face already answers with when the same cause stops it: the instance is
// refusing until it is started again.
func WriteContractChanged(
	ctx context.Context,
	w http.ResponseWriter,
	//nolint:gocritic // hugeParam: the component this hands it to takes it by
	// value, and a pointer here would let a caller keep changing what the
	// renderer is already writing out.
	chrome layouts.Chrome,
) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	return ContractChanged(chrome).Render(ctx, w)
}
