package cache

import (
	"context"
	"io"
	"io/ioutil"
	"os"

	"github.com/containerd/continuity/fs"
	"github.com/moby/buildkit/snapshot"
)

type ReadRequest struct {
	Filename string
	Range    *FileRange
}

type FileRange struct {
	Offset int
	Length int
}

func ReadFile(ctx context.Context, ref ImmutableRef, req ReadRequest) ([]byte, error) {
	mount, err := ref.Mount(ctx, true)
	if err != nil {
		return nil, err
	}

	lm := snapshot.LocalMounter(mount)

	root, err := lm.Mount()
	if err != nil {
		return nil, err
	}

	defer func() {
		if lm != nil {
			lm.Unmount()
		}
	}()

	fp, err := fs.RootPath(root, req.Filename)
	if err != nil {
		return nil, err
	}

	var dt []byte

	if req.Range == nil {
		dt, err = ioutil.ReadFile(fp)
		if err != nil {
			return nil, err
		}
	} else {
		f, err := os.Open(fp)
		if err != nil {
			return nil, err
		}
		dt, err = ioutil.ReadAll(io.NewSectionReader(f, int64(req.Range.Offset), int64(req.Range.Length)))
		f.Close()
		if err != nil {
			return nil, err
		}
	}

	if err := lm.Unmount(); err != nil {
		return nil, err
	}
	lm = nil
	return dt, err
}
