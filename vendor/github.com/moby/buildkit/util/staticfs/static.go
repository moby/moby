package staticfs

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tonistiigi/fsutil"
	"github.com/tonistiigi/fsutil/types"
)

type File struct {
	Stat types.Stat
	Data []byte
}

type FS struct {
	files map[string]File
}

var _ fsutil.FS = &FS{}

func NewFS() *FS {
	return &FS{
		files: map[string]File{},
	}
}

func (fs *FS) Add(p string, stat types.Stat, data []byte) {
	stat.Size_ = int64(len(data))
	if stat.Mode == 0 {
		stat.Mode = 0644
	}
	stat.Path = p
	fs.files[p] = File{
		Stat: stat,
		Data: data,
	}
}

func (fs *FS) Walk(ctx context.Context, fn filepath.WalkFunc) error {
	keys := make([]string, 0, len(fs.files))
	for k := range fs.files {
		keys = append(keys, convertPathToKey(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := convertKeyToPath(k)
		st := fs.files[p].Stat
		if err := fn(p, &fsutil.StatInfo{Stat: &st}, nil); err != nil {
			return err
		}
	}
	return nil
}

func (fs *FS) Open(p string) (io.ReadCloser, error) {
	if f, ok := fs.files[p]; ok {
		return io.NopCloser(bytes.NewReader(f.Data)), nil
	}
	return nil, os.ErrNotExist
}

func convertPathToKey(p string) string {
	return strings.Replace(p, "/", "\x00", -1)
}

func convertKeyToPath(p string) string {
	return strings.Replace(p, "\x00", "/", -1)
}
