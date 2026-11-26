package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"
)

// checksumKey opens the last field of every line. JSON writes a quote inside a
// string as \", so these bytes stand only where a line has its checksum.
var checksumKey = []byte(`,"h":"`)

// checksumLength is how many hexadecimal digits of the SHA-256 a line keeps:
// enough that a changed line is not taken for a whole one by chance, and short
// enough to read.
const checksumLength = 16

// wire is a line as it is written. The field names are the format, and every
// line has them in this order, the checksum last.
type wire struct {
	Seq     uint64 `json:"seq"`
	T       int64  `json:"t"`
	Kind    string `json:"kind"`
	ID      string `json:"id,omitempty"`
	Path    string `json:"path,omitempty"`
	Sum     string `json:"sum,omitempty"`
	Writer  string `json:"writer,omitempty"`
	Session string `json:"s,omitempty"`
	Bytes   string `json:"bytes,omitempty"`
	B64     string `json:"b64,omitempty"`
	H       string `json:"h,omitempty"`
}

// checksum is the first checksumLength digits of the SHA-256 of b.
func checksum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:checksumLength]
}

// encode is e as one line, without the newline that ends it. Content goes in as
// text when it is UTF-8, which JSON keeps byte for byte, and as base64 when it
// is not, which JSON would not: it writes U+FFFD for each byte it cannot read.
func encode(e *Event) []byte {
	w := wire{
		Seq: e.Seq, T: e.Time.UnixMilli(), Kind: e.Kind, ID: e.ID, Path: e.Path,
		Sum: e.Sum, Writer: e.Writer, Session: e.Session,
	}
	if utf8.Valid(e.Content) {
		w.Bytes = string(e.Content)
	} else {
		w.B64 = base64.StdEncoding.EncodeToString(e.Content)
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(w) //nolint:errcheck,errchkjson // a struct of strings and integers always encodes
	body := bytes.TrimSuffix(buf.Bytes(), []byte("}\n"))
	return slices.Concat(body, checksumKey, []byte(checksum(body)), []byte(`"}`))
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
	at := bytes.LastIndex(raw, checksumKey)
	if at < 0 || !bytes.HasSuffix(raw, []byte(`"}`)) || len(raw) < at+len(checksumKey)+2 {
		return Event{}, cut
	}
	if string(raw[at+len(checksumKey):len(raw)-2]) != checksum(raw[:at]) {
		return Event{}, changed
	}
	var w wire
	if json.Unmarshal(raw, &w) != nil || w.Seq == 0 || w.Kind == "" {
		return Event{}, changed
	}
	e := Event{
		Seq: w.Seq, Time: time.UnixMilli(w.T), Kind: w.Kind, ID: w.ID, Path: w.Path,
		Sum: w.Sum, Writer: w.Writer, Session: w.Session, raw: raw,
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
	if e.Kind == KindMark {
		e.mark = markOf(e.Content)
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

// markOf is the id of the mark a line of a machine file is about, or empty when
// content is not one. It is the only thing the ledger reads in a mark's line.
func markOf(content []byte) string {
	var line struct {
		Mark struct {
			ID string `json:"id"`
		} `json:"mark"`
	}
	if json.Unmarshal(content, &line) != nil {
		return ""
	}
	return line.Mark.ID
}

// lines splits data at its newlines, each without its newline and without a
// carriage return before it, leaving out the empty ones. The last is the part
// after the last newline, which is a line cut short or a whole one that lost
// only its newline.
func lines(data []byte) [][]byte {
	var out [][]byte
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if line = bytes.TrimSuffix(line, []byte("\r")); len(line) > 0 {
			out = append(out, line)
		}
	}
	return out
}
