package llbsolver

import (
	"bytes"
	"context"
	"path"

	"github.com/moby/buildkit/cache/contenthash"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/compression"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Selector struct {
	Path        string
	Wildcard    bool
	FollowLinks bool
}

func UnlazyResultFunc(ctx context.Context, res solver.Result, g session.Group) error {
	ref, ok := res.Sys().(*worker.WorkerRef)
	if !ok {
		return errors.Errorf("invalid reference: %T", res)
	}
	if ref.ImmutableRef == nil {
		return nil
	}
	return ref.ImmutableRef.Extract(ctx, g)
}

func NewContentHashFunc(selectors []Selector) solver.ResultBasedCacheFunc {
	return func(ctx context.Context, res solver.Result, s session.Group) (digest.Digest, error) {
		ref, ok := res.Sys().(*worker.WorkerRef)
		if !ok {
			return "", errors.Errorf("invalid reference: %T", res)
		}

		if len(selectors) == 0 {
			selectors = []Selector{{}}
		}

		dgsts := make([][]byte, len(selectors))

		eg, ctx := errgroup.WithContext(ctx)

		for i, sel := range selectors {
			i, sel := i, sel
			eg.Go(func() error {
				if !sel.Wildcard {
					dgst, err := contenthash.Checksum(ctx, ref.ImmutableRef, path.Join("/", sel.Path), sel.FollowLinks, s)
					if err != nil {
						return err
					}
					dgsts[i] = []byte(dgst)
				} else {
					dgst, err := contenthash.ChecksumWildcard(ctx, ref.ImmutableRef, path.Join("/", sel.Path), sel.FollowLinks, s)
					if err != nil {
						return err
					}
					dgsts[i] = []byte(dgst)
				}
				return nil
			})
		}

		if err := eg.Wait(); err != nil {
			return "", err
		}

		return digest.FromBytes(bytes.Join(dgsts, []byte{0})), nil
	}
}

func workerRefConverter(g session.Group) func(ctx context.Context, res solver.Result) (*solver.Remote, error) {
	return func(ctx context.Context, res solver.Result) (*solver.Remote, error) {
		ref, ok := res.Sys().(*worker.WorkerRef)
		if !ok {
			return nil, errors.Errorf("invalid result: %T", res.Sys())
		}

		return ref.GetRemote(ctx, true, compression.Default, g)
	}
}
