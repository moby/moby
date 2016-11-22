package dispatcher

import (
	"math/rand"
	"time"
)

type periodChooser struct {
	period  time.Duration
	epsilon time.Duration
	rand    *rand.Rand
}

func newPeriodChooser(period, eps time.Duration) *periodChooser {
	return &periodChooser{
		period:  period,
		epsilon: eps,
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (pc *periodChooser) Choose() time.Duration {
	var adj int64
	if pc.epsilon > 0 {
		adj = rand.Int63n(int64(2*pc.epsilon)) - int64(pc.epsilon)
	}
	return pc.period + time.Duration(adj)
}
