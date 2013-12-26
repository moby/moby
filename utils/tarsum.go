package utils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"sort"
	"strconv"
)

type TarSum struct {
	io.Reader
	tarR     *tar.Reader
	tarW     *tar.Writer
	gz       *gzip.Writer
	bufTar   *bytes.Buffer
	bufGz    *bytes.Buffer
	h        hash.Hash
	sums     []string
	finished bool
	first    bool
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
		ts.gz = gzip.NewWriter(ts.bufGz)
		ts.h = sha256.New()
		ts.h.Reset()
		ts.first = true
	}

	if ts.finished {
		return ts.bufGz.Read(buf)
	}
	buf2 := make([]byte, len(buf), cap(buf))

	n, err := ts.tarR.Read(buf2)
	if err != nil {
		if err == io.EOF {
			if _, err := ts.h.Write(buf2[:n]); err != nil {
				return 0, err
			}
			if !ts.first {
				ts.sums = append(ts.sums, hex.EncodeToString(ts.h.Sum(nil)))
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
	sort.Strings(ts.sums)
	h := sha256.New()
	if extra != nil {
		h.Write(extra)
	}
	for _, sum := range ts.sums {
		Debugf("-->%s<--", sum)
		h.Write([]byte(sum))
	}
	checksum := "tarsum+sha256:" + hex.EncodeToString(h.Sum(nil))
	Debugf("checksum processed: %s", checksum)
	return checksum
}
