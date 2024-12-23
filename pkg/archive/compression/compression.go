package compression

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"sync/atomic"

	"github.com/containerd/log"
	"github.com/docker/docker/pkg/pools"
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
func (compression *Compression) Extension() string {
	switch *compression {
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
	}
	return ""
}

type readCloserWrapper struct {
	io.Reader
	closer func() error
	closed atomic.Bool
}

func (r *readCloserWrapper) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		log.G(context.TODO()).Error("subsequent attempt to close readCloserWrapper")
		if log.GetLevel() >= log.DebugLevel {
			log.G(context.TODO()).Errorf("stack trace: %s", string(debug.Stack()))
		}

		return nil
	}
	return r.closer()
}

// DecompressStream decompresses the archive and returns a ReaderCloser with the decompressed archive.
func DecompressStream(archive io.Reader) (io.ReadCloser, error) {
	p := pools.BufioReader32KPool
	buf := p.Get(archive)
	bs, err := buf.Peek(10)
	if err != nil && err != io.EOF {
		// Note: we'll ignore any io.EOF error because there are some odd
		// cases where the layer.tar file will be empty (zero bytes) and
		// that results in an io.EOF from the Peek() call. So, in those
		// cases we'll just treat it as a non-compressed stream and
		// that means just create an empty layer.
		// See Issue 18170
		return nil, err
	}

	wrapReader := func(r io.Reader, cancel context.CancelFunc) io.ReadCloser {
		return &readCloserWrapper{
			Reader: r,
			closer: func() error {
				if cancel != nil {
					cancel()
				}
				if readCloser, ok := r.(io.ReadCloser); ok {
					readCloser.Close()
				}
				p.Put(buf)
				return nil
			},
		}
	}

	compressionType := Detect(bs)
	switch compressionType {
	case None:
		return wrapReader(buf, nil), nil
	case Gzip:
		ctx, cancel := context.WithCancel(context.Background())

		gzReader, err := gzipDecompress(ctx, buf)
		if err != nil {
			cancel()
			return nil, err
		}
		return wrapReader(gzReader, cancel), nil
	case Bzip2:
		bz2Reader := bzip2.NewReader(buf)
		return wrapReader(bz2Reader, nil), nil
	case Xz:
		ctx, cancel := context.WithCancel(context.Background())

		xzReader, err := xzDecompress(ctx, buf)
		if err != nil {
			cancel()
			return nil, err
		}
		return wrapReader(xzReader, cancel), nil
	case Zstd:
		zstdReader, err := zstd.NewReader(buf)
		if err != nil {
			return nil, err
		}
		return wrapReader(zstdReader, nil), nil
	default:
		return nil, fmt.Errorf("unsupported compression format %s", (&compressionType).Extension())
	}
}

// CompressStream compresses the dest with specified compression algorithm.
func CompressStream(dest io.Writer, comp Compression) (io.WriteCloser, error) {
	p := pools.BufioWriter32KPool
	buf := p.Get(dest)
	switch comp {
	case None:
		writeBufWrapper := p.NewWriteCloserWrapper(buf, buf)
		return writeBufWrapper, nil
	case Gzip:
		gzWriter := gzip.NewWriter(dest)
		writeBufWrapper := p.NewWriteCloserWrapper(buf, gzWriter)
		return writeBufWrapper, nil
	case Bzip2, Xz:
		// archive/bzip2 does not support writing, and there is no xz support at all
		// However, this is not a problem as docker only currently generates gzipped tars
		return nil, fmt.Errorf("unsupported compression format %s", (&comp).Extension())
	default:
		return nil, fmt.Errorf("unsupported compression format %s", (&comp).Extension())
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
func cmdStream(cmd *exec.Cmd, input io.Reader) (io.ReadCloser, error) {
	cmd.Stdin = input
	pipeR, pipeW := io.Pipe()
	cmd.Stdout = pipeW
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
			pipeW.CloseWithError(fmt.Errorf("%s: %s", err, errBuf.String()))
		} else {
			pipeW.Close()
		}
		close(done)
	}()

	return &readCloserWrapper{
		Reader: pipeR,
		closer: func() error {
			// Close pipeR, and then wait for the command to complete before returning. We have to close pipeR first, as
			// cmd.Wait waits for any non-file stdout/stderr/stdin to close.
			err := pipeR.Close()
			<-done
			return err
		},
	}, nil
}
