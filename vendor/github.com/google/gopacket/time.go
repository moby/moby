// Copyright 2018 The GoPacket Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package gopacket

import (
	"fmt"
	"math"
	"time"
)

// TimestampResolution represents the resolution of timestamps in Base^Exponent.
type TimestampResolution struct {
	Base, Exponent int
}

func (t TimestampResolution) String() string {
	return fmt.Sprintf("%d^%d", t.Base, t.Exponent)
}

// ToDuration returns the smallest representable time difference as a time.Duration
func (t TimestampResolution) ToDuration() time.Duration {
	if t.Base == 0 {
		return 0
	}
	if t.Exponent == 0 {
		return time.Second
	}
	switch t.Base {
	case 10:
		return time.Duration(math.Pow10(t.Exponent + 9))
	case 2:
		if t.Exponent < 0 {
			return time.Second >> uint(-t.Exponent)
		}
		return time.Second << uint(t.Exponent)
	default:
		// this might loose precision
		return time.Duration(float64(time.Second) * math.Pow(float64(t.Base), float64(t.Exponent)))
	}
}

// TimestampResolutionInvalid represents an invalid timestamp resolution
var TimestampResolutionInvalid = TimestampResolution{}

// TimestampResolutionMillisecond is a resolution of 10^-3s
var TimestampResolutionMillisecond = TimestampResolution{10, -3}

// TimestampResolutionMicrosecond is a resolution of 10^-6s
var TimestampResolutionMicrosecond = TimestampResolution{10, -6}

// TimestampResolutionNanosecond is a resolution of 10^-9s
var TimestampResolutionNanosecond = TimestampResolution{10, -9}

// TimestampResolutionNTP is the resolution of NTP timestamps which is 2^-32 ≈ 233 picoseconds
var TimestampResolutionNTP = TimestampResolution{2, -32}

// TimestampResolutionCaptureInfo is the resolution used in CaptureInfo, which his currently nanosecond
var TimestampResolutionCaptureInfo = TimestampResolutionNanosecond

// PacketSourceResolution is an interface for packet data sources that
// support reporting the timestamp resolution of the aqcuired timestamps.
// Returned timestamps will always have NanosecondTimestampResolution due
// to the use of time.Time, but scaling might have occured if acquired
// timestamps have a different resolution.
type PacketSourceResolution interface {
	// Resolution returns the timestamp resolution of acquired timestamps before scaling to NanosecondTimestampResolution.
	Resolution() TimestampResolution
}
