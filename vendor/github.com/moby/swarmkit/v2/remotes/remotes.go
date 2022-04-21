package remotes

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/moby/swarmkit/v2/api"
)

var errRemotesUnavailable = fmt.Errorf("no remote hosts provided")

// DefaultObservationWeight provides a weight to use for positive observations
// that will balance well under repeated observations.
const DefaultObservationWeight = 10

// Remotes keeps track of remote addresses by weight, informed by
// observations.
type Remotes interface {
	// Weight returns the remotes with their current weights.
	Weights() map[api.Peer]int

	// Select a remote from the set of available remotes with optionally
	// excluding ID or address.
	Select(...string) (api.Peer, error)

	// Observe records an experience with a particular remote. A positive weight
	// indicates a good experience and a negative weight a bad experience.
	//
	// The observation will be used to calculate a moving weight, which is
	// implementation dependent. This method will be called such that repeated
	// observations of the same master in each session request are favored.
	Observe(peer api.Peer, weight int)

	// ObserveIfExists records an experience with a particular remote if when a
	// remote exists.
	ObserveIfExists(peer api.Peer, weight int)

	// Remove the remote from the list completely.
	Remove(addrs ...api.Peer)
}

// NewRemotes returns a Remotes instance with the provided set of addresses.
// Entries provided are heavily weighted initially.
func NewRemotes(peers ...api.Peer) Remotes {
	mwr := &remotesWeightedRandom{
		remotes: make(map[api.Peer]int),
	}

	for _, peer := range peers {
		mwr.Observe(peer, DefaultObservationWeight)
	}

	return mwr
}

type remotesWeightedRandom struct {
	remotes map[api.Peer]int
	mu      sync.Mutex

	// workspace to avoid reallocation. these get lazily allocated when
	// selecting values.
	cdf   []float64
	peers []api.Peer
}

func (mwr *remotesWeightedRandom) Weights() map[api.Peer]int {
	mwr.mu.Lock()
	defer mwr.mu.Unlock()

	ms := make(map[api.Peer]int, len(mwr.remotes))
	for addr, weight := range mwr.remotes {
		ms[addr] = weight
	}

	return ms
}

func (mwr *remotesWeightedRandom) Select(excludes ...string) (api.Peer, error) {
	mwr.mu.Lock()
	defer mwr.mu.Unlock()

	// NOTE(stevvooe): We then use a weighted random selection algorithm
	// (http://stackoverflow.com/questions/4463561/weighted-random-selection-from-array)
	// to choose the master to connect to.
	//
	// It is possible that this is insufficient. The following may inform a
	// better solution:

	// https://github.com/LK4D4/sample
	//
	// The first link applies exponential distribution weight choice reservoir
	// sampling. This may be relevant if we view the master selection as a
	// distributed reservoir sampling problem.

	// bias to zero-weighted remotes have same probability. otherwise, we
	// always select first entry when all are zero.
	const bias = 0.001

	// clear out workspace
	mwr.cdf = mwr.cdf[:0]
	mwr.peers = mwr.peers[:0]

	cum := 0.0
	// calculate CDF over weights
Loop:
	for peer, weight := range mwr.remotes {
		for _, exclude := range excludes {
			if peer.NodeID == exclude || peer.Addr == exclude {
				// if this peer is excluded, ignore it by continuing the loop to label Loop
				continue Loop
			}
		}
		if weight < 0 {
			// treat these as zero, to keep there selection unlikely.
			weight = 0
		}

		cum += float64(weight) + bias
		mwr.cdf = append(mwr.cdf, cum)
		mwr.peers = append(mwr.peers, peer)
	}

	if len(mwr.peers) == 0 {
		return api.Peer{}, errRemotesUnavailable
	}

	r := mwr.cdf[len(mwr.cdf)-1] * rand.Float64()
	i := sort.SearchFloat64s(mwr.cdf, r)

	return mwr.peers[i], nil
}

func (mwr *remotesWeightedRandom) Observe(peer api.Peer, weight int) {
	mwr.mu.Lock()
	defer mwr.mu.Unlock()

	mwr.observe(peer, float64(weight))
}

func (mwr *remotesWeightedRandom) ObserveIfExists(peer api.Peer, weight int) {
	mwr.mu.Lock()
	defer mwr.mu.Unlock()

	if _, ok := mwr.remotes[peer]; !ok {
		return
	}

	mwr.observe(peer, float64(weight))
}

func (mwr *remotesWeightedRandom) Remove(addrs ...api.Peer) {
	mwr.mu.Lock()
	defer mwr.mu.Unlock()

	for _, addr := range addrs {
		delete(mwr.remotes, addr)
	}
}

const (
	// remoteWeightSmoothingFactor for exponential smoothing. This adjusts how
	// much of the // observation and old value we are using to calculate the new value.
	// See
	// https://en.wikipedia.org/wiki/Exponential_smoothing#Basic_exponential_smoothing
	// for details.
	remoteWeightSmoothingFactor = 0.5
	remoteWeightMax             = 1 << 8
)

func clip(x float64) float64 {
	if math.IsNaN(x) {
		// treat garbage as such
		// acts like a no-op for us.
		return 0
	}
	return math.Max(math.Min(remoteWeightMax, x), -remoteWeightMax)
}

func (mwr *remotesWeightedRandom) observe(peer api.Peer, weight float64) {

	// While we have a decent, ad-hoc approach here to weight subsequent
	// observations, we may want to look into applying forward decay:
	//
	//  http://dimacs.rutgers.edu/~graham/pubs/papers/fwddecay.pdf
	//
	// We need to get better data from behavior in a cluster.

	// makes the math easier to read below
	var (
		w0 = float64(mwr.remotes[peer])
		w1 = clip(weight)
	)
	const α = remoteWeightSmoothingFactor

	// Multiply the new value to current value, and appy smoothing against the old
	// value.
	wn := clip(α*w1 + (1-α)*w0)

	mwr.remotes[peer] = int(math.Ceil(wn))
}
