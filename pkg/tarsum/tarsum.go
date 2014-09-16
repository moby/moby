package tarsum

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/docker/docker/vendor/src/code.google.com/p/go/src/pkg/archive/tar"

	"github.com/docker/docker/pkg/log"
)

const (
	buf8K  = 8 * 1024
	buf16K = 16 * 1024
	buf32K = 32 * 1024
)

// NewTarSum creates a new interface for calculating a fixed time checksum of a
// tar archive.
//
// This is used for calculating checksums of layers of an image, in some cases
// including the byte payload of the image's json metadata as well, and for
// calculating the checksums for buildcache.
func NewTarSum(r io.Reader, dc bool, v Version) (TarSum, error) {
	if _, ok := tarSumVersions[v]; !ok {
		return nil, ErrVersionNotImplemented
	}
	return &tarSum{Reader: r, DisableCompression: dc, tarSumVersion: v}, nil
}

// TarSum is the generic interface for calculating fixed time
// checksums of a tar archive
type TarSum interface {
	io.Reader
	GetSums() map[string]string
	Sum([]byte) string
	Version() Version
}

// tarSum struct is the structure for a Version0 checksum calculation
type tarSum struct {
	io.Reader
	tarR               *tar.Reader
	tarW               *tar.Writer
	gz                 writeCloseFlusher
	bufTar             *bytes.Buffer
	bufGz              *bytes.Buffer
	bufData            []byte
	h                  hash.Hash
	sums               map[string]string
	currentFile        string
	finished           bool
	first              bool
	DisableCompression bool    // false by default. When false, the output gzip compressed.
	tarSumVersion      Version // this field is not exported so it can not be mutated during use
}

func (ts tarSum) Version() Version {
	return ts.tarSumVersion
}

func (ts tarSum) selectHeaders(h *tar.Header, v Version) (set [][2]string) {
	for _, elem := range [][2]string{
		{"name", h.Name},
		{"mode", strconv.Itoa(int(h.Mode))},
		{"uid", strconv.Itoa(h.Uid)},
		{"gid", strconv.Itoa(h.Gid)},
		{"size", strconv.Itoa(int(h.Size))},
		{"mtime", strconv.Itoa(int(h.ModTime.UTC().Unix()))},
		{"typeflag", string([]byte{h.Typeflag})},
		{"linkname", h.Linkname},
		{"uname", h.Uname},
		{"gname", h.Gname},
		{"devmajor", strconv.Itoa(int(h.Devmajor))},
		{"devminor", strconv.Itoa(int(h.Devminor))},
	} {
		if v == VersionDev && elem[0] == "mtime" {
			continue
		}
		set = append(set, elem)
	}
	return
}

func (ts *tarSum) encodeHeader(h *tar.Header) error {
	for _, elem := range ts.selectHeaders(h, ts.Version()) {
		if _, err := ts.h.Write([]byte(elem[0] + elem[1])); err != nil {
			return err
		}
	}
	return nil
}

func (ts *tarSum) Read(buf []byte) (int, error) {
	if ts.gz == nil {
		ts.bufTar = bytes.NewBuffer([]byte{})
		ts.bufGz = bytes.NewBuffer([]byte{})
		ts.tarR = tar.NewReader(ts.Reader)
		ts.tarW = tar.NewWriter(ts.bufTar)
		if !ts.DisableCompression {
			ts.gz = gzip.NewWriter(ts.bufGz)
		} else {
			ts.gz = &nopCloseFlusher{Writer: ts.bufGz}
		}
		ts.h = sha256.New()
		ts.h.Reset()
		ts.first = true
		ts.sums = make(map[string]string)
	}

	if ts.finished {
		return ts.bufGz.Read(buf)
	}
	if ts.bufData == nil {
		switch {
		case len(buf) <= buf8K:
			ts.bufData = make([]byte, buf8K)
		case len(buf) <= buf16K:
			ts.bufData = make([]byte, buf16K)
		case len(buf) <= buf32K:
			ts.bufData = make([]byte, buf32K)
		default:
			ts.bufData = make([]byte, len(buf))
		}
	}
	buf2 := ts.bufData[:len(buf)-1]

	n, err := ts.tarR.Read(buf2)
	if err != nil {
		if err == io.EOF {
			if _, err := ts.h.Write(buf2[:n]); err != nil {
				return 0, err
			}
			if !ts.first {
				ts.sums[ts.currentFile] = hex.EncodeToString(ts.h.Sum(nil))
				ts.h.Reset()
			} else {
				ts.first = false
			}

			currentHeader, err := ts.tarR.Next()
			if err != nil {
				if err == io.EOF {
					if err := ts.tarW.Close(); err != nil {
						return 0, err
					}
					if _, err := io.Copy(ts.gz, ts.bufTar); err != nil {
						return 0, err
					}
					if err := ts.gz.Close(); err != nil {
						return 0, err
					}
					ts.finished = true
					return n, nil
				}
				return n, err
			}
			ts.currentFile = strings.TrimSuffix(strings.TrimPrefix(currentHeader.Name, "./"), "/")
			if err := ts.encodeHeader(currentHeader); err != nil {
				return 0, err
			}
			if err := ts.tarW.WriteHeader(currentHeader); err != nil {
				return 0, err
			}
			if _, err := ts.tarW.Write(buf2[:n]); err != nil {
				return 0, err
			}
			ts.tarW.Flush()
			if _, err := io.Copy(ts.gz, ts.bufTar); err != nil {
				return 0, err
			}
			ts.gz.Flush()

			return ts.bufGz.Read(buf)
		}
		return n, err
	}

	// Filling the hash buffer
	if _, err = ts.h.Write(buf2[:n]); err != nil {
		return 0, err
	}

	// Filling the tar writter
	if _, err = ts.tarW.Write(buf2[:n]); err != nil {
		return 0, err
	}
	ts.tarW.Flush()

	// Filling the gz writter
	if _, err = io.Copy(ts.gz, ts.bufTar); err != nil {
		return 0, err
	}
	ts.gz.Flush()

	return ts.bufGz.Read(buf)
}

func (ts *tarSum) Sum(extra []byte) string {
	var sums []string

	for _, sum := range ts.sums {
		sums = append(sums, sum)
	}
	sort.Strings(sums)
	h := sha256.New()
	if extra != nil {
		h.Write(extra)
	}
	for _, sum := range sums {
		log.Debugf("-->%s<--", sum)
		h.Write([]byte(sum))
	}
	checksum := ts.Version().String() + "+sha256:" + hex.EncodeToString(h.Sum(nil))
	log.Debugf("checksum processed: %s", checksum)
	return checksum
}

func (ts *tarSum) GetSums() map[string]string {
	return ts.sums
}
