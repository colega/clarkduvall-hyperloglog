package main

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

type hyperLogLogPP struct {
	reg         []uint8
	p           uint8
	m           uint32
	sparse      bool
	tmp_set     set
	sparse_list []uint32
}

func (h *hyperLogLogPP) encodeHash(x uint64) uint32 {
	idx := uint32(eb64(x, 64, 64 - pPrime) << 7)

	if eb64(x, 64 - h.p, 64 - pPrime) == 0 {
		zeros := clz64(eb64(x, 64 - pPrime, 0) << pPrime) + 1
		return idx | uint32(zeros << 1) | 1
	}
	return idx | 1 << 6
}

func (h *hyperLogLogPP) getIndex(k uint32) uint32 {
	return eb32(k, h.p + 7, 7)
}

func (h *hyperLogLogPP) decodeHash(k uint32) (uint32, uint8) {
	r := uint8(0)
	if k & 1 == 1 {
		r = uint8(eb32(k, 7 , 1)) + pPrime - h.p
	} else {
		r = clz32(k << (pPrime - h.p)) + 1
	}
	return h.getIndex(k), r
}

func (h *hyperLogLogPP) merge() {
	keys := make(sortableSlice, 0, len(h.tmp_set))
	for k := range h.tmp_set {
		keys = append(keys, k)
	}
	sort.Sort(keys)

	mask := mPrime - 1
	key_less := func(a uint32, b uint32) bool { return a & mask < b & mask }
	key_equal := func(a uint32, b uint32) bool { return a & mask == b & mask }

	i := 0
	for _, k := range keys {
		for ; i < len(h.sparse_list) && key_less(h.sparse_list[i], k); i++ {}

		if i >= len(h.sparse_list) {
			h.sparse_list = append(h.sparse_list, k)
			continue
		}

		item := h.sparse_list[i]
		if k > item {
			if key_equal(k, item) {
				h.sparse_list[i] = k
			} else {
				h.sparse_list = insert(h.sparse_list, i + 1, k)
			}
		} else if key_less(k, item) {
			h.sparse_list = insert(h.sparse_list, i, k)
		}
		i++
	}

	h.tmp_set = set{}
}

func NewHyperLogLogPP(precision uint8) (*hyperLogLogPP, error) {
	if precision > 18 || precision < 4 {
		return nil, errors.New("precision must be between 4 and 16")
	}

	h := new(hyperLogLogPP)
	h.p = precision
	h.m = 1 << precision
	h.sparse = true
	h.tmp_set = set{}
	h.sparse_list = make([]uint32, 0, h.m / 4)
	return h, nil
}

func (h *hyperLogLogPP) Clear() {
	h.sparse = true
	h.tmp_set = set{}
	h.sparse_list = make([]uint32, 0, h.m / 4)
	h.reg = nil
}

func (h *hyperLogLogPP) toNormal() {
	h.reg = make([]uint8, h.m)
	for _, k := range h.sparse_list {
		i, r := h.decodeHash(k)
		if h.reg[i] < r {
			h.reg[i] = r
		}
	}

	h.sparse = false
	h.tmp_set = nil
	h.sparse_list = nil
}

func (h *hyperLogLogPP) Add(item hash.Hash64) {
	x := item.Sum64()
	if h.sparse {
		h.tmp_set.Add(h.encodeHash(x))

		// Hash map takes approximately (4 + 4 + 1) * 2 * 4 * n bytes
		if uint32(len(h.tmp_set)) * 72 > h.m {
			h.merge()
			// Sparse list takes approximately 4 * n bytes. Add 2 extra to account for
			// memory use of tmp_set.
			if uint32(len(h.sparse_list)) * 6 > h.m {
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

func (h *hyperLogLogPP) estimateBias(est float64) float64 {
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
	return b1 * c + b2 * (1 - c)
}

func (h *hyperLogLogPP) Estimate() uint64 {
	if h.sparse {
		h.merge()
		return uint64(linearCounting(mPrime, mPrime - uint32(len(h.sparse_list))))
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