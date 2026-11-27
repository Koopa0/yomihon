package ledger

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/mark/sealedline"
)

// wire is a line as it is written. The field names are the format, and every
// line has them in this order, sealed by its checksum (internal/mark/sealedline).
type wire struct {
	Seq     uint64 `json:"seq"`
	T       int64  `json:"t"`
	Kind    string `json:"kind"`
	ID      string `json:"id,omitempty"`
	Machine string `json:"writer,omitempty"`
	Process string `json:"s,omitempty"`
	Mark    string `json:"mark,omitempty"`
	Bytes   string `json:"bytes,omitempty"`
	B64     string `json:"b64,omitempty"`
}

// encode is e as one line, without the newline that ends it. Content goes in
// as text when it is UTF-8, which JSON keeps byte for byte, and no longer so
// than as base64; and as base64 otherwise, since JSON writes U+FFFD for each
// byte it cannot read, and escapes a quote or a backslash in two bytes and a
// control character in up to six. A content of n bytes so takes at most 4n/3
// of a line.
func encode(e *Event) []byte {
	w := wire{
		Seq: e.Seq, T: e.Time.UnixMilli(), Kind: e.Kind, ID: e.ID,
		Machine: e.Machine, Process: e.Process, Mark: e.Mark,
	}
	if utf8.Valid(e.Content) && quotedLength(e.Content) <= base64.StdEncoding.EncodedLen(len(e.Content)) {
		w.Bytes = string(e.Content)
	} else {
		w.B64 = base64.StdEncoding.EncodeToString(e.Content)
	}
	line, err := sealedline.Seal(w)
	if err != nil {
		panic("ledger: a line of strings and integers did not encode: " + err.Error())
	}
	return line
}

// quotedLength is how long text is inside the quotes of a JSON string, as a
// line writes it.
func quotedLength(text []byte) int {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(string(text)) //nolint:errcheck,errchkjson // a string always encodes
	return buf.Len() - len(`""`+"\n")
}

// verdict is what reading one line found.
type verdict int

const (
	// whole is a line as it was written.
	whole verdict = iota
	// cut is a line a crash stopped part way: it was never committed.
	cut
	// changed is a line that is all there and is not what was written.
	changed
)

// decode reads one line, without its newline. A line that does not end the way
// every line ends was cut short by a crash before it was committed. One that
// does and whose checksum does not fit what stands before it, or that does not
// say what a line says, was changed after it was written.
func decode(raw []byte) (Event, verdict) {
	switch sealedline.Check(raw) {
	case sealedline.Cut:
		return Event{}, cut
	case sealedline.Changed:
		return Event{}, changed
	case sealedline.Whole:
	}
	var w wire
	if json.Unmarshal(raw, &w) != nil || w.Seq == 0 || w.Kind == "" {
		return Event{}, changed
	}
	e := Event{
		Seq: w.Seq, Time: time.UnixMilli(w.T), Kind: w.Kind, ID: w.ID,
		Machine: w.Machine, Process: w.Process, Mark: w.Mark, raw: raw,
	}
	switch {
	case w.B64 != "" && w.Bytes != "":
		return Event{}, changed
	case w.B64 != "":
		content, err := base64.StdEncoding.DecodeString(w.B64)
		if err != nil {
			return Event{}, changed
		}
		e.Content = content
	case w.Bytes != "":
		e.Content = []byte(w.Bytes)
	}
	return e, whole
}

// seqOf is the seq a line that is not whole still says, if it says one: a
// line a few bytes of which were changed has almost always kept it, and so has
// a line cut short past its first field, which every line opens with. The next
// line must not take a seq a line in the ledger already has.
func seqOf(raw []byte) uint64 {
	var w struct {
		Seq uint64 `json:"seq"`
	}
	if json.Unmarshal(raw, &w) == nil {
		return w.Seq
	}
	digits, found := bytes.CutPrefix(raw, []byte(`{"seq":`))
	if !found {
		return 0
	}
	if end := bytes.IndexByte(digits, ','); end >= 0 {
		digits = digits[:end]
	} else {
		return 0 // cut inside the seq itself: not a number it said whole
	}
	seq, err := strconv.ParseUint(string(digits), 10, 64)
	if err != nil {
		return 0
	}
	return seq
}

// after is next, raised past seq: the seq the next line takes once a line says
// seq. A seq at the top of the range wraps to 0 and so raises nothing.
func after(next, seq uint64) uint64 {
	return max(next, seq+1)
}
