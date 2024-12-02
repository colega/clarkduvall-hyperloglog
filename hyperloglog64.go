// Package hyperloglog implements the HyperLogLog and HyperLogLog++ cardinality
// estimation algorithms.
// These algorithms are used for accurately estimating the cardinality of a
// multiset using constant memory. HyperLogLog++ has multiple improvements over
// HyperLogLog, with a much lower error rate for smaller cardinalities.
//
// HyperLogLog is described here:
// http://algo.inria.fr/flajolet/Publications/FlFuGaMe07.pdf
//
// HyperLogLog++ is described here:
// http://research.google.com/pubs/pub40671.html
package hyperloglog

import (
	"bytes"
	"encoding/gob"
	"errors"
	"math"
)

type HyperLogLog64 struct {
	reg []uint8
	m   uint32
	p   uint8
}

// New64 returns a new initialized HyperLogLog64.
func New64(precision uint8) (*HyperLogLog64, error) {
	if precision > 26 || precision < 4 {
		return nil, errors.New("precision must be between 4 and 26")
	}

	h := &HyperLogLog64{}
	h.p = precision
	h.m = 1 << precision
	h.reg = make([]uint8, h.m)
	return h, nil
}

// Clear sets HyperLogLog64 h back to its initial state.
func (h *HyperLogLog64) Clear() {
	h.reg = make([]uint8, h.m)
}

// AddUint64 adds a new hash to HyperLogLog64 h.
func (h *HyperLogLog64) AddUint64(x uint64) {
	i := eb64(x, 64, 64-h.p) // {x63,...,x64-p}
	w := x<<h.p | 1<<(h.p-1) // {x63-p,...,x0}

	zeroBits := clz64(w) + 1
	if zeroBits > h.reg[i] {
		h.reg[i] = zeroBits
	}
}

// Merge takes another HyperLogLog64 and combines it with HyperLogLog64 h.
func (h *HyperLogLog64) Merge(other *HyperLogLog64) error {
	if h.p != other.p {
		return errors.New("precisions must be equal")
	}

	for i, v := range other.reg {
		if v > h.reg[i] {
			h.reg[i] = v
		}
	}
	return nil
}

// Count returns the cardinality estimate.
func (h *HyperLogLog64) Count() uint64 {
	est := calculateEstimate(h.reg)
	if est <= float64(h.m)*2.5 {
		if v := countZeros(h.reg); v != 0 {
			return uint64(linearCounting(h.m, v))
		}
		return uint64(est)
	} else if est < two32/30 {
		return uint64(est)
	}
	return uint64(-two32 * math.Log(1-est/two32))
}

// GobEncode encodes HyperLogLog64 into a gob.
func (h *HyperLogLog64) GobEncode() ([]byte, error) {
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(h.reg); err != nil {
		return nil, err
	}
	if err := enc.Encode(h.m); err != nil {
		return nil, err
	}
	if err := enc.Encode(h.p); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode decodes gob into a HyperLogLog64 structure.
func (h *HyperLogLog64) GobDecode(b []byte) error {
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	if err := dec.Decode(&h.reg); err != nil {
		return err
	}
	if err := dec.Decode(&h.m); err != nil {
		return err
	}
	if err := dec.Decode(&h.p); err != nil {
		return err
	}
	return nil
}
