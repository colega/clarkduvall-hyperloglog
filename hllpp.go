package hll

import (
	"errors"
	"hash"
	"sort"
)

const pPrime uint8 = 25
const mPrime uint32 = 1 << (uint32(pPrime) - 1)

var threshold = []uint {
	10, 20, 40, 80, 220, 400, 900, 1800, 3100,
	6500, 11500, 20000, 50000, 120000, 350000,
}

type HyperLogLogPlus struct {
	reg        []uint8
	p          uint8
	m          uint32
	sparse     bool
	tmpSet     set
	sparseList *compressedList
}

func (h *HyperLogLogPlus) encodeHash(x uint64) uint32 {
	idx := uint32(eb64(x, 64, 64 - pPrime))

	if eb64(x, 64 - h.p, 64 - pPrime) == 0 {
		zeros := clz64((eb64(x, 64 - pPrime, 0) << pPrime) | (1 << pPrime - 1)) + 1
		return idx << 7 | uint32(zeros << 1) | 1
	}
	return idx << 1
}

func (h *HyperLogLogPlus) getIndex(k uint32) uint32 {
	if k & 1 == 1 {
		return eb32(k, 32, 32 - h.p)
	}
	return eb32(k, pPrime + 1, pPrime - h.p + 1)
}

func (h *HyperLogLogPlus) decodeHash(k uint32) (uint32, uint8) {
	var r uint8
	if k & 1 == 1 {
		r = uint8(eb32(k, 7 , 1)) + pPrime - h.p
	} else {
		r = clz32(k << (32 - pPrime + h.p - 1)) + 1
	}
	return h.getIndex(k), r
}

func (h *HyperLogLogPlus) merge() {
	keys := make(sortableSlice, 0, len(h.tmpSet))
	for k := range h.tmpSet {
		keys = append(keys, k)
	}
	sort.Sort(keys)

	newList := newCompressedList(int(h.m))
	for iter, i := h.sparseList.Iter(), 0; iter.HasNext() || i < len(keys); {
		if !iter.HasNext() {
			newList.Append(keys[i])
			i++
			continue
		}

		if i >= len(keys) {
			newList.Append(iter.Next())
			continue
		}

		x1, x2 := iter.Peek(), keys[i]
		if x1 == x2 {
			newList.Append(iter.Next())
			i++
		} else if x1 > x2 {
			newList.Append(x2)
			i++
		} else {
			newList.Append(iter.Next())
		}
	}

	h.sparseList = newList
	h.tmpSet = set{}
}

func NewHyperLogLogPlus(precision uint8) (*HyperLogLogPlus, error) {
	if precision > 18 || precision < 4 {
		return nil, errors.New("precision must be between 4 and 16")
	}

	h := &HyperLogLogPlus{}
	h.p = precision
	h.m = 1 << precision
	h.sparse = true
	h.tmpSet = set{}
	h.sparseList = newCompressedList(int(h.m))
	return h, nil
}

func (h *HyperLogLogPlus) Clear() {
	h.sparse = true
	h.tmpSet = set{}
	h.sparseList = newCompressedList(int(h.m))
	h.reg = nil
}

func (h *HyperLogLogPlus) toNormal() {
	h.reg = make([]uint8, h.m)
	for iter := h.sparseList.Iter(); iter.HasNext(); {
		i, r := h.decodeHash(iter.Next())
		if h.reg[i] < r {
			h.reg[i] = r
		}
	}

	h.sparse = false
	h.tmpSet = nil
	h.sparseList = nil
}

func (h *HyperLogLogPlus) Add(item hash.Hash64) {
	x := item.Sum64()
	if h.sparse {
		h.tmpSet.Add(h.encodeHash(x))

		if uint32(len(h.tmpSet)) * 100 > h.m {
			h.merge()
			if uint32(h.sparseList.Len()) > h.m {
				h.toNormal()
			}
		}
	} else {
		i := eb64(x, 64, 64 - h.p)      // {x63,...,x64-p}
		w := x << h.p | 1 << (h.p - 1)  // {x63-p,...,x0}

		zeroBits := clz64(w) + 1
		if zeroBits > h.reg[i] {
			h.reg[i] = zeroBits
		}
	}
}

func (h *HyperLogLogPlus) estimateBias(est float64) float64 {
	estTable, biasTable := rawEstimateData[h.p - 4], biasData[h.p - 4]

	if estTable[0] > est {
		return estTable[0] - biasTable[0]
	}

	lastEstimate := estTable[len(estTable)-1]
	if lastEstimate < est {
		return lastEstimate - biasTable[len(biasTable)-1]
	}

	var i int
	for i = 0; i < len(estTable) && estTable[i] < est; i++ {}

	e1, b1 := estTable[i - 1], biasTable[i - 1]
	e2, b2 := estTable[i], biasTable[i]

	c := (est - e1) / (e2 - e1)
	return b1 * (1 - c) + b2 * c
}

func (h *HyperLogLogPlus) Count() uint64 {
	if h.sparse {
		h.merge()
		return uint64(linearCounting(mPrime, mPrime - uint32(h.sparseList.Count)))
	}

	est := calculateEstimate(h.reg)
	if est <= float64(h.m) * 5.0 {
		est -= h.estimateBias(est)
	}

	if v := countZeros(h.reg); v != 0 {
		lc := linearCounting(h.m, v)
		if lc <= float64(threshold[h.p - 4]) {
			return uint64(lc)
		}
	}
	return uint64(est)
}
