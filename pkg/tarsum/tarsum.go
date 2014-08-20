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

type TarSum struct {
	io.Reader
	tarR               *tar.Reader
	tarW               *tar.Writer
	gz                 writeCloseFlusher
	bufTar             *bytes.Buffer
	bufGz              *bytes.Buffer
	bufData            [8192]byte
	h                  hash.Hash
	sums               map[string]string
	currentFile        string
	finished           bool
	first              bool
	DisableCompression bool
}

type writeCloseFlusher interface {
	io.WriteCloser
	Flush() error
}

type nopCloseFlusher struct {
	io.Writer
}

func (n *nopCloseFlusher) Close() error {
	return nil
}

func (n *nopCloseFlusher) Flush() error {
	return nil
}

func (ts *TarSum) encodeHeader(h *tar.Header) error {
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
		// {"atime", strconv.Itoa(int(h.AccessTime.UTC().Unix()))},
		// {"ctime", strconv.Itoa(int(h.ChangeTime.UTC().Unix()))},
	} {
		if _, err := ts.h.Write([]byte(elem[0] + elem[1])); err != nil {
			return err
		}
	}
	return nil
}

func (ts *TarSum) Read(buf []byte) (int, error) {
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
	var buf2 []byte
	if len(buf) > 8192 {
		buf2 = make([]byte, len(buf), cap(buf))
	} else {
		buf2 = ts.bufData[:len(buf)-1]
	}

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

func (ts *TarSum) Sum(extra []byte) string {
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
		log.Infof("-->%s<--", sum)
		h.Write([]byte(sum))
	}
	checksum := "tarsum+sha256:" + hex.EncodeToString(h.Sum(nil))
	log.Infof("checksum processed: %s", checksum)
	return checksum
}

func (ts *TarSum) GetSums() map[string]string {
	return ts.sums
}
