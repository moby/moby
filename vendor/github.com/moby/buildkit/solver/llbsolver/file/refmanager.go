package file

import (
	"context"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/solver/llbsolver/ops/fileoptypes"
	"github.com/moby/buildkit/util/bklog"
	"github.com/pkg/errors"
)

func NewRefManager(cm cache.Manager, name string) *RefManager {
	return &RefManager{cm: cm, desc: name}
}

type RefManager struct {
	cm   cache.Manager
	desc string
}

func (rm *RefManager) Prepare(ctx context.Context, ref fileoptypes.Ref, readonly bool, g session.Group) (_ fileoptypes.Mount, rerr error) {
	ir, ok := ref.(cache.ImmutableRef)
	if !ok && ref != nil {
		return nil, errors.Errorf("invalid ref type: %T", ref)
	}

	if ir != nil && readonly {
		m, err := ir.Mount(ctx, readonly, g)
		if err != nil {
			return nil, err
		}
		return &Mount{m: m, readonly: readonly}, nil
	}

	desc := "fileop target"

	if d := rm.desc; d != "" {
		desc = d
	}

	mr, err := rm.cm.New(ctx, ir, g, cache.WithDescription(desc), cache.CachePolicyRetain)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rerr != nil {
			if err := mr.SetCachePolicyDefault(); err != nil {
				bklog.G(ctx).Errorf("failed to reset FileOp mutable ref cachepolicy: %v", err)
			}
			mr.Release(context.TODO())
		}
	}()
	m, err := mr.Mount(ctx, readonly, g)
	if err != nil {
		return nil, err
	}
	return &Mount{m: m, mr: mr, readonly: readonly}, nil
}

func (rm *RefManager) Commit(ctx context.Context, mount fileoptypes.Mount) (fileoptypes.Ref, error) {
	m, ok := mount.(*Mount)
	if !ok {
		return nil, errors.Errorf("invalid mount type %T", mount)
	}
	if m.mr == nil {
		return nil, errors.Errorf("invalid mount without active ref for commit")
	}
	defer func() {
		m.mr = nil
	}()
	return m.mr.Commit(ctx)
}

type Mount struct {
	m        snapshot.Mountable
	mr       cache.MutableRef
	readonly bool
}

func (m *Mount) Mountable() snapshot.Mountable {
	return m.m
}

func (m *Mount) Release(ctx context.Context) error {
	if m.mr != nil {
		return m.mr.Release(ctx)
	}
	return nil
}
func (m *Mount) IsFileOpMount() {}

func (m *Mount) Readonly() bool {
	return m.readonly
}
