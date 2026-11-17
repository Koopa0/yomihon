package egress

import (
	"io"
	"slices"
)

// Payload is an agent-facing output that was sealed by a Gate. It is opaque:
// there is one way to make one, Gate.Seal, which looks at the rules file again
// once the output exists, and one way to use one, WriteTo. A command that
// returns a Payload cannot have handed over bytes nobody checked, and a writer
// that takes only Payloads cannot be handed anything else.
//
// The zero Payload is empty and writes nothing.
type Payload struct {
	data []byte
}

// Seal makes data a Payload if the rules file is still the one this gate was
// opened on. It is called once the output is assembled, never before: the
// assembly reads notes under the rules file it began with, and what was read
// may be withheld by the rules file as it stands now.
func (g *Gate) Seal(data []byte) (Payload, error) {
	if err := g.Check(); err != nil {
		return Payload{}, err
	}
	return Payload{data: slices.Clone(data)}, nil
}

// WriteTo writes the payload to w and is the only way it leaves. It implements
// io.WriterTo.
func (p Payload) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(p.data)
	return int64(n), err
}
