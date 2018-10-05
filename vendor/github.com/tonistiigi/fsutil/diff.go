package fsutil

import (
	"context"
	"hash"
	"os"

	"github.com/tonistiigi/fsutil/types"
)

type walkerFn func(ctx context.Context, pathC chan<- *currentPath) error

func Changes(ctx context.Context, a, b walkerFn, changeFn ChangeFunc) error {
	return nil
}

type HandleChangeFn func(ChangeKind, string, os.FileInfo, error) error

type ContentHasher func(*types.Stat) (hash.Hash, error)

func GetWalkerFn(root string) walkerFn {
	return func(ctx context.Context, pathC chan<- *currentPath) error {
		return Walk(ctx, root, nil, func(path string, f os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			p := &currentPath{
				path: path,
				f:    f,
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case pathC <- p:
				return nil
			}
		})
	}
}

func emptyWalker(ctx context.Context, pathC chan<- *currentPath) error {
	return nil
}
