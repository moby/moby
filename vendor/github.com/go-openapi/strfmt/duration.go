// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package strfmt

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
)

func init() { //nolint:gochecknoinits // registers duration format in the default registry
	d := Duration(0)
	Default.Add("duration", &d, IsDuration)
}

const (
	hoursInDay = 24
	daysInWeek = 7
	nanos      = uint64(time.Nanosecond)
	micros     = uint64(time.Microsecond)
	millis     = uint64(time.Millisecond)
	seconds    = uint64(time.Second)
	minutes    = uint64(time.Minute)
	hours      = uint64(time.Hour)
	days       = uint64(hoursInDay * time.Hour)
	weeks      = uint64(hoursInDay * daysInWeek * time.Hour)
	maxUint64  = uint64(1 << 63)
)

// timeMultiplier holds all supported aliases for duration units, including their plural form.
//
//nolint:gochecknoglobals // package-level lookup tables for duration parsing
var timeMultiplier = map[string]uint64{
	"ns":           nanos,
	"nano":         nanos,
	"nanosecond":   nanos,
	"nanoseconds":  nanos,
	"nanos":        nanos,
	"us":           micros,
	"µs":           micros, // U+00B5 = micro symbol
	"μs":           micros, // U+03BC = Greek letter mu
	"micro":        micros,
	"micros":       micros,
	"microsecond":  micros,
	"microseconds": micros,
	"ms":           millis,
	"milli":        millis,
	"millis":       millis,
	"millisecond":  millis,
	"milliseconds": millis,
	"s":            seconds,
	"sec":          seconds,
	"secs":         seconds,
	"second":       seconds,
	"seconds":      seconds,
	"m":            minutes,
	"min":          minutes,
	"mins":         minutes,
	"minute":       minutes,
	"minutes":      minutes,
	"h":            hours,
	"hr":           hours,
	"hrs":          hours,
	"hour":         hours,
	"hours":        hours,
	"d":            days,
	"day":          days,
	"days":         days,
	"w":            weeks,
	"wk":           weeks,
	"wks":          weeks,
	"week":         weeks,
	"weeks":        weeks,
}

// IsDuration returns true if the provided string is a valid duration.
func IsDuration(str string) bool {
	_, err := ParseDuration(str)
	return err == nil
}

// Duration represents a duration
//
// Duration stores a period of time as a nanosecond count, with the largest
// representable duration being approximately 290 years.
//
// swagger:strfmt duration.
type Duration time.Duration

// MarshalText turns this instance into text.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// UnmarshalText hydrates this instance from text.
func (d *Duration) UnmarshalText(data []byte) error { // validation is performed later on
	dd, err := ParseDuration(string(data))
	if err != nil {
		return err
	}
	*d = Duration(dd)
	return nil
}

// ParseDuration parses a duration from a string
//
// It is similar to [time.ParseDuration] but support additional units like days and weeks,
// additional abreviations for units and is more tolerant on the presence of blank spaces.
//
// A duration may be negative or fractional.
//
// # Differences with [time.ParseDuration]
//
//   - more supported units and aliases (see below)
//   - sign followed by blank space is tolerated
//   - tolerates blanks between duration and unit (e.g. "300 ms")
//
// # Supported units
//
// Units may be specified using aliases or a plural form.
//
//   - "ns", "nano", "nanosecond", "nanoseconds", "nanos"
//   - "us", "µs" (U+00B5 = micro symbol), "μs" (U+03BC = Greek letter mu), "micro", "micros", "microsecond", "microseconds"
//   - "ms", "milli", "millis", "millisecond", "milliseconds"
//   - "s", "sec", "secs", "second", "seconds"
//   - "m", "min", "mins", "minute", "minutes"
//   - "h", "hr", "hrs", "hour", "hours"
//   - "d", "day", "days"
//   - "w", "wk", "wks", "week", "weeks"
//
// NOTE: inspired by scala duration syntax.
//
// # Examples
//
// "300ms", "-1.5h", "2h45m",
// ".5 week",
// "2 minutes 45 seconds".
//
//nolint:gocognit,gocyclo,cyclop // complexity is only slightly above the usual level, may be tolerated as it mimicks the stdlib.
func ParseDuration(s string) (time.Duration, error) {
	// NOTE: this code is largely inspired by the standard library.
	orig := s
	var d uint64
	neg := false

	// Consume [-+]?
	if s != "" {
		c := s[0]
		if c == '-' || c == '+' {
			neg = c == '-'
			s = s[1:]
		}
	}

	// Consume space
	s = strings.TrimLeftFunc(s, unicode.IsSpace)

	// Special case: if all that is left is "0", this is zero.
	if s == "0" {
		return 0, nil
	}

	if s == "" {
		return 0, parseDurationError(orig, "empty duration")
	}

	for s != "" {
		var (
			v, f  uint64      // integers before, after decimal point
			scale float64 = 1 // value = v + f/scale
		)
		s = strings.TrimLeftFunc(s, unicode.IsSpace)

		// The next character must be 0-9.]
		if s[0] != '.' && ('0' > s[0] || s[0] > '9') {
			return 0, parseDurationError(orig, fmt.Sprintf("expected a numerical value, but got %q", s[0]))
		}

		// Consume integer part [0-9]*
		pl := len(s)
		var ok bool
		v, s, ok = leadingInt(s)
		if !ok {
			return 0, parseDurationError(orig, "expected a leading integer part")
		}
		pre := pl != len(s) // whether we consumed anything before a period

		// Consume fractional part (\.[0-9]*)?
		post := false
		if s != "" && s[0] == '.' {
			s = s[1:]
			pl := len(s)
			f, scale, s = leadingFraction(s)
			post = pl != len(s)
		}

		if !pre && !post {
			// no digits (e.g. ".s" or "-.s")
			return 0, parseDurationError(orig, "expected digits")
		}

		// Consume space.
		s = strings.TrimLeftFunc(s, unicode.IsSpace)

		// Consume unit.
		i := 0
		for ; i < len(s); i++ {
			c := s[i]
			if c == '.' || '0' <= c && c <= '9' || unicode.IsSpace(rune(c)) {
				break
			}
		}

		if i == 0 {
			return 0, parseDurationError(orig, "missing unit in duration")
		}

		u := s[:i]
		s = s[i:]
		unit, ok := timeMultiplier[u]
		if !ok {
			return 0, parseDurationError(orig, fmt.Sprintf("unknown unit %q in duration", u))
		}

		if v > maxUint64/unit {
			// overflow
			return 0, parseDurationError(orig, "numerical overflow")
		}

		v *= unit
		if f > 0 {
			// float64 is needed to be nanosecond accurate for fractions of hours.
			// v >= 0 && (f*unit/scale) <= 3.6e+12 (ns/h, h is the largest unit)
			v += uint64(float64(f) * (float64(unit) / scale))
			if v > maxUint64 {
				// overflow
				return 0, parseDurationError(orig, "numerical overflow")
			}
		}

		d += v
		if d > maxUint64 {
			return 0, parseDurationError(orig, "numerical overflow")
		}
	}

	if neg {
		return -time.Duration(d), nil
	}

	if d > maxUint64-1 {
		return 0, parseDurationError(orig, "numerical overflow")
	}

	return time.Duration(d), nil
}

// Scan reads a Duration value from database driver type.
func (d *Duration) Scan(raw any) error {
	switch v := raw.(type) {
	// Proposal for enhancement: case []byte: // ?
	case int64:
		*d = Duration(v)
	case float64:
		*d = Duration(int64(v))
	case nil:
		*d = Duration(0)
	default:
		return fmt.Errorf("cannot sql.Scan() strfmt.Duration from: %#v: %w", v, ErrFormat)
	}

	return nil
}

// Value converts Duration to a primitive value ready to be written to a database.
func (d Duration) Value() (driver.Value, error) {
	return driver.Value(int64(d)), nil
}

// String converts this duration to a string.
func (d Duration) String() string {
	return time.Duration(d).String()
}

// MarshalJSON returns the Duration as JSON.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON sets the Duration from JSON.
func (d *Duration) UnmarshalJSON(data []byte) error {
	if string(data) == jsonNull {
		return nil
	}

	var dstr string
	if err := json.Unmarshal(data, &dstr); err != nil {
		return err
	}
	tt, err := ParseDuration(dstr)
	if err != nil {
		return err
	}
	*d = Duration(tt)
	return nil
}

// DeepCopyInto copies the receiver and writes its value into out.
func (d *Duration) DeepCopyInto(out *Duration) {
	*out = *d
}

// DeepCopy copies the receiver into a new Duration.
func (d *Duration) DeepCopy() *Duration {
	if d == nil {
		return nil
	}
	out := new(Duration)
	d.DeepCopyInto(out)
	return out
}

func parseDurationError(s, msg string) error {
	if msg == "" {
		return fmt.Errorf("invalid duration: %s: %w", s, ErrFormat)
	}

	return fmt.Errorf("invalid duration: %s: %s: %w", s, msg, ErrFormat)
}

// leadingInt consumes the leading [0-9]* from s.
func leadingInt[bytes []byte | string](s bytes) (x uint64, rem bytes, ok bool) { //nolint:ireturn // false positive
	i := 0
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}

		if x > maxUint64/10 { // overflow
			return 0, rem, false
		}

		x = x*10 + uint64(c) - '0'
		if x > maxUint64 { // overflow
			return 0, rem, false
		}
	}

	return x, s[i:], true
}

// leadingFraction consumes the leading [0-9]* from s.
// //
// It is used only for fractions, so it does not return an error on overflow,
// it just stops accumulating precision.
func leadingFraction(s string) (x uint64, scale float64, rem string) {
	i := 0
	scale = 1
	overflow := false
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}

		if overflow {
			continue
		}

		if x > (maxUint64-1)/10 {
			// It's possible for overflow to give a positive number, so take care.
			overflow = true
			continue
		}

		y := x*10 + uint64(c) - '0'
		if y > maxUint64 {
			overflow = true
			continue
		}

		x = y
		scale *= 10
	}

	return x, scale, s[i:]
}
