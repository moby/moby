package fileoptypes

import (
	"context"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver/pb"
)

type Ref interface {
	Release(context.Context) error
}

type Mount interface {
	IsFileOpMount()
	Release(context.Context) error
	Readonly() bool
}

type Backend interface {
	Mkdir(context.Context, Mount, Mount, Mount, pb.FileActionMkDir) error
	Mkfile(context.Context, Mount, Mount, Mount, pb.FileActionMkFile) error
	Rm(context.Context, Mount, pb.FileActionRm) error
	Copy(context.Context, Mount, Mount, Mount, Mount, pb.FileActionCopy) error
}

type RefManager interface {
	Prepare(ctx context.Context, ref Ref, readonly bool, g session.Group) (Mount, error)
	Commit(ctx context.Context, mount Mount) (Ref, error)
}
