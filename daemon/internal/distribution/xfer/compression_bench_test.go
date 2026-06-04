package xfer

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"testing"

	"github.com/docker/go-units"
	kgzip "github.com/klauspost/compress/gzip"
)

// BenchmarkDecompressStream compares three gzip decoders on the layer
// pull path: stdlib compress/gzip, klauspost/compress/gzip (this PR),
// and unpigz (the existing subprocess fallback in go-archive). Opt-in
// via -bench. The pigz row is skipped when unpigz is not on PATH.
//
// Payload size from MOBY_BENCH_PAYLOAD_SIZE (default 10 MiB,
// units.FromHumanSize syntax).
//
//	go test -bench=BenchmarkDecompressStream -benchmem -run=^$ \
//	    ./daemon/internal/distribution/xfer/
func BenchmarkDecompressStream(b *testing.B) {
	size := benchPayloadSize(b)
	payload := benchPayload(size)
	encoded := encodeGzipForBench(b, payload)
	ratio := float64(len(encoded)) * 100 / float64(len(payload))
	b.Logf("gzip: %d bytes payload compressed to %d bytes (%.1f%% ratio)",
		len(payload), len(encoded), ratio)

	b.Run("gzip/stdlib", func(b *testing.B) {
		runDecoder(b, payload, encoded, decodeStdlib)
	})
	b.Run("gzip/klauspost", func(b *testing.B) {
		runDecoder(b, payload, encoded, decodeKlauspost)
	})
	b.Run("gzip/pigz", func(b *testing.B) {
		if _, err := exec.LookPath("unpigz"); err != nil {
			b.Skip("unpigz not on PATH")
		}
		runDecoder(b, payload, encoded, decodePigz)
	})
}

func runDecoder(b *testing.B, payload, encoded []byte, dec func(io.Reader) (io.ReadCloser, error)) {
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		rc, err := dec(bytes.NewReader(encoded))
		if err != nil {
			b.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, rc); err != nil {
			b.Fatal(err)
		}
		_ = rc.Close()
	}
}

func decodeStdlib(r io.Reader) (io.ReadCloser, error) { return gzip.NewReader(r) }
func decodeKlauspost(r io.Reader) (io.ReadCloser, error) {
	return kgzip.NewReader(r)
}

// decodePigz spawns unpigz as a subprocess, the same shape as
// moby/go-archive's gzipDecompress uses internally.
func decodePigz(r io.Reader) (io.ReadCloser, error) {
	cmd := exec.CommandContext(context.Background(), "unpigz", "-d", "-c")
	cmd.Stdin = r
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &pigzReadCloser{ReadCloser: stdout, cmd: cmd}, nil
}

type pigzReadCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (p *pigzReadCloser) Close() error {
	cerr := p.ReadCloser.Close()
	werr := p.cmd.Wait()
	if cerr != nil {
		return cerr
	}
	return werr
}

func benchPayloadSize(b *testing.B) int {
	s := os.Getenv("MOBY_BENCH_PAYLOAD_SIZE")
	if s == "" {
		return 10 << 20
	}
	n, err := units.FromHumanSize(s)
	if err != nil || n <= 0 {
		b.Fatalf("invalid MOBY_BENCH_PAYLOAD_SIZE %q: %v", s, err)
	}
	return int(n)
}

// benchPayload returns deterministic bytes shaped roughly like a real
// container layer: structured text (compresses well) interleaved with
// pseudo-random noise (does not).
func benchPayload(size int) []byte {
	const block = "moby docker container layer payload " +
		"the quick brown fox jumps over the lazy dog " +
		"package metadata config file source listing\n"
	out := make([]byte, 0, size)
	rng := rand.New(rand.NewSource(1))
	for len(out) < size {
		textChunk := 16 << 10
		noiseChunk := 4 << 10
		for textChunk > 0 && len(out) < size {
			n := min(textChunk, size-len(out))
			for i := 0; i < n; {
				w := copy(out[len(out):len(out)+min(n-i, len(block))], block)
				out = out[:len(out)+w]
				i += w
			}
			textChunk -= n
		}
		if len(out) >= size {
			break
		}
		noise := make([]byte, min(noiseChunk, size-len(out)))
		_, _ = rng.Read(noise)
		out = append(out, noise...)
	}
	return out[:size]
}

func encodeGzipForBench(b *testing.B, p []byte) []byte {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(p); err != nil {
		b.Fatal(err)
	}
	if err := w.Close(); err != nil {
		b.Fatal(err)
	}
	return buf.Bytes()
}
