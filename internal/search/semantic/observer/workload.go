package observer

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"slices"
)

var (
	// ErrInvalid means the committed production observer artifact is malformed.
	ErrInvalid = errors.New("semantic observer workload is invalid")
	// ErrIdentityMismatch means the workload belongs to another query protocol.
	ErrIdentityMismatch = errors.New("semantic observer workload identity mismatch")
)

const (
	artifactMagic = "YOB1"
	identitySize  = 32
	headerSize    = len(artifactMagic) + identitySize + 4 + 4
	queryCount    = 40
)

// Workload is a validated, owned production top-k observer workload.
type Workload struct {
	dimension int
	queries   [][]float32
}

type artifactHeader struct {
	dimension int
	count     int
	offset    int
}

// Dimension returns the component count of every query vector.
func (w *Workload) Dimension() int {
	if w == nil {
		return 0
	}
	return w.dimension
}

// QueryVectors returns owned copies of the 40 production observer vectors.
func (w *Workload) QueryVectors() [][]float32 {
	if w == nil {
		return nil
	}
	queries := make([][]float32, len(w.queries))
	for i := range w.queries {
		queries[i] = slices.Clone(w.queries[i])
	}
	return queries
}

func parse(encoded []byte, expected [identitySize]byte) (*Workload, error) {
	if expected == ([identitySize]byte{}) {
		return nil, ErrIdentityMismatch
	}
	decoded, err := base64.StdEncoding.AppendDecode(make([]byte, 0, base64.StdEncoding.DecodedLen(len(encoded))), encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: decode artifact: %w", ErrInvalid, err)
	}
	header, err := parseHeader(decoded, expected)
	if err != nil {
		return nil, err
	}
	queries, err := parseQueries(decoded, header.offset, header.dimension, header.count)
	if err != nil {
		return nil, err
	}
	return &Workload{dimension: header.dimension, queries: queries}, nil
}

func parseHeader(decoded []byte, expected [identitySize]byte) (artifactHeader, error) {
	if len(decoded) < headerSize || string(decoded[:len(artifactMagic)]) != artifactMagic {
		return artifactHeader{}, fmt.Errorf("%w: header", ErrInvalid)
	}
	offset := len(artifactMagic)
	if !bytes.Equal(decoded[offset:offset+identitySize], expected[:]) {
		return artifactHeader{}, ErrIdentityMismatch
	}
	offset += identitySize
	dimension := binary.BigEndian.Uint32(decoded[offset : offset+4])
	offset += 4
	count := binary.BigEndian.Uint32(decoded[offset : offset+4])
	offset += 4
	components := uint64(dimension) * uint64(count)
	wantBytes := uint64(headerSize) + components*4
	if dimension == 0 || count != queryCount || wantBytes != uint64(len(decoded)) || components > uint64(^uint(0)>>1) {
		return artifactHeader{}, fmt.Errorf("%w: dimensions", ErrInvalid)
	}
	return artifactHeader{dimension: int(dimension), count: int(count), offset: offset}, nil
}

func parseQueries(decoded []byte, offset, dimension, count int) ([][]float32, error) {
	queries := make([][]float32, count)
	for query := range queries {
		vector := make([]float32, dimension)
		var nonzero bool
		for component := range vector {
			bits := binary.BigEndian.Uint32(decoded[offset : offset+4])
			offset += 4
			value := math.Float32frombits(bits)
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return nil, fmt.Errorf("%w: query %d component %d", ErrInvalid, query+1, component+1)
			}
			nonzero = nonzero || value != 0
			vector[component] = value
		}
		if !nonzero {
			return nil, fmt.Errorf("%w: query %d is zero", ErrInvalid, query+1)
		}
		queries[query] = vector
	}
	return queries, nil
}
