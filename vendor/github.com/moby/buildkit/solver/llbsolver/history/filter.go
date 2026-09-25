package history

import (
	"slices"
	"strings"
	"time"

	"github.com/containerd/containerd/v2/pkg/filters"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/util/gitutil"
	"github.com/pkg/errors"
	"github.com/tonistiigi/go-csvvalue"
)

const (
	statusRunning   = "running"
	statusCompleted = "completed"
	statusError     = "error"
	statusCanceled  = "canceled"
)

func filterHistoryEvents(in []*controlapi.BuildHistoryEvent, filters []string, limit int32) ([]*controlapi.BuildHistoryEvent, error) {
	f, err := parseFilters(filters)
	if err != nil {
		return nil, err
	}

	events := make([]*controlapi.BuildHistoryEvent, 0, len(in))
	for _, ev := range in {
		if ev == nil {
			continue
		}
		events = append(events, ev)
	}

	out := make([]*controlapi.BuildHistoryEvent, 0, len(events))

	if len(f) == 0 {
		out = events
	} else {
	loop0:
		for _, ev := range events {
			for _, fn := range f {
				if fn(ev) {
					out = append(out, ev)
					continue loop0
				}
			}
		}
	}

	if limit != 0 {
		if limit < 0 {
			return nil, errors.Errorf("invalid limit %d", limit)
		}
		slices.SortFunc(out, func(a, b *controlapi.BuildHistoryEvent) int {
			aRec := a.Record != nil
			bRec := b.Record != nil
			switch {
			case !aRec && !bRec:
				return 0
			case !aRec:
				return 1
			case !bRec:
				return -1
			}
			return b.Record.CreatedAt.AsTime().Compare(a.Record.CreatedAt.AsTime())
		})
		if int32(len(out)) > limit {
			out = out[:limit]
		}
	}
	return out, nil
}

func parseFilters(in []string) ([]func(*controlapi.BuildHistoryEvent) bool, error) {
	if len(in) == 0 {
		return nil, nil
	}

	var out []func(*controlapi.BuildHistoryEvent) bool
	for _, in := range in {
		fns, err := parseFilter(in)
		if err != nil {
			return nil, err
		}
		out = append(out, func(ev *controlapi.BuildHistoryEvent) bool {
			for _, fn := range fns {
				if !fn(ev) {
					return false
				}
			}
			return true
		})
	}
	return out, nil
}

func timeBasedFilter(f string) (func(*controlapi.BuildHistoryEvent) bool, error) {
	key, sep, value, _ := cutAny(f, []string{">=", "<=", ">", "<"})
	var cmp int64
	switch key {
	case "startedAt", "completedAt":
		v, err := time.ParseDuration(value)
		if err == nil {
			tm := time.Now().Add(-v)
			cmp = tm.Unix()
		} else {
			tm, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return nil, errors.Errorf("invalid time %s", value)
			}
			cmp = tm.Unix()
		}
	case "duration":
		v, err := time.ParseDuration(value)
		if err != nil {
			return nil, errors.Errorf("invalid duration %s", value)
		}
		cmp = int64(v)
	default:
		return nil, nil
	}

	return func(ev *controlapi.BuildHistoryEvent) bool {
		if ev.Record == nil {
			return false
		}
		var val int64
		switch key {
		case "startedAt":
			val = ev.Record.CreatedAt.AsTime().Unix()
		case "completedAt":
			if ev.Record.CompletedAt != nil {
				val = ev.Record.CompletedAt.AsTime().Unix()
			}
		case "duration":
			if ev.Record.CompletedAt != nil {
				val = int64(ev.Record.CompletedAt.AsTime().Sub(ev.Record.CreatedAt.AsTime()))
			}
		}
		switch sep {
		case ">=":
			return val >= cmp
		case "<=":
			return val <= cmp
		case ">":
			return val > cmp
		default:
			return val < cmp
		}
	}, nil
}

func parseFilter(in string) ([]func(*controlapi.BuildHistoryEvent) bool, error) {
	var out []func(*controlapi.BuildHistoryEvent) bool

	fields, err := csvvalue.Fields(in, nil)
	if err != nil {
		return nil, err
	}
	var staticFilters []string

	for _, f := range fields {
		fn, err := timeBasedFilter(f)
		if err != nil {
			return nil, err
		}
		if fn == nil {
			staticFilters = append(staticFilters, f)
			continue
		}
		out = append(out, fn)
	}

	filter, err := filters.ParseAll(strings.Join(staticFilters, ","))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse history filters %v", in)
	}

	out = append(out, func(ev *controlapi.BuildHistoryEvent) bool {
		if ev.Record == nil {
			return false
		}
		return filter.Match(adaptHistoryRecord(ev.Record))
	})
	return out, nil
}

func adaptHistoryRecord(rec *controlapi.BuildHistoryRecord) filters.Adaptor {
	return filters.AdapterFunc(func(fieldpath []string) (string, bool) {
		if len(fieldpath) == 0 {
			return "", false
		}

		switch fieldpath[0] {
		case "ref":
			return rec.Ref, rec.Ref != ""
		case "status":
			if rec.CompletedAt != nil {
				if rec.Error != nil {
					if strings.Contains(rec.Error.Message, "context canceled") {
						return statusCanceled, true
					}
					return statusError, true
				}
				return statusCompleted, true
			}
			return statusRunning, true
		case "repository":
			v, ok := rec.FrontendAttrs["vcs:source"]
			if ok {
				return v, true
			}
			if context, ok := rec.FrontendAttrs["context"]; ok {
				if parsed, err := gitutil.ParseURL(context); err == nil {
					return parsed.Remote, true
				}
			}
			return "", false
		}
		return "", false
	})
}

func cutAny(in string, opt []string) (before string, sep string, after string, found bool) {
	for _, s := range opt {
		if before, after, ok := strings.Cut(in, s); ok {
			return before, s, after, true
		}
	}
	return "", "", "", false
}
