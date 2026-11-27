// Package sealedline is the line format the ledger and the machine files of
// marks share: one JSON object to a line, whose last field is a checksum of
// every byte before that field, so that a line can carry a field its reader
// does not know and still be checked. The format is frozen: machine files are
// read by every version on every computer that shares a vault, so a change here
// is a change of both files.
package sealedline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
)

// checksumKey opens the last field of every line. JSON writes a quote inside a
// string as \", so these bytes stand only where a line has its checksum.
var checksumKey = []byte(`,"h":"`)

// checksumLength is how many hexadecimal digits of the SHA-256 a line keeps:
// enough that a changed line is not taken for a whole one by chance, and short
// enough to read.
const checksumLength = 16

// Verdict is what a look at one line found.
type Verdict int

const (
	// Whole is a line as it was sealed.
	Whole Verdict = iota
	// Cut is a line that does not end the way a sealed line does: one a crash
	// stopped part way, or one that was never sealed.
	Cut
	// Changed is a line that ends the way a sealed line does and does not fit
	// its checksum: something changed it after it was sealed.
	Changed
)

// Seal is v as one line, without the newline that ends it, its checksum the
// last field. v encodes to a JSON object with no field of its own named "h";
// HTML is not escaped, so a line holds the reader's words as they are.
func Seal(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	body, ok := bytes.CutSuffix(buf.Bytes(), []byte("}\n"))
	if !ok || len(body) < 2 {
		return nil, errors.New("seal a line: not a JSON object with a field")
	}
	return slices.Concat(body, checksumKey, []byte(checksum(body)), []byte(`"}`)), nil
}

// Check says what line, without its newline, is.
func Check(line []byte) Verdict {
	at := bytes.LastIndex(line, checksumKey)
	if at < 0 || !bytes.HasSuffix(line, []byte(`"}`)) || len(line) < at+len(checksumKey)+2 {
		return Cut
	}
	if string(line[at+len(checksumKey):len(line)-2]) != checksum(line[:at]) {
		return Changed
	}
	return Whole
}

// Lines splits data at its newlines, each without its newline and without a
// carriage return before it, leaving out the empty ones: a checkout on Windows
// may have put a carriage return there. The last is the part after the last
// newline, which is a line cut short or a whole one that lost only its newline.
func Lines(data []byte) [][]byte {
	var out [][]byte
	for line := range bytes.SplitSeq(data, []byte("\n")) {
		if line = bytes.TrimSuffix(line, []byte("\r")); len(line) > 0 {
			out = append(out, line)
		}
	}
	return out
}

func checksum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:checksumLength]
}
