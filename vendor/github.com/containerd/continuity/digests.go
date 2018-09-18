package continuity

import (
	"fmt"
	"io"
	"sort"

	"github.com/opencontainers/go-digest"
)

// Digester produces a digest for a given read stream
type Digester interface {
	Digest(io.Reader) (digest.Digest, error)
}

// ContentProvider produces a read stream for a given digest
type ContentProvider interface {
	Reader(digest.Digest) (io.ReadCloser, error)
}

type simpleDigester struct {
	algorithm digest.Algorithm
}

func (sd simpleDigester) Digest(r io.Reader) (digest.Digest, error) {
	digester := sd.algorithm.Digester()

	if _, err := io.Copy(digester.Hash(), r); err != nil {
		return "", err
	}

	return digester.Digest(), nil
}

// uniqifyDigests sorts and uniqifies the provided digest, ensuring that the
// digests are not repeated and no two digests with the same algorithm have
// different values. Because a stable sort is used, this has the effect of
// "zipping" digest collections from multiple resources.
func uniqifyDigests(digests ...digest.Digest) ([]digest.Digest, error) {
	sort.Stable(digestSlice(digests)) // stable sort is important for the behavior here.
	seen := map[digest.Digest]struct{}{}
	algs := map[digest.Algorithm][]digest.Digest{} // detect different digests.

	var out []digest.Digest
	// uniqify the digests
	for _, d := range digests {
		if _, ok := seen[d]; ok {
			continue
		}

		seen[d] = struct{}{}
		algs[d.Algorithm()] = append(algs[d.Algorithm()], d)

		if len(algs[d.Algorithm()]) > 1 {
			return nil, fmt.Errorf("conflicting digests for %v found", d.Algorithm())
		}

		out = append(out, d)
	}

	return out, nil
}

// digestsMatch compares the two sets of digests to see if they match.
func digestsMatch(as, bs []digest.Digest) bool {
	all := append(as, bs...)

	uniqified, err := uniqifyDigests(all...)
	if err != nil {
		// the only error uniqifyDigests returns is when the digests disagree.
		return false
	}

	disjoint := len(as) + len(bs)
	if len(uniqified) == disjoint {
		// if these two sets have the same cardinality, we know both sides
		// didn't share any digests.
		return false
	}

	return true
}

type digestSlice []digest.Digest

func (p digestSlice) Len() int           { return len(p) }
func (p digestSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p digestSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
