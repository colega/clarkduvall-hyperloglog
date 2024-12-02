package hyperloglog

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/DmitriyVTitov/size"
	"github.com/stretchr/testify/require"
)

func TestHLL64Count(t *testing.T) {
	h, _ := New64(16)

	n := h.Count()
	if n != 0 {
		t.Error(n)
	}

	h.AddUint64(0x00010fff)
	h.AddUint64(0x00020fff)
	h.AddUint64(0x00030fff)
	h.AddUint64(0x00040fff)
	h.AddUint64(0x00050fff)
	h.AddUint64(0x00050fff)

	n = h.Count()
	if n != 5 {
		t.Error(n)
	}
}

func BenchmarkHLL64_Count(b *testing.B) {
	for _, precision := range []uint8{14, 15, 16, 17, 18} {
		b.Run(fmt.Sprintf("precision=%d", precision), func(b *testing.B) {
			h, err := New64(precision)
			require.NoError(b, err)
			for i := 0; i < 1e6; i++ {
				h.AddUint64(rand.Uint64())
			}
			b.ResetTimer()
			c := uint64(0)
			for i := 0; i < b.N; i++ {
				c += h.Count()
			}
			require.NotZero(b, c)
		})
	}
}

func TestHLL64CountMany(t *testing.T) {
	for _, count := range []uint64{1e6, 1e7, 1e8, 5e8} {
		t.Run(fmt.Sprintf("count=%d", count), func(t *testing.T) {
			seen := make(map[uint64]struct{}, count)

			h, err := New64(16)
			require.NoError(t, err)

			require.Zero(t, h.Count())
			for i := uint64(0); i < count; i++ {
				x := rand.Uint64()
				for _, ok := seen[x]; ok; _, ok = seen[x] {
					x = rand.Uint64()
				}

				h.AddUint64(x)
				seen[x] = struct{}{}
			}

			gotCount := h.Count()
			t.Logf("size: %d", size.Of(h))
			t.Logf("error: %0.3f%%", 100*(float64(gotCount)-float64(count))/float64(count))
			require.InEpsilonf(t, count, gotCount, 0.02, "expected %d, got %d", count, gotCount)
		})
	}
}

func TestHLL64Seen(t *testing.T) {
	for _, count := range []uint64{1e6} {
		t.Run(fmt.Sprintf("count=%d", count), func(t *testing.T) {
			seen := make(map[uint64]struct{}, count)

			h, err := New64(18)
			require.NoError(t, err)

			require.Zero(t, h.Count())
			falsePositives := 0
			falseNegatives := 0
			for i := uint64(0); i < count; i++ {
				x := rand.Uint64()
				for _, ok := seen[x]; ok; _, ok = seen[x] {
					x = rand.Uint64()
				}
				if h.SeenUint64(x) {
					falsePositives++
				}
				h.AddUint64(x)
				if !h.SeenUint64(x) {
					falseNegatives++
				}
				seen[x] = struct{}{}
				if i%128 == 0 {
					require.InEpsilonf(t, i+1, h.Count(), 0.05, "expected %d, got %d", i, h.Count())
				}
			}

			gotCount := h.Count()
			t.Logf("size: %d", size.Of(h))
			t.Logf("false negatives: %d", falseNegatives)
			t.Logf("false positives: %d", falsePositives)
			t.Logf("false positives pct: %0.3f%%", 100*float64(falsePositives)/float64(count))
			t.Logf("error: %0.3f%%", 100*(float64(gotCount)-float64(count))/float64(count))
			require.InEpsilonf(t, count, gotCount, 0.02, "expected %d, got %d", count, gotCount)
		})
	}
}
