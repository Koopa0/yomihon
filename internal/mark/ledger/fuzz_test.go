package ledger

import (
	"bytes"
	"testing"
	"time"
)

// FuzzDecode reads any bytes as a line, and as a whole ledger, without
// panicking. A line it reads as whole fits its checksum and says a seq and a
// kind; and the seq the next line takes is above every seq a whole or changed
// line says, so no seq is given twice.
func FuzzDecode(f *testing.F) {
	f.Add(encode(&Event{Seq: 1, Kind: "header", ID: "1f3a9c07"}))
	f.Add(encode(&Event{Seq: 7, Kind: KindMark, Writer: "a", Session: "s", Content: markLine("m1", "字")}))
	f.Add(encode(&Event{Seq: 9, Kind: "marks-replaced", Content: []byte{0xff, 0}}))
	f.Add([]byte(`{"seq":3,"kind":"x","h":"0000000000000000"}`))
	f.Add([]byte("{\"seq\":1\n\r\n{}\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if e, v := decode(raw); v == whole {
			at := bytes.LastIndex(raw, checksumKey)
			if e.Seq == 0 || e.Kind == "" || string(raw[at+len(checksumKey):len(raw)-2]) != checksum(raw[:at]) {
				t.Fatalf("decode(%q) is whole with seq %d, kind %q", raw, e.Seq, e.Kind)
			}
		}
		var st state
		st.next = 1
		st.parse(raw)
		for i := range st.events {
			if seq := st.events[i].Seq; seq >= st.next {
				t.Fatalf("a whole line says seq %d and the next line would take %d", seq, st.next)
			}
		}
		for _, c := range st.changed {
			if seq := seqOf(c); seq >= st.next {
				t.Fatalf("a changed line says seq %d and the next line would take %d", seq, st.next)
			}
		}
	})
}

// FuzzLineRoundTrip writes any event as a line and reads it back: it comes back
// as it was given, every part of it short of the whole reads as cut or
// changed and never as whole, and so does the line with any one byte changed.
func FuzzLineRoundTrip(f *testing.F) {
	f.Add(uint64(1), "mark", "id", "writer", "s", []byte(`{"mark":{"id":"m1"}}`), 10, 3)
	f.Add(uint64(1<<63), "k", "", "", "", []byte{0xc3, 0x28, '\n', '"', '\\'}, 0, 0)
	f.Add(uint64(42), "marks-replaced", "<&>", "\u2028", ",\"h\":\"", []byte(`,"h":"0123456789abcdef"}`), 1000, 1000)
	f.Fuzz(func(t *testing.T, seq uint64, kind, id, writer, session string, content []byte, cut, flip int) {
		given := Event{Seq: seq, Time: time.UnixMilli(1795164060000), Kind: kind, ID: id, Writer: writer, Session: session, Content: content}
		if seq == 0 || given.check() != nil {
			return // Record gives every line a seq, and refuses what check refuses
		}
		line := encode(&given)
		if bytes.IndexByte(line, '\n') >= 0 {
			t.Fatalf("encode(%+v) holds a newline: %q", given, line)
		}
		got, v := decode(line)
		if v != whole || !sameEvent(&got, &given) {
			t.Fatalf("decode(encode(%+v)) = %+v, %v; want it back whole", given, got, v)
		}
		if n := abs(cut) % len(line); n > 0 {
			if _, v = decode(line[:n]); v == whole {
				t.Fatalf("the first %d bytes of %q read as a whole line", n, line)
			}
		}
		changed := bytes.Clone(line)
		at := abs(flip) % len(changed)
		changed[at] ^= 1 << (abs(flip) % 8)
		if e, v := decode(changed); v == whole {
			t.Fatalf("%q with byte %d changed reads as a whole line: %+v", line, at, e)
		}
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
