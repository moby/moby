package contenthash

import (
	"archive/tar"
	"crypto/sha256"
	"hash"
	"os"
	"path/filepath"
	"time"

	fstypes "github.com/tonistiigi/fsutil/types"
)

// NewFileHash returns new hash that is used for the builder cache keys
func NewFileHash(path string, fi os.FileInfo) (hash.Hash, error) {
	var link string
	if fi.Mode()&os.ModeSymlink != 0 {
		var err error
		link, err = os.Readlink(path)
		if err != nil {
			return nil, err
		}
	}

	stat := &fstypes.Stat{
		Mode:     uint32(fi.Mode()),
		Size_:    fi.Size(),
		ModTime:  fi.ModTime().UnixNano(),
		Linkname: link,
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		stat.Mode = stat.Mode | 0777
	}

	if err := setUnixOpt(path, fi, stat); err != nil {
		return nil, err
	}
	return NewFromStat(stat)
}

func NewFromStat(stat *fstypes.Stat) (hash.Hash, error) {
	// Clear the socket bit since archive/tar.FileInfoHeader does not handle it
	stat.Mode &^= uint32(os.ModeSocket)

	fi := &statInfo{stat}
	hdr, err := tar.FileInfoHeader(fi, stat.Linkname)
	if err != nil {
		return nil, err
	}
	hdr.Name = "" // note: empty name is different from current has in docker build. Name is added on recursive directory scan instead
	hdr.Mode = int64(chmodWindowsTarEntry(os.FileMode(hdr.Mode)))
	hdr.Devmajor = stat.Devmajor
	hdr.Devminor = stat.Devminor

	if len(stat.Xattrs) > 0 {
		hdr.Xattrs = make(map[string]string, len(stat.Xattrs))
		for k, v := range stat.Xattrs {
			hdr.Xattrs[k] = string(v)
		}
	}
	// fmt.Printf("hdr: %#v\n", hdr)
	tsh := &tarsumHash{hdr: hdr, Hash: sha256.New()}
	tsh.Reset() // initialize header
	return tsh, nil
}

type tarsumHash struct {
	hash.Hash
	hdr *tar.Header
}

// Reset resets the Hash to its initial state.
func (tsh *tarsumHash) Reset() {
	// comply with hash.Hash and reset to the state hash had before any writes
	tsh.Hash.Reset()
	WriteV1TarsumHeaders(tsh.hdr, tsh.Hash)
}

type statInfo struct {
	*fstypes.Stat
}

func (s *statInfo) Name() string {
	return filepath.Base(s.Stat.Path)
}
func (s *statInfo) Size() int64 {
	return s.Stat.Size_
}
func (s *statInfo) Mode() os.FileMode {
	return os.FileMode(s.Stat.Mode)
}
func (s *statInfo) ModTime() time.Time {
	return time.Unix(s.Stat.ModTime/1e9, s.Stat.ModTime%1e9)
}
func (s *statInfo) IsDir() bool {
	return s.Mode().IsDir()
}
func (s *statInfo) Sys() interface{} {
	return s.Stat
}
