package progressui

import (
	"bytes"
	"container/ring"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/console"
	"github.com/moby/buildkit/client"
	"github.com/morikuni/aec"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/tonistiigi/units"
	"github.com/tonistiigi/vt100"
	"golang.org/x/time/rate"
)

type displayOpts struct {
	phase       string
	textDesc    string
	consoleDesc string
}

func newDisplayOpts(opts ...DisplayOpt) *displayOpts {
	dsso := &displayOpts{}
	for _, opt := range opts {
		opt(dsso)
	}
	return dsso
}

type DisplayOpt func(b *displayOpts)

func WithPhase(phase string) DisplayOpt {
	return func(b *displayOpts) {
		b.phase = phase
	}
}

func WithDesc(text string, console string) DisplayOpt {
	return func(b *displayOpts) {
		b.textDesc = text
		b.consoleDesc = console
	}
}

type Display struct {
	disp display
}

type display interface {
	// init initializes the display and opens any resources
	// that are required.
	init(displayLimiter *rate.Limiter)

	// update sends the signal to update the display.
	// Some displays will have buffered output and will not
	// display changes for every status update.
	update(ss *client.SolveStatus)

	// refresh updates the display with the latest state.
	// This method only does something with displays that
	// have buffered output.
	refresh()

	// done is invoked when the display will be closed.
	// This method should flush any buffers and close any open
	// resources that were opened by init.
	done()
}

func (d Display) UpdateFrom(ctx context.Context, ch chan *client.SolveStatus) ([]client.VertexWarning, error) {
	tickerTimeout := 150 * time.Millisecond
	displayTimeout := 100 * time.Millisecond

	if v := os.Getenv("TTY_DISPLAY_RATE"); v != "" {
		if r, err := strconv.ParseInt(v, 10, 64); err == nil {
			tickerTimeout = time.Duration(r) * time.Millisecond
			displayTimeout = time.Duration(r) * time.Millisecond
		}
	}

	displayLimiter := rate.NewLimiter(rate.Every(displayTimeout), 1)
	d.disp.init(displayLimiter)
	defer d.disp.done()

	ticker := time.NewTicker(tickerTimeout)
	defer ticker.Stop()

	var warnings []client.VertexWarning
	for {
		select {
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		case <-ticker.C:
			d.disp.refresh()
		case ss, ok := <-ch:
			if !ok {
				return warnings, nil
			}

			d.disp.update(ss)
			for _, w := range ss.Warnings {
				warnings = append(warnings, *w)
			}
			ticker.Reset(tickerTimeout)
		}
	}
}

type DisplayMode string

const (
	// DefaultMode is the default value for the DisplayMode.
	// This is effectively the same as AutoMode.
	DefaultMode DisplayMode = ""
	// AutoMode will choose TtyMode or PlainMode depending on if the output is
	// a tty.
	AutoMode DisplayMode = "auto"
	// QuietMode discards all output.
	QuietMode DisplayMode = "quiet"
	// TtyMode enforces the output is a tty and will otherwise cause an error if it isn't.
	TtyMode DisplayMode = "tty"
	// PlainMode is the human-readable plain text output. This mode is not meant to be read
	// by machines.
	PlainMode DisplayMode = "plain"
	// RawJSONMode is the raw JSON text output. It will marshal the various solve status events
	// to JSON to be read by an external program.
	RawJSONMode DisplayMode = "rawjson"
)

// NewDisplay constructs a Display that outputs to the given io.Writer with the given DisplayMode.
//
// This method will return an error when the DisplayMode is invalid or if TtyMode is used but the io.Writer
// does not refer to a tty. AutoMode will choose TtyMode or PlainMode depending on if the output is a tty or not.
//
// For TtyMode to work, the io.Writer should also implement console.File.
func NewDisplay(out io.Writer, mode DisplayMode, opts ...DisplayOpt) (Display, error) {
	switch mode {
	case AutoMode, TtyMode, DefaultMode:
		if c, err := consoleFromWriter(out); err == nil {
			return newConsoleDisplay(c, opts...), nil
		} else if mode == "tty" {
			return Display{}, errors.Wrap(err, "failed to get console")
		}
		fallthrough
	case PlainMode:
		return newPlainDisplay(out, opts...), nil
	case RawJSONMode:
		return newRawJSONDisplay(out), nil
	case QuietMode:
		return newDiscardDisplay(), nil
	default:
		return Display{}, errors.Errorf("invalid progress mode %s", mode)
	}
}

// consoleFromWriter retrieves a console.Console from an io.Writer.
func consoleFromWriter(out io.Writer) (console.Console, error) {
	f, ok := out.(console.File)
	if !ok {
		return nil, errors.New("output is not a file")
	}
	return console.ConsoleFromFile(f)
}

type discardDisplay struct{}

func newDiscardDisplay() Display {
	return Display{disp: &discardDisplay{}}
}

func (d *discardDisplay) init(displayLimiter *rate.Limiter) {}
func (d *discardDisplay) update(ss *client.SolveStatus)     {}
func (d *discardDisplay) refresh()                          {}
func (d *discardDisplay) done()                             {}

type consoleDisplay struct {
	t              *trace
	disp           *ttyDisplay
	width, height  int
	displayLimiter *rate.Limiter
}

// newConsoleDisplay creates a new Display that prints a TTY
// friendly output.
func newConsoleDisplay(c console.Console, opts ...DisplayOpt) Display {
	dsso := newDisplayOpts(opts...)
	if dsso.phase == "" {
		dsso.phase = "Building"
	}
	return Display{
		disp: &consoleDisplay{
			t:    newTrace(c, true),
			disp: &ttyDisplay{c: c, phase: dsso.phase, desc: dsso.consoleDesc},
		},
	}
}

func (d *consoleDisplay) init(displayLimiter *rate.Limiter) {
	d.displayLimiter = displayLimiter
}

func (d *consoleDisplay) update(ss *client.SolveStatus) {
	d.width, d.height = d.disp.getSize()
	d.t.update(ss, d.width)
	if !d.displayLimiter.Allow() {
		// Exit early as we are not allowed to update the display.
		return
	}
	d.refresh()
}

func (d *consoleDisplay) refresh() {
	d.disp.print(d.t.displayInfo(), d.width, d.height, false)
}

func (d *consoleDisplay) done() {
	d.width, d.height = d.disp.getSize()
	d.disp.print(d.t.displayInfo(), d.width, d.height, true)
	d.t.printErrorLogs(d.t.w)
}

type plainDisplay struct {
	t              *trace
	printer        *textMux
	displayLimiter *rate.Limiter
}

// newPlainDisplay creates a new Display that outputs the status
// in a human-readable plain-text format.
func newPlainDisplay(w io.Writer, opts ...DisplayOpt) Display {
	dsso := newDisplayOpts(opts...)
	return Display{
		disp: &plainDisplay{
			t: newTrace(w, false),
			printer: &textMux{
				w:    w,
				desc: dsso.textDesc,
			},
		},
	}
}

func (d *plainDisplay) init(displayLimiter *rate.Limiter) {
	d.displayLimiter = displayLimiter
}

func (d *plainDisplay) update(ss *client.SolveStatus) {
	if ss != nil {
		d.t.update(ss, 80)
		if !d.displayLimiter.Allow() {
			// Exit early as we are not allowed to update the display.
			return
		}
	}
	d.refresh()
}

func (d *plainDisplay) refresh() {
	d.printer.print(d.t)
}

func (d *plainDisplay) done() {
	// Force the display to refresh.
	d.refresh()
	// Print error logs.
	d.t.printErrorLogs(d.t.w)
}

type rawJSONDisplay struct {
	enc *json.Encoder
	w   io.Writer
}

// newRawJSONDisplay creates a new Display that outputs an unbuffered
// output of status update events.
func newRawJSONDisplay(w io.Writer) Display {
	enc := json.NewEncoder(w)
	return Display{
		disp: &rawJSONDisplay{
			enc: enc,
			w:   w,
		},
	}
}

func (d *rawJSONDisplay) init(displayLimiter *rate.Limiter) {
	// Initialization parameters are ignored for this display.
}

func (d *rawJSONDisplay) update(ss *client.SolveStatus) {
	_ = d.enc.Encode(ss)
}

func (d *rawJSONDisplay) refresh() {
	// Unbuffered display doesn't have anything to refresh.
}

func (d *rawJSONDisplay) done() {
	// No actions needed.
}

const termPad = 10

type displayInfo struct {
	startTime      time.Time
	jobs           []*job
	countTotal     int
	countCompleted int
}

type job struct {
	intervals   []interval
	isCompleted bool
	name        string
	status      string
	hasError    bool
	hasWarning  bool // This is currently unused, but it's here for future use.
	isCanceled  bool
	vertex      *vertex
	showTerm    bool
}

type trace struct {
	w             io.Writer
	startTime     *time.Time
	localTimeDiff time.Duration
	vertexes      []*vertex
	byDigest      map[digest.Digest]*vertex
	updates       map[digest.Digest]struct{}
	modeConsole   bool
	groups        map[string]*vertexGroup // group id -> group
}

type vertex struct {
	*client.Vertex

	statuses []*status
	byID     map[string]*status
	indent   string
	index    int

	logs          [][]byte
	logsPartial   bool
	logsOffset    int
	logsBuffer    *ring.Ring // stores last logs to print them on error
	prev          *client.Vertex
	events        []string
	lastBlockTime *time.Time
	count         int
	statusUpdates map[string]struct{}

	warnings   []client.VertexWarning
	warningIdx int

	jobs      []*job
	jobCached bool

	term      *vt100.VT100
	termBytes int
	termCount int

	// Interval start time in unix nano -> interval. Using a map ensures
	// that updates for the same interval overwrite their previous updates.
	intervals       map[int64]interval
	mergedIntervals []interval

	// whether the vertex should be hidden due to being in a progress group
	// that doesn't have any non-weak members that have started
	hidden bool
}

func (v *vertex) update(c int) {
	if v.count == 0 {
		now := time.Now()
		v.lastBlockTime = &now
	}
	v.count += c
}

func (v *vertex) mostRecentInterval() *interval {
	if v.isStarted() {
		ival := v.mergedIntervals[len(v.mergedIntervals)-1]
		return &ival
	}
	return nil
}

func (v *vertex) isStarted() bool {
	return len(v.mergedIntervals) > 0
}

func (v *vertex) isCompleted() bool {
	if ival := v.mostRecentInterval(); ival != nil {
		return ival.stop != nil
	}
	return false
}

type vertexGroup struct {
	*vertex
	subVtxs map[digest.Digest]client.Vertex
}

func (vg *vertexGroup) refresh() (changed, newlyStarted, newlyRevealed bool) {
	newVtx := *vg.Vertex
	newVtx.Cached = true
	alreadyStarted := vg.isStarted()
	wasHidden := vg.hidden
	for _, subVtx := range vg.subVtxs {
		if subVtx.Started != nil {
			newInterval := interval{
				start: subVtx.Started,
				stop:  subVtx.Completed,
			}
			prevInterval := vg.intervals[subVtx.Started.UnixNano()]
			if !newInterval.isEqual(prevInterval) {
				changed = true
			}
			if !alreadyStarted {
				newlyStarted = true
			}
			vg.intervals[subVtx.Started.UnixNano()] = newInterval

			if !subVtx.ProgressGroup.Weak {
				vg.hidden = false
			}
		}

		// Group is considered cached iff all subvtxs are cached
		newVtx.Cached = newVtx.Cached && subVtx.Cached

		// Group error is set to the first error found in subvtxs, if any
		if newVtx.Error == "" {
			newVtx.Error = subVtx.Error
		} else {
			vg.hidden = false
		}
	}

	if vg.Cached != newVtx.Cached {
		changed = true
	}
	if vg.Error != newVtx.Error {
		changed = true
	}
	vg.Vertex = &newVtx

	if !vg.hidden && wasHidden {
		changed = true
		newlyRevealed = true
	}

	var ivals []interval
	for _, ival := range vg.intervals {
		ivals = append(ivals, ival)
	}
	vg.mergedIntervals = mergeIntervals(ivals)

	return changed, newlyStarted, newlyRevealed
}

type interval struct {
	start *time.Time
	stop  *time.Time
}

func (ival interval) duration() time.Duration {
	if ival.start == nil {
		return 0
	}
	if ival.stop == nil {
		return time.Since(*ival.start)
	}
	return ival.stop.Sub(*ival.start)
}

func (ival interval) isEqual(other interval) (isEqual bool) {
	return equalTimes(ival.start, other.start) && equalTimes(ival.stop, other.stop)
}

func equalTimes(t1, t2 *time.Time) bool {
	if t2 == nil {
		return t1 == nil
	}
	if t1 == nil {
		return false
	}
	return t1.Equal(*t2)
}

// mergeIntervals takes a slice of (start, stop) pairs and returns a slice where
// any intervals that overlap in time are combined into a single interval. If an
// interval's stop time is nil, it is treated as positive infinity and consumes
// any intervals after it. Intervals with nil start times are ignored and not
// returned.
func mergeIntervals(intervals []interval) []interval {
	// remove any intervals that have not started
	var filtered []interval
	for _, interval := range intervals {
		if interval.start != nil {
			filtered = append(filtered, interval)
		}
	}
	intervals = filtered

	if len(intervals) == 0 {
		return nil
	}

	// sort intervals by start time
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].start.Before(*intervals[j].start)
	})

	var merged []interval
	cur := intervals[0]
	for i := 1; i < len(intervals); i++ {
		next := intervals[i]
		if cur.stop == nil {
			// if cur doesn't stop, all intervals after it will be merged into it
			merged = append(merged, cur)
			return merged
		}
		if cur.stop.Before(*next.start) {
			// if cur stops before next starts, no intervals after cur will be
			// merged into it; cur stands on its own
			merged = append(merged, cur)
			cur = next
			continue
		}
		if next.stop == nil {
			// cur and next partially overlap, but next also never stops, so all
			// subsequent intervals will be merged with both cur and next
			merged = append(merged, interval{
				start: cur.start,
				stop:  nil,
			})
			return merged
		}
		if cur.stop.After(*next.stop) || cur.stop.Equal(*next.stop) {
			// cur fully subsumes next
			continue
		}
		// cur partially overlaps with next, merge them together into cur
		cur = interval{
			start: cur.start,
			stop:  next.stop,
		}
	}
	// append anything we are left with
	merged = append(merged, cur)
	return merged
}

type status struct {
	*client.VertexStatus
}

func newTrace(w io.Writer, modeConsole bool) *trace {
	return &trace{
		byDigest:    make(map[digest.Digest]*vertex),
		updates:     make(map[digest.Digest]struct{}),
		w:           w,
		modeConsole: modeConsole,
		groups:      make(map[string]*vertexGroup),
	}
}

func (t *trace) triggerVertexEvent(v *client.Vertex) {
	if v.Started == nil {
		return
	}

	var old client.Vertex
	vtx := t.byDigest[v.Digest]
	if v := vtx.prev; v != nil {
		old = *v
	}

	changed := false
	if v.Digest != old.Digest {
		changed = true
	}
	if v.Name != old.Name {
		changed = true
	}
	if v.Started != old.Started {
		if v.Started != nil && old.Started == nil || !v.Started.Equal(*old.Started) {
			changed = true
		}
	}
	if v.Completed != old.Completed && v.Completed != nil {
		changed = true
	}
	if v.Cached != old.Cached {
		changed = true
	}
	if v.Error != old.Error {
		changed = true
	}

	if changed {
		vtx.update(1)
		t.updates[v.Digest] = struct{}{}
	}

	t.byDigest[v.Digest].prev = v
}

func (t *trace) update(s *client.SolveStatus, termWidth int) {
	seenGroups := make(map[string]struct{})
	var groups []string
	for _, v := range s.Vertexes {
		if t.startTime == nil {
			t.startTime = v.Started
		}
		if v.ProgressGroup != nil {
			group, ok := t.groups[v.ProgressGroup.Id]
			if !ok {
				group = &vertexGroup{
					vertex: &vertex{
						Vertex: &client.Vertex{
							Digest: digest.Digest(v.ProgressGroup.Id),
							Name:   v.ProgressGroup.Name,
						},
						byID:          make(map[string]*status),
						statusUpdates: make(map[string]struct{}),
						intervals:     make(map[int64]interval),
						hidden:        true,
					},
					subVtxs: make(map[digest.Digest]client.Vertex),
				}
				if t.modeConsole {
					group.term = vt100.NewVT100(termHeight, termWidth-termPad)
				}
				t.groups[v.ProgressGroup.Id] = group
				t.byDigest[group.Digest] = group.vertex
			}
			if _, ok := seenGroups[v.ProgressGroup.Id]; !ok {
				groups = append(groups, v.ProgressGroup.Id)
				seenGroups[v.ProgressGroup.Id] = struct{}{}
			}
			group.subVtxs[v.Digest] = *v
			t.byDigest[v.Digest] = group.vertex
			continue
		}
		prev, ok := t.byDigest[v.Digest]
		if !ok {
			t.byDigest[v.Digest] = &vertex{
				byID:          make(map[string]*status),
				statusUpdates: make(map[string]struct{}),
				intervals:     make(map[int64]interval),
			}
			if t.modeConsole {
				t.byDigest[v.Digest].term = vt100.NewVT100(termHeight, termWidth-termPad)
			}
		}
		t.triggerVertexEvent(v)
		if v.Started != nil && (prev == nil || !prev.isStarted()) {
			if t.localTimeDiff == 0 {
				t.localTimeDiff = time.Since(*v.Started)
			}
			t.vertexes = append(t.vertexes, t.byDigest[v.Digest])
		}
		// allow a duplicate initial vertex that shouldn't reset state
		if !(prev != nil && prev.isStarted() && v.Started == nil) {
			t.byDigest[v.Digest].Vertex = v
		}
		if v.Started != nil {
			t.byDigest[v.Digest].intervals[v.Started.UnixNano()] = interval{
				start: v.Started,
				stop:  v.Completed,
			}
			var ivals []interval
			for _, ival := range t.byDigest[v.Digest].intervals {
				ivals = append(ivals, ival)
			}
			t.byDigest[v.Digest].mergedIntervals = mergeIntervals(ivals)
		}
		t.byDigest[v.Digest].jobCached = false
	}
	for _, groupID := range groups {
		group := t.groups[groupID]
		changed, newlyStarted, newlyRevealed := group.refresh()
		if newlyStarted {
			if t.localTimeDiff == 0 {
				t.localTimeDiff = time.Since(*group.mergedIntervals[0].start)
			}
		}
		if group.hidden {
			continue
		}
		if newlyRevealed {
			t.vertexes = append(t.vertexes, group.vertex)
		}
		if changed {
			group.update(1)
			t.updates[group.Digest] = struct{}{}
		}
		group.jobCached = false
	}
	for _, s := range s.Statuses {
		v, ok := t.byDigest[s.Vertex]
		if !ok {
			continue // shouldn't happen
		}
		v.jobCached = false
		prev, ok := v.byID[s.ID]
		if !ok {
			v.byID[s.ID] = &status{VertexStatus: s}
		}
		if s.Started != nil && (prev == nil || prev.Started == nil) {
			v.statuses = append(v.statuses, v.byID[s.ID])
		}
		v.byID[s.ID].VertexStatus = s
		v.statusUpdates[s.ID] = struct{}{}
		t.updates[v.Digest] = struct{}{}
		v.update(1)
	}
	for _, w := range s.Warnings {
		v, ok := t.byDigest[w.Vertex]
		if !ok {
			continue // shouldn't happen
		}
		v.warnings = append(v.warnings, *w)
		v.update(1)
	}
	for _, l := range s.Logs {
		v, ok := t.byDigest[l.Vertex]
		if !ok {
			continue // shouldn't happen
		}
		v.jobCached = false
		if v.term != nil {
			if v.term.Width != termWidth {
				termHeight = max(termHeightMin, min(termHeightInitial, v.term.Height-termHeightMin-1))
				v.term.Resize(termHeight, termWidth-termPad)
			}
			v.termBytes += len(l.Data)
			v.term.Write(l.Data) // error unhandled on purpose. don't trust vt100
		}
		i := 0
		complete := split(l.Data, byte('\n'), func(dt []byte) {
			if v.logsPartial && len(v.logs) != 0 && i == 0 {
				v.logs[len(v.logs)-1] = append(v.logs[len(v.logs)-1], dt...)
			} else {
				ts := time.Duration(0)
				if ival := v.mostRecentInterval(); ival != nil {
					ts = l.Timestamp.Sub(*ival.start)
				}
				prec := 1
				sec := ts.Seconds()
				if sec < 10 {
					prec = 3
				} else if sec < 100 {
					prec = 2
				}
				v.logs = append(v.logs, []byte(fmt.Sprintf("%s %s", fmt.Sprintf("%.[2]*[1]f", sec, prec), dt)))
			}
			i++
		})
		v.logsPartial = !complete
		t.updates[v.Digest] = struct{}{}
		v.update(1)
	}
}

func (t *trace) printErrorLogs(f io.Writer) {
	for _, v := range t.vertexes {
		if v.Error != "" && !strings.HasSuffix(v.Error, context.Canceled.Error()) {
			fmt.Fprintln(f, "------")
			fmt.Fprintf(f, " > %s:\n", v.Name)
			// tty keeps original logs
			for _, l := range v.logs {
				f.Write(l)
				fmt.Fprintln(f)
			}
			// printer keeps last logs buffer
			if v.logsBuffer != nil {
				for i := 0; i < v.logsBuffer.Len(); i++ {
					if v.logsBuffer.Value != nil {
						fmt.Fprintln(f, string(v.logsBuffer.Value.([]byte)))
					}
					v.logsBuffer = v.logsBuffer.Next()
				}
			}
			fmt.Fprintln(f, "------")
		}
	}
}

func (t *trace) displayInfo() (d displayInfo) {
	d.startTime = time.Now()
	if t.startTime != nil {
		d.startTime = t.startTime.Add(t.localTimeDiff)
	}
	d.countTotal = len(t.byDigest)
	for _, v := range t.byDigest {
		if v.ProgressGroup != nil || v.hidden {
			// don't count vtxs in a group, they are merged into a single vtx
			d.countTotal--
			continue
		}
		if v.isCompleted() {
			d.countCompleted++
		}
	}

	for _, v := range t.vertexes {
		if v.jobCached {
			d.jobs = append(d.jobs, v.jobs...)
			continue
		}
		var jobs []*job
		j := &job{
			name:        strings.ReplaceAll(v.Name, "\t", " "),
			vertex:      v,
			isCompleted: true,
		}
		for _, ival := range v.intervals {
			j.intervals = append(j.intervals, interval{
				start: addTime(ival.start, t.localTimeDiff),
				stop:  addTime(ival.stop, t.localTimeDiff),
			})
			if ival.stop == nil {
				j.isCompleted = false
			}
		}
		j.intervals = mergeIntervals(j.intervals)
		if v.Error != "" {
			if strings.HasSuffix(v.Error, context.Canceled.Error()) {
				j.isCanceled = true
				j.name = "CANCELED " + j.name
			} else {
				j.hasError = true
				j.name = "ERROR " + j.name
			}
		}
		if v.Cached {
			j.name = "CACHED " + j.name
		}
		j.name = v.indent + j.name
		jobs = append(jobs, j)
		for _, s := range v.statuses {
			j := &job{
				intervals: []interval{{
					start: addTime(s.Started, t.localTimeDiff),
					stop:  addTime(s.Completed, t.localTimeDiff),
				}},
				isCompleted: s.Completed != nil,
				name:        v.indent + "=> " + s.ID,
			}
			if s.Total != 0 {
				j.status = fmt.Sprintf("%.2f / %.2f", units.Bytes(s.Current), units.Bytes(s.Total))
			} else if s.Current != 0 {
				j.status = fmt.Sprintf("%.2f", units.Bytes(s.Current))
			}
			jobs = append(jobs, j)
		}
		for _, w := range v.warnings {
			msg := "WARN: " + string(w.Short)
			var mostRecentInterval interval
			if ival := v.mostRecentInterval(); ival != nil {
				mostRecentInterval = *ival
			}
			j := &job{
				intervals: []interval{{
					start: addTime(mostRecentInterval.start, t.localTimeDiff),
					stop:  addTime(mostRecentInterval.stop, t.localTimeDiff),
				}},
				name:       msg,
				isCanceled: true,
			}
			jobs = append(jobs, j)
		}
		d.jobs = append(d.jobs, jobs...)
		v.jobs = jobs
		v.jobCached = true
	}

	return d
}

func split(dt []byte, sep byte, fn func([]byte)) bool {
	if len(dt) == 0 {
		return false
	}
	for {
		if len(dt) == 0 {
			return true
		}
		idx := bytes.IndexByte(dt, sep)
		if idx == -1 {
			fn(dt)
			return false
		}
		fn(dt[:idx])
		dt = dt[idx+1:]
	}
}

func addTime(tm *time.Time, d time.Duration) *time.Time {
	if tm == nil {
		return nil
	}
	t := tm.Add(d)
	return &t
}

type ttyDisplay struct {
	c         console.Console
	phase     string
	desc      string
	lineCount int
	repeated  bool
}

func (disp *ttyDisplay) getSize() (int, int) {
	width := 80
	height := 10
	if disp.c != nil {
		size, err := disp.c.Size()
		if err == nil && size.Width > 0 && size.Height > 0 {
			width = int(size.Width)
			height = int(size.Height)
		}
	}
	return width, height
}

func setupTerminals(jobs []*job, height int, all bool) []*job {
	var candidates []*job
	numInUse := 0
	for _, j := range jobs {
		if j.vertex != nil && j.vertex.termBytes > 0 && !j.isCompleted {
			candidates = append(candidates, j)
		}
		if !j.isCompleted {
			numInUse++
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		idxI := candidates[i].vertex.termBytes + candidates[i].vertex.termCount*50
		idxJ := candidates[j].vertex.termBytes + candidates[j].vertex.termCount*50
		return idxI > idxJ
	})

	numFree := height - 2 - numInUse
	numToHide := 0
	termHeight = max(termHeightMin, min(termHeightInitial, height-termHeightMin-1))
	termLimit := termHeight + 3

	for i := 0; numFree > termLimit && i < len(candidates); i++ {
		candidates[i].showTerm = true
		numToHide += candidates[i].vertex.term.UsedHeight()
		numFree -= termLimit
	}

	if !all {
		jobs = wrapHeight(jobs, height-2-numToHide)
	}

	return jobs
}

func (disp *ttyDisplay) print(d displayInfo, width, height int, all bool) {
	// this output is inspired by Buck
	d.jobs = setupTerminals(d.jobs, height, all)
	b := aec.EmptyBuilder
	for i := 0; i <= disp.lineCount; i++ {
		b = b.Up(1)
	}
	if !disp.repeated {
		b = b.Down(1)
	}
	disp.repeated = true
	fmt.Fprint(disp.c, b.Column(0).ANSI)

	statusStr := ""
	if d.countCompleted > 0 && d.countCompleted == d.countTotal && all {
		statusStr = "FINISHED"
	}

	fmt.Fprint(disp.c, aec.Hide)
	defer fmt.Fprint(disp.c, aec.Show)

	out := fmt.Sprintf("[+] %s %.1fs (%d/%d) %s", disp.phase, time.Since(d.startTime).Seconds(), d.countCompleted, d.countTotal, statusStr)
	if disp.desc != "" {
		out = align(out, disp.desc, width-1)
	} else {
		out = align(out, "", width)
	}
	fmt.Fprintln(disp.c, out)
	lineCount := 0
	for _, j := range d.jobs {
		if len(j.intervals) == 0 {
			continue
		}
		var dt float64
		for _, ival := range j.intervals {
			dt += ival.duration().Seconds()
		}
		if dt < 0.05 {
			dt = 0
		}
		pfx := " => "
		timer := fmt.Sprintf(" %3.1fs\n", dt)
		status := j.status
		showStatus := false

		left := width - len(pfx) - len(timer) - 1
		if status != "" {
			if left+len(status) > 20 {
				showStatus = true
				left -= len(status) + 1
			}
		}
		if left < 12 { // too small screen to show progress
			continue
		}
		name := j.name
		if len(name) > left {
			name = name[:left]
		}

		out := pfx + name
		if showStatus {
			out += " " + status
		}

		out = align(out, timer, width)
		if j.isCompleted {
			color := colorRun
			if j.isCanceled {
				color = colorCancel
			} else if j.hasError {
				color = colorError
			} else if j.hasWarning {
				// This is currently unused, but it's here for future use.
				color = colorWarning
			}
			if color != nil {
				out = aec.Apply(out, color)
			}
		}
		fmt.Fprint(disp.c, out)
		lineCount++
		if j.showTerm {
			term := j.vertex.term
			term.Resize(termHeight, width-termPad)
			for _, l := range term.Content {
				if !isEmpty(l) {
					out := aec.Apply(fmt.Sprintf(" => => # %s\n", string(l)), aec.Faint)
					fmt.Fprint(disp.c, out)
					lineCount++
				}
			}
			j.vertex.termCount++
			j.showTerm = false
		}
	}
	// override previous content
	if diff := disp.lineCount - lineCount; diff > 0 {
		for i := 0; i < diff; i++ {
			fmt.Fprintln(disp.c, strings.Repeat(" ", width))
		}
		fmt.Fprint(disp.c, aec.EmptyBuilder.Up(uint(diff)).Column(0).ANSI)
	}
	disp.lineCount = lineCount
}

func isEmpty(l []rune) bool {
	for _, r := range l {
		if r != ' ' {
			return false
		}
	}
	return true
}

func align(l, r string, w int) string {
	return fmt.Sprintf("%-[2]*[1]s %[3]s", l, w-len(r)-1, r)
}

func wrapHeight(j []*job, limit int) []*job {
	if limit < 0 {
		return nil
	}
	var wrapped []*job
	wrapped = append(wrapped, j...)
	if len(j) > limit {
		wrapped = wrapped[len(j)-limit:]

		// wrap things around if incomplete jobs were cut
		var invisible []*job
		for _, j := range j[:len(j)-limit] {
			if !j.isCompleted {
				invisible = append(invisible, j)
			}
		}

		if l := len(invisible); l > 0 {
			rewrapped := make([]*job, 0, len(wrapped))
			for _, j := range wrapped {
				if !j.isCompleted || l <= 0 {
					rewrapped = append(rewrapped, j)
				}
				l--
			}
			freespace := len(wrapped) - len(rewrapped)
			wrapped = append(invisible[len(invisible)-freespace:], rewrapped...)
		}
	}
	return wrapped
}
