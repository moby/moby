package jobs

import (
	"fmt"
	"math"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseCron(t *testing.T) {
	for _, tc := range []struct {
		doc       string
		expr      string
		expectErr string
	}{
		{doc: "every minute", expr: "* * * * *"},
		{doc: "daily at 3am", expr: "0 3 * * *"},
		{doc: "lists ranges steps", expr: "*/15 0-6 1,15 */2 1-5"},
		{doc: "sunday as 7", expr: "0 12 * * 7"},
		{doc: "range with step", expr: "10-50/10 * * * *"},

		{doc: "too few fields", expr: "* * * *", expectErr: "must have 5 fields"},
		{doc: "too many fields", expr: "* * * * * *", expectErr: "must have 5 fields"},
		{doc: "not a number", expr: "a * * * *", expectErr: "not a number"},
		{doc: "minute out of range", expr: "60 * * * *", expectErr: "outside 0-59"},
		{doc: "hour out of range", expr: "* 24 * * *", expectErr: "outside 0-23"},
		{doc: "day-of-month zero", expr: "* * 0 * *", expectErr: "outside 1-31"},
		{doc: "month out of range", expr: "* * * 13 *", expectErr: "outside 1-12"},
		{doc: "weekday out of range", expr: "* * * * 8", expectErr: "outside 0-7"},
		{doc: "reversed range", expr: "30-10 * * * *", expectErr: "range is reversed"},
		{doc: "zero step", expr: "*/0 * * * *", expectErr: "invalid step"},
		{doc: "step on single value", expr: "3/5 * * * *", expectErr: "a step requires * or a range"},
		{doc: "month name", expr: "* * * jan *", expectErr: "not a number"},
		{doc: "weekday name", expr: "* * * * mon", expectErr: "not a number"},
		{doc: "shortcut", expr: "@daily", expectErr: "must have 5 fields"},
		{doc: "double range", expr: "1-2-3 * * * *", expectErr: "not a number"},
		{doc: "empty step", expr: "*/ * * * *", expectErr: "invalid step"},
		{doc: "step without range", expr: "/5 * * * *", expectErr: "a step requires * or a range"},
		{doc: "open-ended range", expr: "5- * * * *", expectErr: "not a number"},
		{doc: "open-started range", expr: "-5 * * * *", expectErr: "not a number"},
		{doc: "empty list entry", expr: "1,,2 * * * *", expectErr: "not a number"},
		{doc: "leading sign", expr: "+5 * * * *", expectErr: "not a number"},
		{doc: "leading sign in step", expr: "*/+5 * * * *", expectErr: "invalid step"},
	} {
		t.Run(tc.doc, func(t *testing.T) {
			_, err := parseCron(tc.expr)
			if tc.expectErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Check(t, is.ErrorContains(err, tc.expectErr))
			}
		})
	}
}

func TestParseCronLargeStep(t *testing.T) {
	t.Parallel()

	schedule, err := parseCron(fmt.Sprintf("0 0 */%d * *", math.MaxInt))
	assert.NilError(t, err)
	assert.Equal(t, schedule.dom, uint64(1<<1))
}

func TestCronNext(t *testing.T) {
	utc := func(y int, mo time.Month, d, h, mi int) time.Time {
		return time.Date(y, mo, d, h, mi, 0, 0, time.UTC)
	}
	for _, tc := range []struct {
		doc   string
		expr  string
		after time.Time
		want  time.Time
		never bool
	}{
		{
			doc:  "daily before the hour",
			expr: "0 3 * * *", after: utc(2026, time.January, 15, 2, 0),
			want: utc(2026, time.January, 15, 3, 0),
		},
		{
			doc:  "strictly after an exact hit",
			expr: "0 3 * * *", after: utc(2026, time.January, 15, 3, 0),
			want: utc(2026, time.January, 16, 3, 0),
		},
		{
			doc:  "seconds are truncated away",
			expr: "0 3 * * *", after: utc(2026, time.January, 15, 2, 59).Add(30 * time.Second),
			want: utc(2026, time.January, 15, 3, 0),
		},
		{
			doc:  "quarter-hour step",
			expr: "*/15 * * * *", after: utc(2026, time.January, 15, 10, 7),
			want: utc(2026, time.January, 15, 10, 15),
		},
		{
			doc:  "step wraps to next hour",
			expr: "*/15 * * * *", after: utc(2026, time.January, 15, 10, 45),
			want: utc(2026, time.January, 15, 11, 0),
		},
		{
			doc:  "yearly",
			expr: "0 0 1 1 *", after: utc(2026, time.June, 1, 0, 0),
			want: utc(2027, time.January, 1, 0, 0),
		},
		{
			doc:  "weekday match",
			expr: "0 12 * * 0", after: utc(2026, time.January, 15, 0, 0), // a Thursday
			want: utc(2026, time.January, 18, 12, 0), // the next Sunday
		},
		{
			doc:  "sunday spelled as 7",
			expr: "0 12 * * 7", after: utc(2026, time.January, 15, 0, 0),
			want: utc(2026, time.January, 18, 12, 0),
		},
		{
			doc: "restricted day-of-month and day-of-week combine as OR",
			// The next Friday (March 6th) comes before the next 13th.
			expr: "0 0 13 * 5", after: utc(2026, time.March, 1, 0, 0),
			want: utc(2026, time.March, 6, 0, 0),
		},
		{
			doc: "star-step day-of-month does not defeat the weekday",
			// Vixie's first-character rule: */1 counts as unrestricted, so
			// the weekday alone constrains the day.
			expr: "0 3 */1 * 1", after: utc(2026, time.January, 15, 0, 0), // a Thursday
			want: utc(2026, time.January, 19, 3, 0), // the next Monday
		},
		{
			doc: "star-step day-of-month mask still applies",
			// January 16th is even: the next odd day is the 17th.
			expr: "0 3 */2 * *", after: utc(2026, time.January, 15, 4, 0),
			want: utc(2026, time.January, 17, 3, 0),
		},
		{
			doc:  "february 29th waits for a leap year",
			expr: "0 0 29 2 *", after: utc(2026, time.January, 1, 0, 0),
			want: utc(2028, time.February, 29, 0, 0),
		},
		{
			doc: "sparse weekday-bound february 29th",
			// February 29th on a Sunday: after 2032 the next is 2060.
			expr: "0 0 29 2 */7", after: utc(2032, time.March, 1, 0, 0),
			want: utc(2060, time.February, 29, 0, 0),
		},
		{
			doc:  "impossible date never fires",
			expr: "0 0 30 2 *", after: utc(2026, time.January, 1, 0, 0),
			never: true,
		},
	} {
		t.Run(tc.doc, func(t *testing.T) {
			schedule, err := parseCron(tc.expr)
			assert.NilError(t, err)
			got, ok := schedule.next(tc.after, time.UTC)
			if tc.never {
				assert.Check(t, !ok)
				return
			}
			assert.Assert(t, ok)
			assert.Check(t, is.Equal(got.String(), tc.want.String()))
		})
	}
}

func TestParseCronWildcard(t *testing.T) {
	// The time-change classification follows Vixie's first-character rule on
	// the minute and hour fields, not mask fullness.
	for expr, wild := range map[string]bool{
		"30 2 * * *":   false,
		"*/5 2 * * *":  true,  // step on a wildcard minute field
		"30 */2 * * *": true,  // step on a wildcard hour field
		"5,* 2 * * *":  false, // a list is fixed-time even when it contains *
		"30 2 */2 * 1": false, // day fields never affect the classification
	} {
		schedule, err := parseCron(expr)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(schedule.wildcard, wild), expr)
	}
}

func TestCronNextDST(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	assert.NilError(t, err)
	chatham, err := time.LoadLocation("Pacific/Chatham")
	assert.NilError(t, err)
	havana, err := time.LoadLocation("America/Havana")
	assert.NilError(t, err)
	stJohns, err := time.LoadLocation("America/St_Johns")
	assert.NilError(t, err)
	monrovia, err := time.LoadLocation("Africa/Monrovia")
	assert.NilError(t, err)
	schedule, err := parseCron("30 2 * * *")
	assert.NilError(t, err)

	t.Run("spring forward fires a skipped fixed-time occurrence at the transition", func(t *testing.T) {
		// On 2026-03-29 the Paris wall clock jumps from 02:00 to 03:00;
		// 02:30 never happens that day, so the occurrence fires right
		// after the transition, at 03:00 CEST (01:00 UTC).
		after := time.Date(2026, time.March, 29, 0, 0, 0, 0, paris)
		got, ok := schedule.next(after, paris)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.UTC().String(), time.Date(2026, time.March, 29, 1, 0, 0, 0, time.UTC).String()))

		// The catch-up replaces the occurrence: the next one is the
		// following day's.
		got, ok = schedule.next(got, paris)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.String(), time.Date(2026, time.March, 30, 2, 30, 0, 0, paris).String()))
	})

	t.Run("spring forward collapses occurrences onto the transition", func(t *testing.T) {
		// 02:00 is skipped and 03:00 is the transition instant itself:
		// both nominal occurrences land on the same instant and fire once.
		sched, err := parseCron("0 2,3 * * *")
		assert.NilError(t, err)

		after := time.Date(2026, time.March, 29, 0, 0, 0, 0, paris)
		got, ok := sched.next(after, paris)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.UTC().String(), time.Date(2026, time.March, 29, 1, 0, 0, 0, time.UTC).String()))

		got, ok = sched.next(got, paris)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.String(), time.Date(2026, time.March, 30, 2, 0, 0, 0, paris).String()))
	})

	t.Run("spring forward catch-up on a transition inside the hour", func(t *testing.T) {
		// Chatham jumps from 02:45 to 03:45: a 03:00 occurrence falls in
		// the gap and fires at 03:45, the first instant after it.
		sched, err := parseCron("0 3 * * *")
		assert.NilError(t, err)

		after := time.Date(2026, time.September, 27, 0, 0, 0, 0, chatham)
		got, ok := sched.next(after, chatham)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.String(), time.Date(2026, time.September, 27, 3, 45, 0, 0, chatham).String()))
	})

	t.Run("spring forward catch-up on a transition skipping midnight", func(t *testing.T) {
		// Havana skips 00:00-00:59 on 2026-03-08: a 00:30 occurrence fires
		// at 01:00, reached by the day search walking through the
		// normalized midnight.
		sched, err := parseCron("30 0 * * *")
		assert.NilError(t, err)

		after := time.Date(2026, time.March, 7, 12, 0, 0, 0, havana)
		got, ok := sched.next(after, havana)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.String(), time.Date(2026, time.March, 8, 1, 0, 0, 0, havana).String()))
	})

	t.Run("spring forward does not catch up a wildcard schedule", func(t *testing.T) {
		// A wildcard schedule (minute or hour field starting with *)
		// follows the wall clock: the nonexistent 02:30 is not made up,
		// the next occurrence is 03:30 CEST.
		sched, err := parseCron("30 * * * *")
		assert.NilError(t, err)

		after := time.Date(2026, time.March, 29, 0, 45, 0, 0, time.UTC) // 01:45 CET
		got, ok := sched.next(after, paris)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.UTC().String(), time.Date(2026, time.March, 29, 1, 30, 0, 0, time.UTC).String()))
	})

	t.Run("fall back fires a wildcard schedule in both passes", func(t *testing.T) {
		// On 2026-10-25 Paris repeats the 02:00-02:59 hour. An
		// interval-style schedule keeps its real-time cadence through both
		// passes instead of skipping the repeated one.
		sched, err := parseCron("0,30 * * * *")
		assert.NilError(t, err)

		cur := time.Date(2026, time.October, 25, 0, 45, 0, 0, time.UTC) // 02:45 CEST, first pass
		for _, want := range []time.Time{
			time.Date(2026, time.October, 25, 1, 0, 0, 0, time.UTC),  // 02:00 CET, second pass
			time.Date(2026, time.October, 25, 1, 30, 0, 0, time.UTC), // 02:30 CET, second pass
			time.Date(2026, time.October, 25, 2, 0, 0, 0, time.UTC),  // 03:00 CET
		} {
			got, ok := sched.next(cur, paris)
			assert.Assert(t, ok)
			assert.Check(t, is.Equal(got.UTC().String(), want.String()))
			cur = got
		}
	})

	t.Run("fall back fires a step wildcard in both passes", func(t *testing.T) {
		// Vixie's first-character rule: */1 is a wildcard field even though
		// its mask is identical to a fixed list of every hour, so the
		// schedule follows the wall clock through the repeated hour.
		sched, err := parseCron("30 */1 * * *")
		assert.NilError(t, err)

		after := time.Date(2026, time.October, 25, 0, 45, 0, 0, time.UTC) // 02:45 CEST, first pass
		got, ok := sched.next(after, paris)
		assert.Assert(t, ok)
		// 02:30 CET, the second pass of the repeated hour.
		assert.Check(t, is.Equal(got.UTC().String(), time.Date(2026, time.October, 25, 1, 30, 0, 0, time.UTC).String()))
	})

	t.Run("spring forward catch-up reached by minute steps", func(t *testing.T) {
		// The hour mask also matches the hour before the gap, so the walk
		// arrives at the transition by minute steps rather than an hour
		// jump.
		sched, err := parseCron("15 1,2 * * *")
		assert.NilError(t, err)

		after := time.Date(2026, time.March, 29, 0, 15, 0, 0, time.UTC) // 01:15 CET, just fired
		got, ok := sched.next(after, paris)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.UTC().String(), time.Date(2026, time.March, 29, 1, 0, 0, 0, time.UTC).String()))
	})

	t.Run("fall back fires once in the repeated hour", func(t *testing.T) {
		// On 2026-10-25 the Paris wall clock falls from 03:00 back to
		// 02:00: 02:30 happens twice (CEST then CET). The occurrence fires
		// on the first one only.
		after := time.Date(2026, time.October, 25, 0, 0, 0, 0, paris)
		first, ok := schedule.next(after, paris)
		assert.Assert(t, ok)
		// The first 02:30 is the CEST one, i.e. 00:30 UTC.
		assert.Check(t, is.Equal(first.UTC().String(), time.Date(2026, time.October, 25, 0, 30, 0, 0, time.UTC).String()))

		// The occurrence after it is the next day's, not the repeated
		// (CET, 01:30 UTC) 02:30 of the same day.
		second, ok := schedule.next(first, paris)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(second.String(), time.Date(2026, time.October, 26, 2, 30, 0, 0, paris).String()))
	})

	t.Run("fall back does not repeat the hour after a fire on its last minute", func(t *testing.T) {
		// The occurrence after 02:59 CEST is a minute later in real time,
		// at 02:00 CET, but that wall-clock time already fired an hour ago.
		schedule, err := parseCron("0,59 2 * * *")
		assert.NilError(t, err)

		lastMinute := time.Date(2026, time.October, 25, 0, 59, 0, 0, time.UTC)
		got, ok := schedule.next(lastMinute, paris)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.String(), time.Date(2026, time.October, 26, 2, 0, 0, 0, paris).String()))
	})

	t.Run("transition inside the hour does not skip the partial hour", func(t *testing.T) {
		// On 2026-09-27 the Chatham wall clock jumps from 02:45 to 03:45:
		// minutes 45-59 of hour 3 exist that day, so 03:50 must fire. A
		// plain one-hour advance from 02:00 overshoots them onto 04:00.
		sched, err := parseCron("50 3 * * *")
		assert.NilError(t, err)

		after := time.Date(2026, time.September, 27, 0, 0, 0, 0, chatham)
		got, ok := sched.next(after, chatham)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.String(), time.Date(2026, time.September, 27, 3, 50, 0, 0, chatham).String()))
	})

	t.Run("transition skipping midnight does not stall the day search", func(t *testing.T) {
		// On 2026-03-08 the Havana wall clock jumps from 00:00 to 01:00, so
		// that day's midnight normalizes back into March 7.
		schedule, err := parseCron("0 0 * * 1")
		assert.NilError(t, err)

		after := time.Date(2026, time.March, 7, 12, 0, 0, 0, havana) // a Saturday
		got, ok := schedule.next(after, havana)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.String(), time.Date(2026, time.March, 9, 0, 0, 0, 0, havana).String()))
	})

	t.Run("fall back across midnight fires a wildcard on the re-entered date", func(t *testing.T) {
		// Until 2011 America/St_Johns fell back at 00:01: on 2009-11-01 the
		// wall clock returns from 00:01 to 23:01, re-entering October 31.
		// The month search, standing in November, must not jump over the
		// transition to December: the re-entered date holds a second 23:30,
		// which a wildcard schedule fires.
		sched, err := parseCron("30 * 31 10 *")
		assert.NilError(t, err)

		after := time.Date(2009, time.November, 1, 2, 0, 0, 0, time.UTC) // 23:30 NDT, first pass
		got, ok := sched.next(after, stJohns)
		assert.Assert(t, ok)
		// 23:30 NST, the second pass of October 31.
		assert.Check(t, is.Equal(got.UTC().String(), time.Date(2009, time.November, 1, 3, 0, 0, 0, time.UTC).String()))
	})

	t.Run("fall back across midnight fires a wildcard on the re-entered weekday", func(t *testing.T) {
		// The same transition with an unmatched weekday instead of month:
		// the day search, standing on Sunday, must not jump over the return
		// to Saturday October 31.
		sched, err := parseCron("30 * * * 6")
		assert.NilError(t, err)

		after := time.Date(2009, time.November, 1, 2, 0, 0, 0, time.UTC)
		got, ok := sched.next(after, stJohns)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.UTC().String(), time.Date(2009, time.November, 1, 3, 0, 0, 0, time.UTC).String()))
	})

	t.Run("spring forward catch-up on a transition off the whole minute", func(t *testing.T) {
		// Liberia left its -00:44:30 offset on 1972-01-07: the wall clock
		// jumps from 00:00:00 to 00:44:30, at an instant that is not on a
		// whole minute. The minute walk must land exactly on it, or the
		// skipped 00:15 occurrence is never caught up.
		sched, err := parseCron("15 0,23 * * *")
		assert.NilError(t, err)

		after := time.Date(1972, time.January, 7, 0, 0, 0, 0, time.UTC)
		got, ok := sched.next(after, monrovia)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.UTC().String(), time.Date(1972, time.January, 7, 0, 44, 30, 0, time.UTC).String()))

		// The catch-up replaces the occurrence: the next one is that
		// evening's.
		got, ok = sched.next(got, monrovia)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.UTC().String(), time.Date(1972, time.January, 7, 23, 15, 0, 0, time.UTC).String()))
	})

	t.Run("off-minute transition does not catch up an occurrence that fired", func(t *testing.T) {
		// The minute before the Monrovia transition was fully lived: its
		// 23:59 occurrence fired and must not fire a second time as a
		// skipped-time catch-up at the transition half a minute later.
		sched, err := parseCron("59 23 * * *")
		assert.NilError(t, err)

		after := time.Date(1972, time.January, 7, 0, 44, 0, 0, time.UTC) // the 23:59 fire
		got, ok := sched.next(after, monrovia)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.UTC().String(), time.Date(1972, time.January, 7, 23, 59, 0, 0, time.UTC).String()))
	})

	t.Run("rule-era year boundaries do not stall the search", func(t *testing.T) {
		// Beyond its transition table a zone's bounds come from the TZif
		// rule, which Go evaluates with year-granularity segment bounds
		// that can sit at or before a queried instant near a leap year's
		// end; a jump capped at such a bound must keep making progress.
		// Both scans cross December 2040, where the embedded tzdata does
		// exactly that.
		impossible, err := parseCron("0 0 30 2 *")
		assert.NilError(t, err)
		_, ok := impossible.next(time.Date(2026, time.January, 1, 0, 0, 0, 0, paris), paris)
		assert.Check(t, !ok)

		sparse, err := parseCron("0 0 29 2 *")
		assert.NilError(t, err)
		got, ok := sparse.next(time.Date(2040, time.March, 1, 0, 0, 0, 0, paris), paris)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.String(), time.Date(2044, time.February, 29, 0, 0, 0, 0, paris).String()))
	})

	t.Run("fall back across midnight still fires a fixed time once", func(t *testing.T) {
		// The re-entered October 31 repeats 23:30, but a fixed-time
		// schedule already fired on the first pass; its next occurrence is
		// next year's.
		sched, err := parseCron("30 23 31 10 *")
		assert.NilError(t, err)

		after := time.Date(2009, time.November, 1, 2, 0, 0, 0, time.UTC) // the first-pass fire
		got, ok := sched.next(after, stJohns)
		assert.Assert(t, ok)
		assert.Check(t, is.Equal(got.String(), time.Date(2010, time.October, 31, 23, 30, 0, 0, stJohns).String()))
	})
}
