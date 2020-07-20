package session

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

type Group interface {
	SessionIterator() Iterator
}
type Iterator interface {
	NextSession() string
}

func NewGroup(ids ...string) Group {
	return &group{ids: ids}
}

type group struct {
	ids []string
}

func (g *group) SessionIterator() Iterator {
	return &group{ids: g.ids}
}

func (g *group) NextSession() string {
	if len(g.ids) == 0 {
		return ""
	}
	v := g.ids[0]
	g.ids = g.ids[1:]
	return v
}

func AllSessionIDs(g Group) (out []string) {
	if g == nil {
		return nil
	}
	it := g.SessionIterator()
	if it == nil {
		return nil
	}
	for {
		v := it.NextSession()
		if v == "" {
			return
		}
		out = append(out, v)
	}
}

func (sm *Manager) Any(ctx context.Context, g Group, f func(context.Context, string, Caller) error) error {
	if g == nil {
		return nil
	}

	iter := g.SessionIterator()
	if iter == nil {
		return nil
	}

	var lastErr error
	for {
		id := iter.NextSession()
		if id == "" {
			if lastErr != nil {
				return lastErr
			}
			return errors.Errorf("no active sessions")
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		c, err := sm.Get(timeoutCtx, id)
		if err != nil {
			lastErr = err
			continue
		}
		if err := f(ctx, id, c); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
}
