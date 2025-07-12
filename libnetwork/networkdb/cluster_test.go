package networkdb

import (
	"maps"
	"math"
	"math/bits"
	"slices"
	"strings"
	"testing"

	"github.com/montanaflynn/stats"
	"pgregory.net/rapid"
)

func TestMRandomNodes(t *testing.T) {
	cfg := DefaultConfig()
	// The easiest way to ensure that we don't accidentally generate node
	// IDs that match the local one is to include runes that the generator
	// will never emit.
	cfg.NodeID = "_thisnode"
	uut := newNetworkDB(cfg)

	t.Run("EmptySlice", func(t *testing.T) {
		sample := uut.mRandomNodes(3, nil)
		if len(sample) != 0 {
			t.Errorf("got sample size %d, want 0", len(sample))
		}
	})

	t.Run("OnlyLocalNode", func(t *testing.T) {
		sample := uut.mRandomNodes(3, []string{cfg.NodeID})
		if len(sample) != 0 {
			t.Errorf("got sample size %d, want 0", len(sample))
		}
	})

	gen := rapid.Custom(func(t *rapid.T) []string {
		s := rapid.SliceOfNDistinct(rapid.StringMatching(`[a-z]{10}`), 0, 100, rapid.ID).Draw(t, "node-names")
		insertPoint := rapid.IntRange(0, len(s)).Draw(t, "insertPoint")
		return slices.Insert(s, insertPoint, cfg.NodeID)
	})

	rapid.Check(t, func(t *rapid.T) {
		nodes := gen.Draw(t, "nodes")
		m := rapid.IntRange(0, len(nodes)).Draw(t, "m")

		takeSample := func() []string {
			sample := uut.mRandomNodes(m, nodes)
			if len(sample) != min(m, len(nodes)-1) {
				t.Errorf("got sample size %d, want %d", len(sample), min(m, len(nodes)-1))
			}
			if idx := slices.Index(sample, cfg.NodeID); idx >= 0 {
				t.Errorf("sample contains local node ID at index %d\n%v", idx, sample)
			}
			assertUniqueElements(t, sample)
			return sample
		}

		p := kpermutations(uint64(len(nodes)-1), uint64(m))
		switch {
		case p <= 1:
			// Only one permutation is possible, so cannot test randomness.
			// Assert the other properties by taking a few samples.
			for range 100 {
				_ = takeSample()
			}
			return
		case p <= 10:
			// With a small number of possible k-permutations, we
			// can feasibly test how many samples it takes to get
			// all of them.
			seen := make(map[string]bool)
			var i int
			for i = range 10000 {
				sample := takeSample()
				seen[strings.Join(sample, ",")] = true
				if len(seen) == int(p) {
					break
				}
			}
			if len(seen) != int(p) {
				t.Errorf("did not see all %d permutations after %d trials", p, i+1)
			}
			t.Logf("saw all %d permutations after %d samples", p, i+1)
		default:
			uniques := 0
			sample1 := takeSample()
			for range 10 {
				sample2 := takeSample()
				if !slices.Equal(sample1, sample2) {
					uniques++
				}
			}
			if uniques == 0 {
				t.Error("mRandomNodes returned the same sample multiple times")
			}
		}

		// We are testing randomness so statistical outliers are
		// occasionally expected even when the probability
		// distribution is uniform. Run multiple trials to make
		// test flakes unlikely in practice.
		extremes := 0
		for range 10 {
			counts := make(map[string]int)
			for _, n := range nodes {
				if n != cfg.NodeID {
					counts[n] = 0
				}
			}
			const samples = 10000
			for range samples {
				for _, n := range uut.mRandomNodes(m, nodes) {
					counts[n]++
				}
			}
			// Adding multiple samples together should yield a normal distribution
			// if the samples are unbiased.
			countsf := stats.LoadRawData(slices.Collect(maps.Values(counts)))
			nf := stats.NormFit(countsf)
			mean, stdev := nf[0], nf[1]
			minv, _ := countsf.Min()
			maxv, _ := countsf.Max()
			if minv < mean-4*stdev || maxv > mean+4*stdev {
				extremes++
				t.Logf("Mean: %f, StdDev: %f, Min: %f, Max: %f", mean, stdev, minv, maxv)
			}
		}
		if extremes > 2 {
			t.Errorf("outliers in distribution: %d/10 trials, expected <2/10", extremes)
		}
	})
}

func assertUniqueElements[S ~[]E, E comparable](t rapid.TB, s S) {
	t.Helper()
	counts := make(map[E]int)
	for _, e := range s {
		counts[e]++
	}
	for e, c := range counts {
		if c > 1 {
			t.Errorf("element %v appears %d times in the slice, expected 1", e, c)
		}
	}
}

// kpermutations returns P(n,k), the number of permutations of k elements chosen
// from a set of size n. The calculation is saturating: if the result is larger than
// can be represented by a uint64, math.MaxUint64 is returned.
func kpermutations(n, k uint64) uint64 {
	if k > n {
		return 0
	}
	if k == 0 || n == 0 {
		return 1
	}
	p := uint64(1)
	for i := range k {
		var hi uint64
		hi, p = bits.Mul64(p, n-i)
		if hi != 0 {
			return math.MaxUint64
		}
	}
	return p
}
