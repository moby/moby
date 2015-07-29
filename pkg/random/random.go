package random

import (
	"io"
	"math/rand"
	"sync"
	"time"
)

// Rand is a global *rand.Rand instance, which initilized with NewSource() source.
var Rand = rand.New(NewSource())

// Reader is a global, shared instance of a pseudorandom bytes generator.
// It doesn't consume entropy.
var Reader io.Reader = &reader{rnd: Rand}

// copypaste from standard math/rand
type lockedSource struct {
	lk  sync.Mutex
	src rand.Source
}

func (r *lockedSource) Int63() (n int64) {
	r.lk.Lock()
	n = r.src.Int63()
	r.lk.Unlock()
	return
}

func (r *lockedSource) Seed(seed int64) {
	r.lk.Lock()
	r.src.Seed(seed)
	r.lk.Unlock()
}

// NewSource returns math/rand.Source safe for concurrent use and initialized
// with current unix-nano timestamp
func NewSource() rand.Source {
	return &lockedSource{
		src: rand.NewSource(time.Now().UnixNano()),
	}
}

type reader struct {
	rnd *rand.Rand
}

func (r *reader) Read(b []byte) (int, error) {
	i := 0
	for {
		val := r.rnd.Int63()
		for val > 0 {
			b[i] = byte(val)
			i++
			if i == len(b) {
				return i, nil
			}
			val >>= 8
		}
	}
}
