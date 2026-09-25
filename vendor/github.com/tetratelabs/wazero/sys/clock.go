package sys

// ClockResolution is a positive granularity of clock precision in
// nanoseconds. For example, if the resolution is 1us, this returns 1000.
//
// Note: Some implementations return arbitrary resolution because there's
// no perfect alternative. For example, according to the source in time.go,
// windows monotonic resolution can be 15ms. See /RATIONALE.md.
type ClockResolution uint32

// Walltime returns the current unix/epoch time, seconds since midnight UTC
// 1 January 1970, with a nanosecond fraction.
type Walltime func() (sec int64, nsec int32)

// Nanotime returns nanoseconds since an arbitrary start point, used to measure
// elapsed time. This is sometimes referred to as a tick or monotonic time.
//
// Note: There are no constraints on the value return except that it
// increments. For example, -1 is a valid if the next value is >= 0.
type Nanotime func() int64

// Nanosleep puts the current goroutine to sleep for at least ns nanoseconds.
type Nanosleep func(ns int64)

// Osyield yields the processor, typically to implement spin-wait loops.
type Osyield func()
