package cache

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"

	cdcompression "github.com/containerd/containerd/archive/compression"
	"github.com/moby/buildkit/session"
)

const keyFileList = "filelist"

// FileList returns an ordered list of files present in the cache record that were
// changed compared to the parent. The paths of the files are in same format as they
// are in the tar stream (AUFS whiteout format). If the reference does not have a
// a blob associated with it, the list is empty.
func (sr *immutableRef) FileList(ctx context.Context, s session.Group) ([]string, error) {
	res, err := g.Do(ctx, fmt.Sprintf("filelist-%s", sr.ID()), func(ctx context.Context) (interface{}, error) {
		dt, err := sr.GetExternal(keyFileList)
		if err == nil && dt != nil {
			var files []string
			if err := json.Unmarshal(dt, &files); err != nil {
				return nil, err
			}
			return files, nil
		}

		if sr.getBlob() == "" {
			return nil, nil
		}

		// lazy blobs need to be pulled first
		if err := sr.Extract(ctx, s); err != nil {
			return nil, err
		}

		desc, err := sr.ociDesc(ctx, sr.descHandlers, false)
		if err != nil {
			return nil, err
		}

		ra, err := sr.cm.ContentStore.ReaderAt(ctx, desc)
		if err != nil {
			return nil, err
		}

		r, err := cdcompression.DecompressStream(io.NewSectionReader(ra, 0, ra.Size()))
		if err != nil {
			return nil, err
		}
		defer r.Close()

		var files []string

		rdr := tar.NewReader(r)
		for {
			hdr, err := rdr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			name := path.Clean(hdr.Name)
			files = append(files, name)
		}
		sort.Strings(files)

		dt, err = json.Marshal(files)
		if err != nil {
			return nil, err
		}
		if err := sr.SetExternal(keyFileList, dt); err != nil {
			return nil, err
		}
		return files, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return res.([]string), nil
}
