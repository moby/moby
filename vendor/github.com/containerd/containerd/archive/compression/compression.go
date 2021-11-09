/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package compression

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/containerd/containerd/log"
	"github.com/klauspost/compress/zstd"
)

type (
	// Compression is the state represents if compressed or not.
	Compression int
)

const (
	// Uncompressed represents the uncompressed.
	Uncompressed Compression = iota
	// Gzip is gzip compression algorithm.
	Gzip
	// Zstd is zstd compression algorithm.
	Zstd
)

const disablePigzEnv = "CONTAINERD_DISABLE_PIGZ"

var (
	initPigz   sync.Once
	unpigzPath string
)

var (
	bufioReader32KPool = &sync.Pool{
		New: func() interface{} { return bufio.NewReaderSize(nil, 32*1024) },
	}
)

// DecompressReadCloser include the stream after decompress and the compress method detected.
type DecompressReadCloser interface {
	io.ReadCloser
	// GetCompression returns the compress method which is used before decompressing
	GetCompression() Compression
}

type readCloserWrapper struct {
	io.Reader
	compression Compression
	closer      func() error
}

func (r *readCloserWrapper) Close() error {
	if r.closer != nil {
		return r.closer()
	}
	return nil
}

func (r *readCloserWrapper) GetCompression() Compression {
	return r.compression
}

type writeCloserWrapper struct {
	io.Writer
	closer func() error
}

func (w *writeCloserWrapper) Close() error {
	if w.closer != nil {
		w.closer()
	}
	return nil
}

type bufferedReader struct {
	buf *bufio.Reader
}

func newBufferedReader(r io.Reader) *bufferedReader {
	buf := bufioReader32KPool.Get().(*bufio.Reader)
	buf.Reset(r)
	return &bufferedReader{buf}
}

func (r *bufferedReader) Read(p []byte) (n int, err error) {
	if r.buf == nil {
		return 0, io.EOF
	}
	n, err = r.buf.Read(p)
	if err == io.EOF {
		r.buf.Reset(nil)
		bufioReader32KPool.Put(r.buf)
		r.buf = nil
	}
	return
}

func (r *bufferedReader) Peek(n int) ([]byte, error) {
	if r.buf == nil {
		return nil, io.EOF
	}
	return r.buf.Peek(n)
}

// DetectCompression detects the compression algorithm of the source.
func DetectCompression(source []byte) Compression {
	for compression, m := range map[Compression][]byte{
		Gzip: {0x1F, 0x8B, 0x08},
		Zstd: {0x28, 0xb5, 0x2f, 0xfd},
	} {
		if len(source) < len(m) {
			// Len too short
			continue
		}
		if bytes.Equal(m, source[:len(m)]) {
			return compression
		}
	}
	return Uncompressed
}

// DecompressStream decompresses the archive and returns a ReaderCloser with the decompressed archive.
func DecompressStream(archive io.Reader) (DecompressReadCloser, error) {
	buf := newBufferedReader(archive)
	bs, err := buf.Peek(10)
	if err != nil && err != io.EOF {
		// Note: we'll ignore any io.EOF error because there are some odd
		// cases where the layer.tar file will be empty (zero bytes) and
		// that results in an io.EOF from the Peek() call. So, in those
		// cases we'll just treat it as a non-compressed stream and
		// that means just create an empty layer.
		// See Issue docker/docker#18170
		return nil, err
	}

	switch compression := DetectCompression(bs); compression {
	case Uncompressed:
		return &readCloserWrapper{
			Reader:      buf,
			compression: compression,
		}, nil
	case Gzip:
		ctx, cancel := context.WithCancel(context.Background())
		gzReader, err := gzipDecompress(ctx, buf)
		if err != nil {
			cancel()
			return nil, err
		}

		return &readCloserWrapper{
			Reader:      gzReader,
			compression: compression,
			closer: func() error {
				cancel()
				return gzReader.Close()
			},
		}, nil
	case Zstd:
		zstdReader, err := zstd.NewReader(buf)
		if err != nil {
			return nil, err
		}
		return &readCloserWrapper{
			Reader:      zstdReader,
			compression: compression,
			closer: func() error {
				zstdReader.Close()
				return nil
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported compression format %s", (&compression).Extension())
	}
}

// CompressStream compresses the dest with specified compression algorithm.
func CompressStream(dest io.Writer, compression Compression) (io.WriteCloser, error) {
	switch compression {
	case Uncompressed:
		return &writeCloserWrapper{dest, nil}, nil
	case Gzip:
		return gzip.NewWriter(dest), nil
	case Zstd:
		return zstd.NewWriter(dest)
	default:
		return nil, fmt.Errorf("unsupported compression format %s", (&compression).Extension())
	}
}

// Extension returns the extension of a file that uses the specified compression algorithm.
func (compression *Compression) Extension() string {
	switch *compression {
	case Gzip:
		return "gz"
	case Zstd:
		return "zst"
	}
	return ""
}

func gzipDecompress(ctx context.Context, buf io.Reader) (io.ReadCloser, error) {
	initPigz.Do(func() {
		if unpigzPath = detectPigz(); unpigzPath != "" {
			log.L.Debug("using pigz for decompression")
		}
	})

	if unpigzPath == "" {
		return gzip.NewReader(buf)
	}

	return cmdStream(exec.CommandContext(ctx, unpigzPath, "-d", "-c"), buf)
}

func cmdStream(cmd *exec.Cmd, in io.Reader) (io.ReadCloser, error) {
	reader, writer := io.Pipe()

	cmd.Stdin = in
	cmd.Stdout = writer

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			writer.CloseWithError(fmt.Errorf("%s: %s", err, errBuf.String()))
		} else {
			writer.Close()
		}
	}()

	return reader, nil
}

func detectPigz() string {
	path, err := exec.LookPath("unpigz")
	if err != nil {
		log.L.WithError(err).Debug("unpigz not found, falling back to go gzip")
		return ""
	}

	// Check if pigz disabled via CONTAINERD_DISABLE_PIGZ env variable
	value := os.Getenv(disablePigzEnv)
	if value == "" {
		return path
	}

	disable, err := strconv.ParseBool(value)
	if err != nil {
		log.L.WithError(err).Warnf("could not parse %s: %s", disablePigzEnv, value)
		return path
	}

	if disable {
		return ""
	}

	return path
}
