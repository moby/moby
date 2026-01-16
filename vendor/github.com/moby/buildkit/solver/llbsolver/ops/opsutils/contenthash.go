package opsutils

import (
	"bytes"
	"context"
	"path"

	"github.com/moby/buildkit/cache/contenthash"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/util/cachedigest"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Selector struct {
	Path            string
	Wildcard        bool
	FollowLinks     bool
	IncludePatterns []string
	ExcludePatterns []string
	RequiredPaths   []string
}

func (sel Selector) HasWildcardOrFilters() bool {
	return sel.Wildcard || len(sel.IncludePatterns) != 0 || len(sel.ExcludePatterns) != 0
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
			eg.Go(func() error {
				dgst, err := contenthash.Checksum(
					ctx, ref.ImmutableRef, path.Join("/", sel.Path),
					contenthash.ChecksumOpts{
						Wildcard:        sel.Wildcard,
						FollowLinks:     sel.FollowLinks,
						IncludePatterns: sel.IncludePatterns,
						ExcludePatterns: sel.ExcludePatterns,
						RequiredPaths:   sel.RequiredPaths,
					},
					s,
				)
				if err != nil {
					return errors.Wrapf(err, "failed to calculate checksum of ref %s", ref.ID())
				}
				dgsts[i] = []byte(dgst)
				return nil
			})
		}

		if err := eg.Wait(); err != nil {
			return "", err
		}

		return cachedigest.FromBytes(bytes.Join(dgsts, []byte{0}), cachedigest.TypeDigestList)
	}
}
