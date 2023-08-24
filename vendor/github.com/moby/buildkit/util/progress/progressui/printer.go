package progressui

import (
	"container/ring"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	digest "github.com/opencontainers/go-digest"
	"github.com/tonistiigi/units"
)

const antiFlicker = 5 * time.Second
const maxDelay = 10 * time.Second
const minTimeDelta = 5 * time.Second
const minProgressDelta = 0.05 // %

const logsBufferSize = 10

type lastStatus struct {
	Current   int64
	Timestamp time.Time
}

type textMux struct {
	w         io.Writer
	current   digest.Digest
	last      map[string]lastStatus
	notFirst  bool
	nextIndex int
}

func (p *textMux) printVtx(t *trace, dgst digest.Digest) {
	if p.last == nil {
		p.last = make(map[string]lastStatus)
	}

	v, ok := t.byDigest[dgst]
	if !ok {
		return
	}

	if v.index == 0 {
		p.nextIndex++
		v.index = p.nextIndex
	}

	if dgst != p.current {
		if p.current != "" {
			old := t.byDigest[p.current]
			if old.logsPartial {
				fmt.Fprintln(p.w, "")
			}
			old.logsOffset = 0
			old.count = 0
			fmt.Fprintf(p.w, "#%d ...\n", old.index)
		}

		if p.notFirst {
			fmt.Fprintln(p.w, "")
		} else {
			p.notFirst = true
		}

		if os.Getenv("PROGRESS_NO_TRUNC") == "0" {
			fmt.Fprintf(p.w, "#%d %s\n", v.index, limitString(v.Name, 72))
		} else {
			fmt.Fprintf(p.w, "#%d %s\n", v.index, v.Name)
		}
	}

	if len(v.events) != 0 {
		v.logsOffset = 0
	}
	for _, ev := range v.events {
		fmt.Fprintf(p.w, "#%d %s\n", v.index, ev)
	}
	v.events = v.events[:0]

	isOpenStatus := false // remote cache loading can currently produce status updates without active vertex
	for _, s := range v.statuses {
		if _, ok := v.statusUpdates[s.ID]; ok {
			doPrint := true

			if last, ok := p.last[s.ID]; ok && s.Completed == nil {
				var progressDelta float64
				if s.Total > 0 {
					progressDelta = float64(s.Current-last.Current) / float64(s.Total)
				}
				timeDelta := s.Timestamp.Sub(last.Timestamp)
				if progressDelta < minProgressDelta && timeDelta < minTimeDelta {
					doPrint = false
				}
			}

			if !doPrint {
				continue
			}

			p.last[s.ID] = lastStatus{
				Timestamp: s.Timestamp,
				Current:   s.Current,
			}

			var bytes string
			if s.Total != 0 {
				bytes = fmt.Sprintf(" %.2f / %.2f", units.Bytes(s.Current), units.Bytes(s.Total))
			} else if s.Current != 0 {
				bytes = fmt.Sprintf(" %.2f", units.Bytes(s.Current))
			}
			var tm string
			endTime := s.Timestamp
			if s.Completed != nil {
				endTime = *s.Completed
			}
			if s.Started != nil {
				diff := endTime.Sub(*s.Started).Seconds()
				if diff > 0.01 {
					tm = fmt.Sprintf(" %.1fs", diff)
				}
			}
			if s.Completed != nil {
				tm += " done"
			} else {
				isOpenStatus = true
			}
			fmt.Fprintf(p.w, "#%d %s%s%s\n", v.index, s.ID, bytes, tm)
		}
	}
	v.statusUpdates = map[string]struct{}{}

	for _, w := range v.warnings[v.warningIdx:] {
		fmt.Fprintf(p.w, "#%d WARN: %s\n", v.index, w.Short)
		v.warningIdx++
	}

	for i, l := range v.logs {
		if i == 0 {
			l = l[v.logsOffset:]
		}
		fmt.Fprintf(p.w, "%s", []byte(l))
		if i != len(v.logs)-1 || !v.logsPartial {
			fmt.Fprintln(p.w, "")
		}
		if v.logsBuffer == nil {
			v.logsBuffer = ring.New(logsBufferSize)
		}
		v.logsBuffer.Value = l
		if !v.logsPartial {
			v.logsBuffer = v.logsBuffer.Next()
		}
	}

	if len(v.logs) > 0 {
		if v.logsPartial {
			v.logs = v.logs[len(v.logs)-1:]
			v.logsOffset = len(v.logs[0])
		} else {
			v.logs = nil
			v.logsOffset = 0
		}
	}

	p.current = dgst
	if v.isCompleted() && !isOpenStatus {
		p.current = ""
		v.count = 0

		if v.logsPartial {
			fmt.Fprintln(p.w, "")
		}
		if v.Error != "" {
			if strings.HasSuffix(v.Error, context.Canceled.Error()) {
				fmt.Fprintf(p.w, "#%d CANCELED\n", v.index)
			} else {
				fmt.Fprintf(p.w, "#%d ERROR: %s\n", v.index, v.Error)
			}
		} else if v.Cached {
			fmt.Fprintf(p.w, "#%d CACHED\n", v.index)
		} else {
			tm := ""
			var ivals []interval
			for _, ival := range v.intervals {
				ivals = append(ivals, ival)
			}
			ivals = mergeIntervals(ivals)
			if len(ivals) > 0 {
				var dt float64
				for _, ival := range ivals {
					dt += ival.duration().Seconds()
				}
				tm = fmt.Sprintf(" %.1fs", dt)
			}
			fmt.Fprintf(p.w, "#%d DONE%s\n", v.index, tm)
		}
	}

	delete(t.updates, dgst)
}

func sortCompleted(t *trace, m map[digest.Digest]struct{}) []digest.Digest {
	out := make([]digest.Digest, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		vtxi := t.byDigest[out[i]]
		vtxj := t.byDigest[out[j]]
		return vtxi.mostRecentInterval().stop.Before(*vtxj.mostRecentInterval().stop)
	})
	return out
}

func (p *textMux) print(t *trace) {
	completed := map[digest.Digest]struct{}{}
	rest := map[digest.Digest]struct{}{}

	for dgst := range t.updates {
		v, ok := t.byDigest[dgst]
		if !ok {
			continue
		}
		if v.ProgressGroup != nil || v.hidden {
			// skip vtxs in a group (they are merged into a single vtx) and hidden ones
			continue
		}
		if v.isCompleted() {
			completed[dgst] = struct{}{}
		} else {
			rest[dgst] = struct{}{}
		}
	}

	current := p.current

	// items that have completed need to be printed first
	if _, ok := completed[current]; ok {
		p.printVtx(t, current)
	}

	for _, dgst := range sortCompleted(t, completed) {
		if dgst != current {
			p.printVtx(t, dgst)
		}
	}

	if len(rest) == 0 {
		if current != "" {
			if v := t.byDigest[current]; v.isStarted() && !v.isCompleted() {
				return
			}
		}
		// make any open vertex active
		for dgst, v := range t.byDigest {
			if v.isStarted() && !v.isCompleted() && v.ProgressGroup == nil && !v.hidden {
				p.printVtx(t, dgst)
				return
			}
		}
		return
	}

	// now print the active one
	if _, ok := rest[current]; ok {
		p.printVtx(t, current)
	}

	stats := map[digest.Digest]*vtxStat{}
	now := time.Now()
	sum := 0.0
	var max digest.Digest
	if current != "" {
		rest[current] = struct{}{}
	}
	for dgst := range rest {
		v, ok := t.byDigest[dgst]
		if !ok {
			continue
		}
		if v.lastBlockTime == nil {
			// shouldn't happen, but not worth crashing over
			continue
		}
		tm := now.Sub(*v.lastBlockTime)
		speed := float64(v.count) / tm.Seconds()
		overLimit := tm > maxDelay && dgst != current
		stats[dgst] = &vtxStat{blockTime: tm, speed: speed, overLimit: overLimit}
		sum += speed
		if overLimit || max == "" || stats[max].speed < speed {
			max = dgst
		}
	}
	for dgst := range stats {
		stats[dgst].share = stats[dgst].speed / sum
	}

	if _, ok := completed[current]; ok || current == "" {
		p.printVtx(t, max)
		return
	}

	// show items that were hidden
	for dgst := range rest {
		if stats[dgst].overLimit {
			p.printVtx(t, dgst)
			return
		}
	}

	// fair split between vertexes
	if 1.0/(1.0-stats[current].share)*antiFlicker.Seconds() < stats[current].blockTime.Seconds() {
		p.printVtx(t, max)
		return
	}
}

type vtxStat struct {
	blockTime time.Duration
	speed     float64
	share     float64
	overLimit bool
}

func limitString(s string, l int) string {
	if len(s) > l {
		return s[:l] + "..."
	}
	return s
}
