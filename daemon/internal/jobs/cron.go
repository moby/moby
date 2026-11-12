package jobs

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	// Schedules name IANA timezones and must evaluate the same on every
	// host, including Windows and minimal images without zoneinfo. The
	// system database still takes precedence where it exists.
	_ "time/tzdata"
)

// cronSchedule is a parsed five-field crontab expression. Each set is a
// bitmask of the accepted values for its field.
type cronSchedule struct {
	minutes uint64 // 0-59
	hours   uint64 // 0-23
	dom     uint64 // 1-31, day of month
	months  uint64 // 1-12
	dow     uint64 // 0-6, day of week, Sunday is 0

	// The day-of-month and day-of-week fields follow the standard crontab
	// quirk: when both are restricted (neither is *), a day matching either
	// fires; otherwise the restricted one must match.
	domRestricted, dowRestricted bool

	// wildcard marks a schedule whose minute or hour field starts with *,
	// Vixie cron's classification for time-change handling: a wildcard
	// schedule follows the wall clock through a transition (firing in both
	// passes of a repeated span, never catching up a skipped time), while a
	// fixed-time schedule fires exactly once per nominal occurrence.
	wildcard bool
}

// nextCronBoundYears caps the search for the next occurrence. The Gregorian
// calendar, weekdays included, repeats every 400 years, so an expression
// with no occurrence in that span never fires, as with impossible dates
// such as "0 0 30 2 *"; sparse ones such as "0 0 29 2 */7" (February 29 on
// a Sunday) still find occurrences decades apart. The scan jumps over
// unmatched months and days, so even a full-span search stays in the
// milliseconds.
const nextCronBoundYears = 400

// parseCron parses a five-field crontab expression: minute, hour,
// day-of-month, month, day-of-week. Each field accepts numbers, lists
// (a,b,c), ranges (a-b) and steps (*/n or a-b/n).
//
// The wire format is deliberately strict: month and weekday names, @daily
// style shortcuts, and Quartz extensions (L, #, W) are rejected. Clients
// expand shortcuts before submitting, so the daemon-side grammar can stay
// small and unambiguous.
func parseCron(expr string) (*cronSchedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields (minute hour day-of-month month day-of-week), got %d", len(fields))
	}
	var (
		schedule cronSchedule
		err      error
	)
	if schedule.minutes, err = parseCronField(fields[0], 0, 59); err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	if schedule.hours, err = parseCronField(fields[1], 0, 23); err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	if schedule.dom, err = parseCronField(fields[2], 1, 31); err != nil {
		return nil, fmt.Errorf("day-of-month field: %w", err)
	}
	if schedule.months, err = parseCronField(fields[3], 1, 12); err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	if schedule.dow, err = parseCronField(fields[4], 0, 7); err != nil {
		return nil, fmt.Errorf("day-of-week field: %w", err)
	}
	// Both 0 and 7 mean Sunday; fold 7 onto 0 so matching only checks 0-6.
	if schedule.dow&(1<<7) != 0 {
		schedule.dow = (schedule.dow &^ (1 << 7)) | 1
	}
	// Vixie's first-character rule: a field is "restricted" only when it
	// does not start with *, so */2 counts as unrestricted for the AND/OR
	// day-combination choice while its mask still applies.
	schedule.domRestricted = !strings.HasPrefix(fields[2], "*")
	schedule.dowRestricted = !strings.HasPrefix(fields[4], "*")
	schedule.wildcard = strings.HasPrefix(fields[0], "*") || strings.HasPrefix(fields[1], "*")
	return &schedule, nil
}

// parseCronField parses one field into a bitmask of accepted values.
func parseCronField(field string, minValue, maxValue int) (uint64, error) {
	var mask uint64
	for part := range strings.SplitSeq(field, ",") {
		rangeExpr, stepExpr, hasStep := strings.Cut(part, "/")
		step := 1
		if hasStep {
			// Steps only make sense over a span; a step on a single value
			// ("3/5") is a Vixie laxity this grammar deliberately rejects.
			if rangeExpr != "*" && !strings.Contains(rangeExpr, "-") {
				return 0, fmt.Errorf("invalid term %q: a step requires * or a range", part)
			}
			// Atoi tolerates a leading sign; the grammar is digits only.
			if stepExpr == "" || stepExpr[0] < '0' || stepExpr[0] > '9' {
				return 0, fmt.Errorf("invalid step in term %q", part)
			}
			parsed, err := strconv.Atoi(stepExpr)
			if err != nil || parsed <= 0 {
				return 0, fmt.Errorf("invalid step in term %q", part)
			}
			step = parsed
		}

		lo, hi := minValue, maxValue
		if rangeExpr != "*" {
			loExpr, hiExpr, isRange := strings.Cut(rangeExpr, "-")
			var err error
			if lo, err = parseCronValue(loExpr, minValue, maxValue); err != nil {
				return 0, fmt.Errorf("invalid term %q: %w", part, err)
			}
			hi = lo
			if isRange {
				if hi, err = parseCronValue(hiExpr, minValue, maxValue); err != nil {
					return 0, fmt.Errorf("invalid term %q: %w", part, err)
				}
				if hi < lo {
					return 0, fmt.Errorf("invalid term %q: range is reversed", part)
				}
			}
		}
		for v := lo; v <= hi; v += step {
			mask |= 1 << v
			// Check the remaining span before adding an untrusted step,
			// which may be large enough to overflow an int.
			if step > hi-v {
				break
			}
		}
	}
	if mask == 0 {
		return 0, errors.New("field is empty")
	}
	return mask, nil
}

// parseCronValue parses one numeric field value and checks its bounds.
func parseCronValue(expr string, minValue, maxValue int) (int, error) {
	// Atoi tolerates a leading sign; the grammar is digits only.
	if expr == "" || expr[0] < '0' || expr[0] > '9' {
		return 0, fmt.Errorf("%q is not a number (names and shortcuts are not supported)", expr)
	}
	value, err := strconv.Atoi(expr)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number (names and shortcuts are not supported)", expr)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("value %d is outside %d-%d", value, minValue, maxValue)
	}
	return value, nil
}

// next returns the first occurrence strictly after the given time, evaluated
// on the wall clock of loc, and false when no occurrence exists within the
// search bound.
//
// Daylight-saving transitions follow crontab(5) semantics. A fixed-time
// schedule (minute and hour fields both restricted) fires exactly once per
// nominal occurrence: one falling in the skipped span of a spring-forward
// day fires immediately after the transition (occurrences collapsing onto
// the same instant fire once), and one in the repeated span of a fall-back
// day fires on the first pass only. A wildcard schedule (minute or hour
// field starting with *) follows the wall clock instead: times that never
// happen are not made up, and times that happen twice fire twice.
func (s *cronSchedule) next(after time.Time, loc *time.Location) (time.Time, bool) {
	t := minuteStep(after.In(loc))

	bound := after.AddDate(nextCronBoundYears, 0, 0)
	for t.Before(bound) {
		if !s.wildcard && s.skippedOccurrence(t) {
			return t, true
		}
		if s.months&(1<<uint(t.Month())) == 0 {
			// Jump to the first minute of the next month.
			t = jumpForward(t, time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc))
			continue
		}
		if !s.dayMatches(t) {
			t = jumpForward(t, time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc))
			continue
		}
		if s.hours&(1<<uint(t.Hour())) == 0 {
			t = hourJump(t)
			continue
		}
		if s.minutes&(1<<uint(t.Minute())) == 0 {
			t = minuteStep(t)
			continue
		}
		// The first pass through a fall-back day's repeated hour already
		// matched this wall-clock time, so a fixed-time schedule fires only
		// once; a wildcard schedule follows the wall clock and fires again.
		if !s.wildcard {
			if end, repeated := repeatedWallClock(t); repeated {
				t = end
				continue
			}
		}
		return t, true
	}
	return time.Time{}, false
}

// minuteStep advances t to the next whole minute, stopping at a zone
// transition that comes first. Almost every transition sits on a whole
// minute, but offsets predating 1972 can hold seconds — Africa/Monrovia left
// -00:44:30 that year — and a step over such a transition would miss the
// instant skippedOccurrence inspects.
func minuteStep(t time.Time) time.Time {
	next := t.Truncate(time.Minute).Add(time.Minute)
	if _, end := t.ZoneBounds(); end.After(t) && end.Before(next) {
		return end
	}
	return next
}

// hourJump advances t to the next hour boundary in absolute time rather than
// by rebuilding wall-clock components: across a fall-back transition the
// rebuilt time is ambiguous and Go may resolve it to the second occurrence,
// skipping the first one; the absolute walk visits both in real order and the
// fields are re-checked on every loop anyway.
//
// The jump is capped at the zone transition when one comes first: a
// transition inside the hour moves the wall clock, so a full hour in absolute
// time can overshoot wall-clock times that exist. In Pacific/Chatham the
// clock jumps from 02:45 to 03:45, and an hour from 02:00 lands on 04:00,
// past the day's only 03:45-03:59 span.
func hourJump(t time.Time) time.Time {
	next := t.Add(time.Duration(60-t.Minute()) * time.Minute)
	if _, end := t.ZoneBounds(); end.After(t) && end.Before(next) {
		return end
	}
	return next
}

// jumpForward returns target, or the zone transition before it when one comes
// first, or the minute after t when target is not after it.
//
// The cap at the transition mirrors hourJump's: a backward transition can
// re-enter the very dates the jump is skipping — America/St_Johns used to
// fall back at 00:01, returning the wall clock to the previous day — so the
// day and month masks must be rechecked where the offset changes.
//
// Where a transition skips midnight, as in America/Havana, time.Date
// normalizes a day's midnight back into the previous day, so jumping to it
// would repeat forever; walking minutes reaches the transition instead.
//
// Beyond a zone's transition table Go evaluates its TZif rule with
// year-granularity segment bounds, which near a leap year's end can sit at or
// before t itself; both caps take only a bound strictly ahead of t, or the
// walk would stop making progress.
func jumpForward(t, target time.Time) time.Time {
	if _, end := t.ZoneBounds(); end.After(t) && end.Before(target) {
		return end
	}
	if target.After(t) {
		return target
	}

	return t.Add(time.Minute)
}

// repeatedWallClock reports whether t's wall-clock time already happened
// earlier, because a transition just set the clock back, and returns when
// the repeated span ends.
func repeatedWallClock(t time.Time) (end time.Time, repeated bool) {
	start, _ := t.ZoneBounds()
	if start.IsZero() {
		return time.Time{}, false
	}

	_, offset := t.Zone()
	_, before := start.Add(-time.Second).Zone()

	setBack := time.Duration(before-offset) * time.Second
	if setBack <= 0 || t.Sub(start) >= setBack {
		return time.Time{}, false
	}

	return start.Add(setBack), true
}

// skippedOccurrence reports whether t is the first instant after a
// spring-forward transition whose skipped wall-clock span contains an
// occurrence of s: crontab(5) runs a fixed-time job whose time was skipped
// immediately after the transition. As there, only time changes under three
// hours get this treatment; a larger jump is a clock correction, not a
// daylight-saving change, and is not caught up.
//
// The walk lands exactly on every gap transition: every advance — the minute
// steps as much as the hour, day and month jumps — is capped at the zone
// transition it would otherwise cross, so checking the instants where a new
// zone begins covers every gap.
func (s *cronSchedule) skippedOccurrence(t time.Time) bool {
	start, _ := t.ZoneBounds()
	if !start.Equal(t) {
		return false
	}

	_, offset := t.Zone()
	_, before := t.Add(-time.Second).Zone()

	skipped := time.Duration(offset-before) * time.Second
	if skipped <= 0 || skipped >= 3*time.Hour {
		return false
	}

	// Walk the skipped wall-clock minutes in pure calendar space: they have
	// no time.Time in loc, by definition. The transition's seconds take part
	// in the bounds: half a minute off, the walk would reach back into the
	// last minute lived before the transition and skip the first one lost
	// to it.
	end := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	for w := end.Add(-skipped); w.Before(end); w = w.Add(time.Minute) {
		if s.matchesWall(w) {
			return true
		}
	}
	return false
}

// matchesWall reports whether a calendar time's components match every field
// mask.
func (s *cronSchedule) matchesWall(w time.Time) bool {
	return s.months&(1<<uint(w.Month())) != 0 &&
		s.dayMatches(w) &&
		s.hours&(1<<uint(w.Hour())) != 0 &&
		s.minutes&(1<<uint(w.Minute())) != 0
}

// dayMatches applies Vixie cron's day-combination rule: both masks always
// apply, combined with OR only when both fields are restricted ("1,15 * Sun"
// means the 1st, the 15th AND Sundays) and with AND otherwise (an
// unrestricted field's mask is full, so it never excludes anything).
func (s *cronSchedule) dayMatches(t time.Time) bool {
	domOK := s.dom&(1<<uint(t.Day())) != 0
	dowOK := s.dow&(1<<uint(t.Weekday())) != 0
	if s.domRestricted && s.dowRestricted {
		return domOK || dowOK
	}
	return domOK && dowOK
}
