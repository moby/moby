package fsutil

import (
	"context"
	"hash"
	"os"

	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil/types"
)

type walkerFn func(ctx context.Context, pathC chan<- *currentPath) error

func Changes(ctx context.Context, a, b walkerFn, changeFn ChangeFunc) error {
	return nil
}

type HandleChangeFn func(ChangeKind, string, os.FileInfo, error) error

type ContentHasher func(*types.Stat) (hash.Hash, error)

func getWalkerFn(root string) walkerFn {
	return func(ctx context.Context, pathC chan<- *currentPath) error {
		return errors.Wrap(Walk(ctx, root, nil, func(path string, f os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			stat, ok := f.Sys().(*types.Stat)
			if !ok {
				return errors.Errorf("%T invalid file without stat information", f.Sys())
			}

			p := &currentPath{
				path: path,
				stat: stat,
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case pathC <- p:
				return nil
			}
		}), "failed to walk")
	}
}

func emptyWalker(ctx context.Context, pathC chan<- *currentPath) error {
	return nil
}
