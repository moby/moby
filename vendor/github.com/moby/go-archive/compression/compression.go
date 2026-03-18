package compression

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/containerd/log"
	"github.com/klauspost/compress/zstd"
)

// Compression is the state represents if compressed or not.
type Compression int

const (
	None  Compression = 0 // None represents the uncompressed.
	Bzip2 Compression = 1 // Bzip2 is bzip2 compression algorithm.
	Gzip  Compression = 2 // Gzip is gzip compression algorithm.
	Xz    Compression = 3 // Xz is xz compression algorithm.
	Zstd  Compression = 4 // Zstd is zstd compression algorithm.
)

// Extension returns the extension of a file that uses the specified compression algorithm.
func (c *Compression) Extension() string {
	switch *c {
	case None:
		return "tar"
	case Bzip2:
		return "tar.bz2"
	case Gzip:
		return "tar.gz"
	case Xz:
		return "tar.xz"
	case Zstd:
		return "tar.zst"
	default:
		return ""
	}
}

type readCloserWrapper struct {
	io.Reader
	closer func() error
}

func (r *readCloserWrapper) Close() error {
	if r.closer != nil {
		return r.closer()
	}
	return nil
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

var bufioReader32KPool = &sync.Pool{
	New: func() interface{} { return bufio.NewReaderSize(nil, 32*1024) },
}

type bufferedReader struct {
	buf *bufio.Reader
}

func newBufferedReader(r io.Reader) *bufferedReader {
	buf := bufioReader32KPool.Get().(*bufio.Reader)
	buf.Reset(r)
	return &bufferedReader{buf}
}

func (r *bufferedReader) Read(p []byte) (int, error) {
	if r.buf == nil {
		return 0, io.EOF
	}
	n, err := r.buf.Read(p)
	if errors.Is(err, io.EOF) {
		r.buf.Reset(nil)
		bufioReader32KPool.Put(r.buf)
		r.buf = nil
	}
	return n, err
}

func (r *bufferedReader) Peek(n int) ([]byte, error) {
	if r.buf == nil {
		return nil, io.EOF
	}
	return r.buf.Peek(n)
}

// DecompressStream decompresses the archive and returns a ReaderCloser with the decompressed archive.
func DecompressStream(archive io.Reader) (io.ReadCloser, error) {
	buf := newBufferedReader(archive)
	bs, err := buf.Peek(10)
	if err != nil && !errors.Is(err, io.EOF) {
		// Note: we'll ignore any io.EOF error because there are some odd
		// cases where the layer.tar file will be empty (zero bytes) and
		// that results in an io.EOF from the Peek() call. So, in those
		// cases we'll just treat it as a non-compressed stream and
		// that means just create an empty layer.
		// See Issue 18170
		return nil, err
	}

	switch compression := Detect(bs); compression {
	case None:
		return &readCloserWrapper{
			Reader: buf,
		}, nil
	case Gzip:
		ctx, cancel := context.WithCancel(context.Background())
		gzReader, err := gzipDecompress(ctx, buf)
		if err != nil {
			cancel()
			return nil, err
		}

		return &readCloserWrapper{
			Reader: gzReader,
			closer: func() error {
				cancel()
				return gzReader.Close()
			},
		}, nil
	case Bzip2:
		bz2Reader := bzip2.NewReader(buf)
		return &readCloserWrapper{
			Reader: bz2Reader,
		}, nil
	case Xz:
		ctx, cancel := context.WithCancel(context.Background())

		xzReader, err := xzDecompress(ctx, buf)
		if err != nil {
			cancel()
			return nil, err
		}

		return &readCloserWrapper{
			Reader: xzReader,
			closer: func() error {
				cancel()
				return xzReader.Close()
			},
		}, nil
	case Zstd:
		zstdReader, err := zstd.NewReader(buf)
		if err != nil {
			return nil, err
		}
		return &readCloserWrapper{
			Reader: zstdReader,
			closer: func() error {
				zstdReader.Close()
				return nil
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported compression format (%d)", compression)
	}
}

// CompressStream compresses the dest with specified compression algorithm.
func CompressStream(dest io.Writer, compression Compression) (io.WriteCloser, error) {
	switch compression {
	case None:
		return nopWriteCloser{dest}, nil
	case Gzip:
		return gzip.NewWriter(dest), nil
	case Bzip2:
		// archive/bzip2 does not support writing.
		return nil, errors.New("unsupported compression format: tar.bz2")
	case Xz:
		// there is no xz support at all
		// However, this is not a problem as docker only currently generates gzipped tars
		return nil, errors.New("unsupported compression format: tar.xz")
	default:
		return nil, fmt.Errorf("unsupported compression format (%d)", compression)
	}
}

func xzDecompress(ctx context.Context, archive io.Reader) (io.ReadCloser, error) {
	args := []string{"xz", "-d", "-c", "-q"}

	return cmdStream(exec.CommandContext(ctx, args[0], args[1:]...), archive)
}

func gzipDecompress(ctx context.Context, buf io.Reader) (io.ReadCloser, error) {
	if noPigzEnv := os.Getenv("MOBY_DISABLE_PIGZ"); noPigzEnv != "" {
		noPigz, err := strconv.ParseBool(noPigzEnv)
		if err != nil {
			log.G(ctx).WithError(err).Warn("invalid value in MOBY_DISABLE_PIGZ env var")
		}
		if noPigz {
			log.G(ctx).Debugf("Use of pigz is disabled due to MOBY_DISABLE_PIGZ=%s", noPigzEnv)
			return gzip.NewReader(buf)
		}
	}

	unpigzPath, err := exec.LookPath("unpigz")
	if err != nil {
		log.G(ctx).Debugf("unpigz binary not found, falling back to go gzip library")
		return gzip.NewReader(buf)
	}

	log.G(ctx).Debugf("Using %s to decompress", unpigzPath)

	return cmdStream(exec.CommandContext(ctx, unpigzPath, "-d", "-c"), buf)
}

// cmdStream executes a command, and returns its stdout as a stream.
// If the command fails to run or doesn't complete successfully, an error
// will be returned, including anything written on stderr.
func cmdStream(cmd *exec.Cmd, in io.Reader) (io.ReadCloser, error) {
	reader, writer := io.Pipe()

	cmd.Stdin = in
	cmd.Stdout = writer

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	// Run the command and return the pipe
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Ensure the command has exited before we clean anything up
	done := make(chan struct{})

	// Copy stdout to the returned pipe
	go func() {
		if err := cmd.Wait(); err != nil {
			_ = writer.CloseWithError(fmt.Errorf("%w: %s", err, errBuf.String()))
		} else {
			_ = writer.Close()
		}
		close(done)
	}()

	return &readCloserWrapper{
		Reader: reader,
		closer: func() error {
			// Close pipeR, and then wait for the command to complete before returning. We have to close pipeR first, as
			// cmd.Wait waits for any non-file stdout/stderr/stdin to close.
			err := reader.Close()
			<-done
			return err
		},
	}, nil
}
