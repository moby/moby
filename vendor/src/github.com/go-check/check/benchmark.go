// Copyright (c) 2012 The Go Authors. All rights reserved.
// 
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
// 
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
// 
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package check

import (
	"fmt"
	"runtime"
	"time"
)

var memStats runtime.MemStats

// testingB is a type passed to Benchmark functions to manage benchmark
// timing and to specify the number of iterations to run.
type timer struct {
	start     time.Time // Time test or benchmark started
	duration  time.Duration
	N         int
	bytes     int64
	timerOn   bool
	benchTime time.Duration
	// The initial states of memStats.Mallocs and memStats.TotalAlloc.
	startAllocs uint64
	startBytes  uint64
	// The net total of this test after being run.
	netAllocs uint64
	netBytes  uint64
}

// StartTimer starts timing a test. This function is called automatically
// before a benchmark starts, but it can also used to resume timing after
// a call to StopTimer.
func (c *C) StartTimer() {
	if !c.timerOn {
		c.start = time.Now()
		c.timerOn = true

		runtime.ReadMemStats(&memStats)
		c.startAllocs = memStats.Mallocs
		c.startBytes = memStats.TotalAlloc
	}
}

// StopTimer stops timing a test. This can be used to pause the timer
// while performing complex initialization that you don't
// want to measure.
func (c *C) StopTimer() {
	if c.timerOn {
		c.duration += time.Now().Sub(c.start)
		c.timerOn = false
		runtime.ReadMemStats(&memStats)
		c.netAllocs += memStats.Mallocs - c.startAllocs
		c.netBytes += memStats.TotalAlloc - c.startBytes
	}
}

// ResetTimer sets the elapsed benchmark time to zero.
// It does not affect whether the timer is running.
func (c *C) ResetTimer() {
	if c.timerOn {
		c.start = time.Now()
		runtime.ReadMemStats(&memStats)
		c.startAllocs = memStats.Mallocs
		c.startBytes = memStats.TotalAlloc
	}
	c.duration = 0
	c.netAllocs = 0
	c.netBytes = 0
}

// SetBytes informs the number of bytes that the benchmark processes
// on each iteration. If this is called in a benchmark it will also
// report MB/s.
func (c *C) SetBytes(n int64) {
	c.bytes = n
}

func (c *C) nsPerOp() int64 {
	if c.N <= 0 {
		return 0
	}
	return c.duration.Nanoseconds() / int64(c.N)
}

func (c *C) mbPerSec() float64 {
	if c.bytes <= 0 || c.duration <= 0 || c.N <= 0 {
		return 0
	}
	return (float64(c.bytes) * float64(c.N) / 1e6) / c.duration.Seconds()
}

func (c *C) timerString() string {
	if c.N <= 0 {
		return fmt.Sprintf("%3.3fs", float64(c.duration.Nanoseconds())/1e9)
	}
	mbs := c.mbPerSec()
	mb := ""
	if mbs != 0 {
		mb = fmt.Sprintf("\t%7.2f MB/s", mbs)
	}
	nsop := c.nsPerOp()
	ns := fmt.Sprintf("%10d ns/op", nsop)
	if c.N > 0 && nsop < 100 {
		// The format specifiers here make sure that
		// the ones digits line up for all three possible formats.
		if nsop < 10 {
			ns = fmt.Sprintf("%13.2f ns/op", float64(c.duration.Nanoseconds())/float64(c.N))
		} else {
			ns = fmt.Sprintf("%12.1f ns/op", float64(c.duration.Nanoseconds())/float64(c.N))
		}
	}
	memStats := ""
	if c.benchMem {
		allocedBytes := fmt.Sprintf("%8d B/op", int64(c.netBytes)/int64(c.N))
		allocs := fmt.Sprintf("%8d allocs/op", int64(c.netAllocs)/int64(c.N))
		memStats = fmt.Sprintf("\t%s\t%s", allocedBytes, allocs)
	}
	return fmt.Sprintf("%8d\t%s%s%s", c.N, ns, mb, memStats)
}

func min(x, y int) int {
	if x > y {
		return y
	}
	return x
}

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

// roundDown10 rounds a number down to the nearest power of 10.
func roundDown10(n int) int {
	var tens = 0
	// tens = floor(log_10(n))
	for n > 10 {
		n = n / 10
		tens++
	}
	// result = 10^tens
	result := 1
	for i := 0; i < tens; i++ {
		result *= 10
	}
	return result
}

// roundUp rounds x up to a number of the form [1eX, 2eX, 5eX].
func roundUp(n int) int {
	base := roundDown10(n)
	if n < (2 * base) {
		return 2 * base
	}
	if n < (5 * base) {
		return 5 * base
	}
	return 10 * base
}
