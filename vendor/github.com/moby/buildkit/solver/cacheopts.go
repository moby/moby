package solver

import (
	"context"

	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type CacheOpts map[interface{}]interface{}

type cacheOptGetterKey struct{}

func CacheOptGetterOf(ctx context.Context) func(keys ...interface{}) map[interface{}]interface{} {
	if v := ctx.Value(cacheOptGetterKey{}); v != nil {
		if getter, ok := v.(func(keys ...interface{}) map[interface{}]interface{}); ok {
			return getter
		}
	}
	return nil
}

func withAncestorCacheOpts(ctx context.Context, start *state) context.Context {
	return context.WithValue(ctx, cacheOptGetterKey{}, func(keys ...interface{}) map[interface{}]interface{} {
		keySet := make(map[interface{}]struct{})
		for _, k := range keys {
			keySet[k] = struct{}{}
		}
		values := make(map[interface{}]interface{})
		walkAncestors(start, func(st *state) bool {
			if st.clientVertex.Error != "" {
				// don't use values from cancelled or otherwise error'd vertexes
				return false
			}
			for _, res := range st.op.cacheRes {
				if res.Opts == nil {
					continue
				}
				for k := range keySet {
					if v, ok := res.Opts[k]; ok {
						values[k] = v
						delete(keySet, k)
						if len(keySet) == 0 {
							return true
						}
					}
				}
			}
			return false
		})
		return values
	})
}

func walkAncestors(start *state, f func(*state) bool) {
	stack := [][]*state{{start}}
	cache := make(map[digest.Digest]struct{})
	for len(stack) > 0 {
		sts := stack[len(stack)-1]
		if len(sts) == 0 {
			stack = stack[:len(stack)-1]
			continue
		}
		st := sts[len(sts)-1]
		stack[len(stack)-1] = sts[:len(sts)-1]
		if st == nil {
			continue
		}
		if _, ok := cache[st.origDigest]; ok {
			continue
		}
		cache[st.origDigest] = struct{}{}
		if shouldStop := f(st); shouldStop {
			return
		}
		stack = append(stack, []*state{})
		for _, parentDgst := range st.clientVertex.Inputs {
			st.solver.mu.RLock()
			parent := st.solver.actives[parentDgst]
			st.solver.mu.RUnlock()
			if parent == nil {
				logrus.Warnf("parent %q not found in active job list during cache opt search", parentDgst)
				continue
			}
			stack[len(stack)-1] = append(stack[len(stack)-1], parent)
		}
	}
}
