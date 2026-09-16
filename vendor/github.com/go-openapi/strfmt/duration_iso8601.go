// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package strfmt

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ISO 8601 duration unit lengths, expressed in nanoseconds.
//
// Calendar units are collapsed to fixed lengths (a year is 365 days, a month is 30 days): a [time.Duration] cannot
// carry calendar context, so this conversion is inherently lossy and not calendar-correct.
// It matches the historical behaviour of ISO duration libraries in this space.
const (
	isoSecond = uint64(time.Second)
	isoMinute = uint64(time.Minute)
	isoHour   = uint64(time.Hour)
	isoDay    = 24 * isoHour
	isoWeek   = 7 * isoDay
	isoMonth  = 30 * isoDay
	isoYear   = 365 * isoDay

	// maxDurationMagnitude is 1<<63: the magnitude of [math.MinInt64], and one more than [math.MaxInt64].
	//
	// A parsed magnitude may reach exactly this value only for a negative duration (yielding [math.MinInt64]).
	maxDurationMagnitude = uint64(1) << 63
)

// Base-10 parsing / formatting constants (decimalBase lives in default.go).
const (
	// decimalAccMax bounds a uint64 accumulator so that acc*decimalBase + 9 cannot overflow.
	decimalAccMax = (math.MaxUint64 - (decimalBase - 1)) / decimalBase
	// nanoDigits is the number of fractional digits in one whole second (nanosecond precision).
	nanoDigits = 9
)

// ISODuration is an ISO 8601 / RFC 3339 duration (e.g. "P1Y2M3DT4H5M6S", "P2W").
//
// The type parameter P selects the parsing policy at compile time (see [ISODurationPolicy]).
// Every instantiation is a distinct type that round-trips through JSON, text, SQL and BSON.
// Use the [DurationISO8601] alias for the strict, spec-compliant default.
//
// Like [time.Duration], it stores a nanosecond count; the largest representable duration is approximately 290 years.
// Calendar units are collapsed to fixed lengths (year = 365 days, month = 30 days), which is lossy by nature.
//
// This generic type carries no swagger:strfmt annotation: go-swagger binds a concrete format name to the
// [DurationISO8601] alias, not to a generic declaration.
type ISODuration[P ISODurationPolicy] time.Duration

// DurationISO8601 is the strict, spec-compliant ISO 8601 duration: the JSON Schema draft 2020 "duration" format
// (RFC 3339 Appendix A).
//
// It binds to the explicit, unambiguous "duration-iso8601" handle. Plain "duration" is a context-dependent
// default resolved by the registry ([Default] → human, [JSONSchema2020Registry] → ISO), not by this static type.
//
// swagger:strfmt duration-iso8601.
type DurationISO8601 = ISODuration[DurationStrict]

// ParseISO8601Duration parses an ISO 8601 / RFC 3339 duration string.
//
// With no options it enforces the strict RFC 3339 Appendix A grammar.
// Options relax individual rules for programmatic callers.
func ParseISO8601Duration(s string, opts ...ISODurationOption) (time.Duration, error) {
	cfg := DurationStrict{}.isoDurationConfig()
	for _, o := range opts {
		o(&cfg)
	}
	return parseISO8601Duration(s, cfg)
}

// --- Format / encoding methods ---

// String renders the duration in canonical ISO 8601 form.
//
// Unlike [ISODuration.MarshalText], String is a best-effort display form: it always renders the true value (lossless),
// even under a strict policy that could not serialize it (e.g. a sign or sub-second precision).
func (d ISODuration[P]) String() string { return isoFormat(time.Duration(d)) }

// MarshalText implements [encoding.TextMarshaler], applying the policy P: a strict policy errors on a value it cannot
// represent (see [isoEmit]).
func (d ISODuration[P]) MarshalText() ([]byte, error) {
	var p P
	s, err := isoEmit(time.Duration(d), p.isoDurationConfig())
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler], applying the policy P.
func (d *ISODuration[P]) UnmarshalText(text []byte) error {
	var p P
	dd, err := parseISO8601Duration(string(text), p.isoDurationConfig())
	if err != nil {
		return err
	}
	*d = ISODuration[P](dd)
	return nil
}

// MarshalJSON returns the duration as a JSON string, applying the policy P.
func (d ISODuration[P]) MarshalJSON() ([]byte, error) {
	var p P
	s, err := isoEmit(time.Duration(d), p.isoDurationConfig())
	if err != nil {
		return nil, err
	}
	return json.Marshal(s)
}

// UnmarshalJSON sets the duration from a JSON string, applying the policy P.
func (d *ISODuration[P]) UnmarshalJSON(data []byte) error {
	if string(data) == jsonNull {
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	var p P
	dd, err := parseISO8601Duration(str, p.isoDurationConfig())
	if err != nil {
		return err
	}
	*d = ISODuration[P](dd)
	return nil
}

// Scan reads a duration (nanoseconds) from a database driver value.
func (d *ISODuration[P]) Scan(raw any) error {
	switch v := raw.(type) {
	case int64:
		*d = ISODuration[P](v)
	case float64:
		*d = ISODuration[P](int64(v))
	case nil:
		*d = ISODuration[P](0)
	default:
		return fmt.Errorf("cannot sql.Scan() strfmt.ISODuration from: %#v: %w", v, ErrFormat)
	}
	return nil
}

// Value writes the duration as a nanosecond count.
func (d ISODuration[P]) Value() (driver.Value, error) {
	return driver.Value(int64(d)), nil
}

// Equal reports whether two durations are equal.
func (d ISODuration[P]) Equal(other ISODuration[P]) bool { return d == other }

// DeepCopyInto copies the receiver into out.
func (d *ISODuration[P]) DeepCopyInto(out *ISODuration[P]) { *out = *d }

// DeepCopy copies the receiver into a new value.
func (d *ISODuration[P]) DeepCopy() *ISODuration[P] {
	if d == nil {
		return nil
	}
	out := new(ISODuration[P])
	d.DeepCopyInto(out)
	return out
}

// IsDurationISO8601 returns true if the string is a valid strict ISO 8601 duration.
func IsDurationISO8601(s string) bool {
	_, err := parseISO8601Duration(s, DurationStrict{}.isoDurationConfig())
	return err == nil
}

func isoDurationError(input, msg string) error {
	return fmt.Errorf("invalid ISO 8601 duration %q: %s: %w", input, msg, ErrFormat)
}

// section positions used to enforce component ordering (strictly increasing).
//
//nolint:gochecknoglobals,mnd // immutable ordinal lookup tables; the integers are sequence positions, not magic constants
var (
	isoDatePos = map[byte]int{'Y': 1, 'M': 2, 'W': 3, 'D': 4}
	isoTimePos = map[byte]int{'H': 1, 'M': 2, 'S': 3}
)

// isoDateSlot maps the year/month/day designators to contiguous slots for the strict anchoring (gap) check.
//
// 'W' is deliberately excluded: in strict mode it is exclusive, so it never coexists with Y/M/D.
func isoDateSlot(des byte) (int, bool) {
	switch des {
	case 'Y':
		return 0, true
	case 'M':
		return 1, true
	case 'D':
		return 2, true //nolint:mnd // contiguous slot index (Y=0, M=1, D=2), not a magic constant
	default:
		return 0, false
	}
}

//nolint:gocognit,gocyclo,cyclop // a single-pass grammar scanner; the branches mirror the ABNF.
func parseISO8601Duration(input string, cfg isoDurationConfig) (time.Duration, error) {
	s := input
	if cfg.allowSpace {
		s = strings.TrimSpace(s)
	}

	neg := false
	if cfg.allowSign && s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}

	if s == "" || s[0] != 'P' {
		return 0, isoDurationError(input, `must start with "P"`)
	}
	s = s[1:]
	if s == "" {
		return 0, isoDurationError(input, "no components after P")
	}

	var (
		total        uint64
		inTime       bool
		seenAny      bool
		weekSeen     bool
		fractionUsed bool
		lastDatePos  int
		lastTimePos  int
		datePresent  [3]bool // Y, M, D — for the anchoring (gap) check
		timePresent  [3]bool // H, M, S
	)

	for s != "" {
		c := s[0]

		if c == 'T' {
			if inTime {
				return 0, isoDurationError(input, `duplicate "T" separator`)
			}
			if weekSeen && !cfg.weekCombinable {
				return 0, isoDurationError(input, `"W" cannot be combined with other components`)
			}
			inTime = true
			s = s[1:]
			if s == "" {
				return 0, isoDurationError(input, `no components after "T"`)
			}
			continue
		}

		// A fraction, if any, must be on the least significant component: nothing may follow it.
		if fractionUsed {
			return 0, isoDurationError(input, "a fraction is only allowed on the least significant component")
		}

		// Scan the integer part (ASCII digits only).
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 {
			return 0, isoDurationError(input, fmt.Sprintf("expected a digit, got %q", s[0]))
		}
		intPart := s[:i]

		// Optional decimal fraction.
		var fracPart string
		hasFrac := false
		if i < len(s) && (s[i] == '.' || s[i] == ',') {
			if !cfg.allowFraction {
				return 0, isoDurationError(input, "decimal fraction is not allowed")
			}
			hasFrac = true
			j := i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			fracPart = s[i+1 : j]
			i = j
		}

		if i >= len(s) {
			return 0, isoDurationError(input, "value without a unit designator")
		}
		des := s[i]
		s = s[i+1:]

		unit, err := isoResolveUnit(input, des, inTime, cfg,
			&lastDatePos, &lastTimePos, &weekSeen, seenAny, &datePresent, &timePresent)
		if err != nil {
			return 0, err
		}
		seenAny = true
		if hasFrac {
			fractionUsed = true
		}

		val, err := isoScaleValue(input, intPart, fracPart, unit)
		if err != nil {
			return 0, err
		}

		// Accumulate with the stdlib overflow discipline: the magnitude may reach 1<<63 only for a negative duration.
		if total > maxDurationMagnitude-val {
			return 0, isoDurationError(input, "value out of range")
		}
		total += val
	}

	if inTime && !timePresent[0] && !timePresent[1] && !timePresent[2] {
		return 0, isoDurationError(input, `no components after "T"`)
	}
	if !seenAny {
		return 0, isoDurationError(input, "no components")
	}

	// Strict anchoring: the present Y/M/D and H/M/S components must each form a contiguous run (e.g. P1Y2D and PT1H2S are
	// rejected).
	if !cfg.relaxAnchoring {
		if err := isoCheckContiguous(input, datePresent, "date"); err != nil {
			return 0, err
		}
		if err := isoCheckContiguous(input, timePresent, "time"); err != nil {
			return 0, err
		}
	}

	if total > maxDurationMagnitude-1 {
		if neg && total == maxDurationMagnitude {
			return math.MinInt64, nil
		}
		return 0, isoDurationError(input, "value out of range")
	}
	if neg {
		return -time.Duration(total), nil
	}
	return time.Duration(total), nil
}

// isoResolveUnit validates a designator in context (section, ordering, week
// exclusivity) and returns its unit length in nanoseconds.
func isoResolveUnit(
	input string, des byte, inTime bool, cfg isoDurationConfig,
	lastDatePos, lastTimePos *int, weekSeen *bool, seenAny bool,
	datePresent, timePresent *[3]bool,
) (uint64, error) {
	if inTime {
		pos, ok := isoTimePos[des]
		if !ok {
			return 0, isoDurationError(input, fmt.Sprintf("%q is not a valid time-section designator", des))
		}
		if pos <= *lastTimePos {
			return 0, isoDurationError(input, fmt.Sprintf("designator %q is out of order", des))
		}
		*lastTimePos = pos
		timePresent[pos-1] = true
		switch des {
		case 'H':
			return isoHour, nil
		case 'M':
			return isoMinute, nil
		default: // 'S'
			return isoSecond, nil
		}
	}

	pos, ok := isoDatePos[des]
	if !ok {
		return 0, isoDurationError(input, fmt.Sprintf("%q is not a valid date-section designator", des))
	}
	if pos <= *lastDatePos {
		return 0, isoDurationError(input, fmt.Sprintf("designator %q is out of order", des))
	}

	if des == 'W' {
		if !cfg.weekCombinable && seenAny {
			return 0, isoDurationError(input, `"W" cannot be combined with other components`)
		}
		*lastDatePos = pos
		*weekSeen = true
		return isoWeek, nil
	}

	if *weekSeen && !cfg.weekCombinable {
		return 0, isoDurationError(input, `"W" cannot be combined with other components`)
	}
	*lastDatePos = pos
	if slot, ok := isoDateSlot(des); ok {
		datePresent[slot] = true
	}
	switch des {
	case 'Y':
		return isoYear, nil
	case 'M':
		return isoMonth, nil
	default: // 'D'
		return isoDay, nil
	}
}

// isoCheckContiguous rejects gaps in a section's component chain.
func isoCheckContiguous(input string, present [3]bool, section string) error {
	first, last := -1, -1
	for i, p := range present {
		if p {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return nil
	}
	for i := first; i <= last; i++ {
		if !present[i] {
			return isoDurationError(input, "non-contiguous "+section+" components (a gap in the unit chain)")
		}
	}
	return nil
}

// isoScaleValue converts an integer (and optional fractional) component to nanoseconds, checking every overflow
// boundary.
func isoScaleValue(input, intPart, fracPart string, unit uint64) (uint64, error) {
	v, ok := isoParseUint(intPart)
	if !ok {
		return 0, isoDurationError(input, "value out of range")
	}
	// Bound v*unit to the duration magnitude *before* multiplying, so the later fractional addition cannot wrap a uint64.
	if v > maxDurationMagnitude/unit {
		return 0, isoDurationError(input, "value out of range")
	}
	val := v * unit

	if fracPart != "" {
		f, scale := isoParseFraction(fracPart)
		if f > 0 {
			// float64 is accurate enough for a sub-unit fraction (matches the standard library technique); the magnitude stays
			// below the uint64 ceiling because val <= 1<<63 and the addend is < unit.
			val += uint64(float64(f) * (float64(unit) / float64(scale)))
			if val > maxDurationMagnitude {
				return 0, isoDurationError(input, "value out of range")
			}
		}
	}

	return val, nil
}

// isoParseUint parses digits into a uint64, reporting overflow.
func isoParseUint(s string) (uint64, bool) {
	var v uint64
	for i := range len(s) {
		if v > decimalAccMax {
			return 0, false
		}
		v = v*decimalBase + uint64(s[i]-'0')
	}

	return v, true
}

// isoParseFraction parses fractional digits into (value, scale=10^len), capping precision at 18 digits so the value
// stays exact in the subsequent float maths.
func isoParseFraction(s string) (uint64, uint64) {
	const maxFracDigits = 18
	if len(s) > maxFracDigits {
		s = s[:maxFracDigits]
	}
	var f, scale uint64 = 0, 1
	for i := range len(s) {
		f = f*decimalBase + uint64(s[i]-'0')
		scale *= decimalBase
	}

	return f, scale
}

// isoFormat renders a duration in canonical ISO 8601 form (P[n]DT[n]H[n]M[n]S).
//
// Intermediate zero time components are emitted when needed to keep the output contiguous (e.g. 1h5s → "PT1H0M5S"),
// so the structure is always syntactically valid ISO 8601. Years, months and weeks are not reconstructed (a
// [time.Duration] does not carry calendar context); a fractional second is emitted losslessly, and a negative duration
// is prefixed with "-".
func isoFormat(d time.Duration) string {
	if d == 0 {
		return "PT0S"
	}
	neg := d < 0
	var u uint64
	if neg {
		u = uint64(-d)
	} else {
		u = uint64(d)
	}

	days := u / isoDay
	u %= isoDay
	hours := u / isoHour
	u %= isoHour
	minutes := u / isoMinute
	u %= isoMinute
	seconds := u / isoSecond
	frac := u % isoSecond

	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}

	b.WriteByte('P')
	if days > 0 {
		b.WriteString(strconv.FormatUint(days, decimalBase))
		b.WriteByte('D')
	}
	isoWriteTimeSection(&b, hours, minutes, seconds, frac)

	return b.String()
}

// isoWriteTimeSection appends the "T…"  part of a canonical ISO 8601 duration, emitting a zero-minute filler when it
// must bridge hours and seconds so the output stays contiguous (e.g. 1h5s → "T1H0M5S").
func isoWriteTimeSection(b *strings.Builder, hours, minutes, seconds, frac uint64) {
	hasH := hours > 0
	hasM := minutes > 0
	hasS := seconds > 0 || frac > 0
	if !hasH && !hasM && !hasS {
		return
	}

	b.WriteByte('T')
	if hasH {
		b.WriteString(strconv.FormatUint(hours, decimalBase))
		b.WriteByte('H')
	}

	// Emit minutes if non-zero, or as a zero filler bridging hours and seconds.
	if hasM || (hasH && hasS) {
		b.WriteString(strconv.FormatUint(minutes, decimalBase))
		b.WriteByte('M')
	}

	if hasS {
		b.WriteString(strconv.FormatUint(seconds, decimalBase))
		if frac > 0 {
			b.WriteByte('.')
			b.WriteString(isoFormatFraction(frac))
		}
		b.WriteByte('S')
	}
}

// isoEmit renders d under the given policy.
//
// A policy that forbids a feature the value requires (a sign, or sub-second precision) cannot represent it, and returns
// an error rather than emitting output a matching parser would reject.
func isoEmit(d time.Duration, cfg isoDurationConfig) (string, error) {
	if !cfg.allowSign && d < 0 {
		return "", fmt.Errorf("a negative duration cannot be represented under this ISO 8601 policy: %w", ErrFormat)
	}

	if !cfg.allowFraction && d%time.Second != 0 {
		return "", fmt.Errorf("sub-second precision cannot be represented under this ISO 8601 policy: %w", ErrFormat)
	}

	return isoFormat(d), nil
}

// isoFormatFraction renders a nanosecond remainder (0..1e9) as its 9-digit fractional part with trailing zeros trimmed.
func isoFormatFraction(frac uint64) string {
	s := strconv.FormatUint(frac, decimalBase)
	if len(s) < nanoDigits {
		s = strings.Repeat("0", nanoDigits-len(s)) + s
	}
	return strings.TrimRight(s, "0")
}
