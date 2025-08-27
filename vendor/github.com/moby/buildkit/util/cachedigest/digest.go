package cachedigest

import (
	"bytes"
	"encoding/binary"
	"hash"
	"regexp"
	"sync"

	"github.com/moby/buildkit/util/bklog"
	digest "github.com/opencontainers/go-digest"
)

type Type string

const (
	TypeJSON       Type = "json"
	TypeString     Type = "string"
	TypeStringList Type = "string-list"
	TypeDigestList Type = "digest-list"
	TypeFileList   Type = "file-list"
	TypeFile       Type = "file"
)

func (t Type) String() string {
	return string(t)
}

func NewHash(typ Type) *Hash {
	return defaultDB.NewHash(typ)
}

func FromBytes(dt []byte, t Type) (digest.Digest, error) {
	return defaultDB.FromBytes(dt, t)
}

type Hash struct {
	h      hash.Hash
	typ    Type
	db     *DB
	frames []Frame
}

func (h *Hash) Reset() {
	h.h.Reset()
	h.frames = h.frames[:0]
}

func (h *Hash) BlockSize() int {
	return h.h.BlockSize()
}

func (h *Hash) Size() int {
	return h.h.Size()
}

func (h *Hash) Write(p []byte) (n int, err error) {
	n, err = h.h.Write(p)
	if n > 0 && h.db != nil {
		h.frames = append(h.frames, Frame{ID: FrameIDData, Data: bytes.Clone(p[:n])})
	}
	return n, err
}

func (h *Hash) WriteNoDebug(p []byte) (n int, err error) {
	n, err = h.h.Write(p)
	if n > 0 && h.db != nil {
		if len(h.frames) > 0 && h.frames[len(h.frames)-1].ID == FrameIDSkip {
			last := &h.frames[len(h.frames)-1]
			prevLen := binary.LittleEndian.Uint32(last.Data)
			binary.LittleEndian.PutUint32(last.Data, prevLen+uint32(n))
		} else {
			lenBytes := make([]byte, 4)
			binary.LittleEndian.PutUint32(lenBytes, uint32(n))
			h.frames = append(h.frames, Frame{ID: FrameIDSkip, Data: lenBytes})
		}
	}
	return n, err
}

func (h *Hash) Sum() digest.Digest {
	sum := digest.NewDigest(digest.SHA256, h.h)
	if h.db != nil && len(h.frames) > 0 {
		frames := []Frame{
			{ID: FrameIDType, Data: []byte(string(h.typ))},
		}
		frames = append(frames, h.frames...)
		h.db.saveFrames(sum.String(), frames)
	}
	return sum
}

type Record struct {
	Digest     digest.Digest `json:"digest"`
	Type       Type          `json:"type"`
	Data       []Frame       `json:"data,omitempty"`
	SubRecords []*Record     `json:"subRecords,omitempty"`
}

var shaRegexpOnce = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`\bsha256:[a-f0-9]{64}\b`)
})

func (r *Record) LoadSubRecords(loader func(d digest.Digest) (Type, []Frame, error)) error {
	var checksums []string
	var dt []byte

	for _, f := range r.Data {
		if f.ID != FrameIDData {
			continue
		}
		dt = append(dt, f.Data...)
	}
	switch r.Type {
	case TypeString:
		// find regex matches in the data
		matches := shaRegexpOnce().FindAllSubmatch(dt, -1)
		for _, match := range matches {
			if len(match) > 0 {
				checksums = append(checksums, string(match[0]))
			}
		}
	case TypeDigestList:
		for _, dgst := range bytes.Split(dt, []byte{0}) {
			checksums = append(checksums, string(dgst))
		}
	case TypeFileList:
		for _, nameChecksumPair := range bytes.Split(dt, []byte{0}) {
			idx := bytes.LastIndex(nameChecksumPair, []byte("sha256:"))
			if idx < 0 {
				bklog.L.Warnf("invalid file list entry %q, missing sha256 prefix", nameChecksumPair)
				continue
			}
			checksums = append(checksums, string(nameChecksumPair[idx:]))
		}
	}

	dgsts := make([]digest.Digest, 0, len(checksums))
	for _, dgst := range checksums {
		if d, err := digest.Parse(dgst); err == nil {
			dgsts = append(dgsts, d)
		} else {
			bklog.L.Warnf("failed to parse debug info digest %q: %v", dgst, err)
		}
	}
	for _, dgst := range dgsts {
		typ, frames, err := loader(digest.Digest(dgst))
		if err != nil {
			bklog.L.Warnf("failed to load sub-record for %s: %v", dgst, err)
			continue
		}
		rr := &Record{
			Digest: digest.Digest(dgst),
			Type:   typ,
			Data:   frames,
		}
		if err := rr.LoadSubRecords(loader); err != nil {
			return err
		}

		r.SubRecords = append(r.SubRecords, rr)
	}
	return nil
}
