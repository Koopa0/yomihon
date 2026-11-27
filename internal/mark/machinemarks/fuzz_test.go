package machinemarks

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// FuzzDecode reads any bytes as a line. It never panics, and a line it reads
// holds the shapes every writer makes. That a strict prefix or a changed byte
// is not read is sealedline's to hold.
func FuzzDecode(f *testing.F) {
	good, err := (&Line{Fmt: Format, M: "1f3a9c07", Rev: 3, T: 5, S: "7a1f03d2", Mark: thought(1, "words \"quoted\" and 字"), Seen: Seen{"c2e4b815/5d01aa3e": 9}, Edit: &Edit{ID: "e", Seq: 2}}).encode()
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range [][]byte{good, nil, []byte("{}"), []byte("<<<<<<< HEAD"), []byte(`{"fmt":2}`), good[:len(good)/2], append(bytes.Clone(good), '\r')} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		l, v := decode(raw)
		if v != readable {
			return
		}
		if l.Fmt != Format || !isMachine(l.M) || !isID(l.Mark.ID) || l.Rev == 0 {
			t.Fatalf("a line read has %+v", l)
		}
	})
}

// linesFrom makes lines of three machines and two processes from bytes, six
// bytes for each.
func linesFrom(data []byte) []*kept {
	machines := []string{"11111111", "22222222", "33333333"}
	processes := []string{"", "aaaaaaaa"}
	var out []*kept
	// Two different lines of one writer, one rev and one mark are what only a
	// hand makes, and which of them is read is the order the lines come in: the
	// one case in which the order matters. The lines made here have one each.
	type key struct {
		w, id string
		rev   uint64
	}
	made := map[key]bool{}
	for ; len(data) >= 6; data = data[6:] {
		l := Line{
			Fmt: Format, M: machines[int(data[0])%3], S: processes[int(data[0]>>2)%2],
			Rev: uint64(data[1]%8) + 1, T: int64(data[2] % 5),
			Mark: thought(int(data[3]%3)+1, fmt.Sprintf("v%d", data[3])),
		}
		if k := (key{l.Writer(), l.Mark.ID, l.Rev}); made[k] {
			continue
		} else {
			made[k] = true
		}
		l.Mark.Deleted = data[4]&1 == 1
		l.Mark.At = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		if n := data[4] >> 1 % 3; n > 0 {
			l.Seen = Seen{}
			for i := range n {
				l.Seen[machines[int(data[5]>>(2*i))%3]+"/"+processes[int(data[5]>>(2*i+1))%2]] = uint64(data[5]>>i)%8 + 1
			}
		}
		raw, err := l.encode()
		if err != nil {
			panic(err)
		}
		out = append(out, &kept{Line: l, raw: raw})
	}
	return out
}

// shown is what a resolution says, in a form two resolutions are compared by.
func shown(ls []*kept) string {
	var parts []string
	for id, r := range merge(ls) {
		var heads []string
		for _, h := range r.heads {
			heads = append(heads, fmt.Sprintf("%s#%d", h.Writer(), h.Rev))
		}
		parts = append(parts, id+":"+strings.Join(heads, ","))
	}
	slices.Sort(parts)
	return strings.Join(parts, ";")
}

// FuzzMerge resolves random lines. The result does not depend on the order the
// lines are in or on a line given twice; a mark that has lines has a value;
// that value is a deletion only if every value is; the values of a conflict do
// not replace one another; and a line that has read every writer and is the
// latest is the mark's only value.
func FuzzMerge(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 4, 2, 1, 3, 3, 9})
	f.Add([]byte{1, 1, 1, 1, 0, 0, 2, 2, 2, 1, 1, 1, 3, 3, 3, 1, 5, 7})
	f.Fuzz(func(t *testing.T, data []byte) {
		ls := linesFrom(data)
		base := shown(ls)
		reversed := slices.Clone(ls)
		slices.Reverse(reversed)
		doubled := slices.Concat(ls, ls)
		if got := shown(reversed); got != base {
			t.Fatalf("the order of the lines changed the result:\n%s\n%s", base, got)
		}
		if got := shown(doubled); got != base {
			t.Fatalf("a line given twice changed the result:\n%s\n%s", base, got)
		}
		for id, r := range merge(ls) {
			if len(r.heads) == 0 {
				t.Fatalf("mark %s has lines and no value", id)
			}
			for _, h := range r.heads {
				if r.newest[h.Writer()] != h {
					t.Fatalf("mark %s shows a line of %s that its writer's newer line replaced", id, h.Writer())
				}
			}
			anyAlive := slices.ContainsFunc(r.heads, func(h *kept) bool { return !h.Mark.Deleted })
			if anyAlive && r.heads[0].Mark.Deleted {
				t.Fatalf("mark %s shows a deletion with a live value beside it", id)
			}
			if len(r.heads) > 1 {
				replaced := false
				for _, a := range r.heads {
					for _, b := range r.heads {
						replaced = replaced || (a != b && replaces(&a.Line, &b.Line))
					}
				}
				// Lines that say they read each other are all kept, and are the
				// one case in which values replace one another.
				if replaced && !everyoneIsReplaced(ls, id) {
					t.Fatalf("mark %s shows a value beside one that replaces it", id)
				}
			}
			// A line that has read every writer, and is of a new writer with a
			// higher rev, is the one value.
			top := &kept{Fmt: Format, M: "99999999", Rev: 1, T: 100, Mark: r.heads[0].Mark, Seen: Seen{}}
			for w, n := range r.newest {
				top.Seen[w] = n.Rev
			}
			top.Mark.Deleted = false
			var group []*kept
			for _, l := range ls {
				if l.Mark.ID == id {
					group = append(group, l)
				}
			}
			if got := resolve(append(group, top)); len(got.heads) != 1 || got.heads[0] != top {
				t.Fatalf("a line that read every writer of mark %s is not its one value", id)
			}
		}
	})
}

// everyoneIsReplaced says every writer's newest line of mark id is replaced by
// another's: the case in which no line is left, and every line stands.
func everyoneIsReplaced(ls []*kept, id string) bool {
	var group []*kept
	for _, l := range ls {
		if l.Mark.ID == id {
			group = append(group, l)
		}
	}
	newest := map[string]*kept{}
	for _, l := range group {
		if old := newest[l.Writer()]; old == nil || l.Rev > old.Rev {
			newest[l.Writer()] = l
		}
	}
	for _, l := range newest {
		covered := false
		for _, o := range group {
			covered = covered || (o.Writer() != l.Writer() && o.Seen[l.Writer()] >= l.Rev)
		}
		if !covered {
			return false
		}
	}
	return true
}
