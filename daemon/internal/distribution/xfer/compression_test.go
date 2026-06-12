package xfer

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"reflect"
	"testing"

	kzstd "github.com/klauspost/compress/zstd"
	"github.com/moby/go-archive/compression"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// withFastGzipDecompression enables the klauspost dispatcher and
// restores the previous value when the test ends.
func withFastGzipDecompression(t *testing.T) {
	t.Helper()
	prev := decompressStream
	EnableFastGzipDecompression(context.Background())
	t.Cleanup(func() { decompressStream = prev })
}

// resetDecompressStream forces the dispatcher back to the default for
// the duration of the test.
func resetDecompressStream(t *testing.T) {
	t.Helper()
	prev := decompressStream
	decompressStream = compression.DecompressStream
	t.Cleanup(func() { decompressStream = prev })
}

func TestDecompressStreamDefaultsToGoArchive(t *testing.T) {
	resetDecompressStream(t)
	got := reflect.ValueOf(decompressStream).Pointer()
	want := reflect.ValueOf(compression.DecompressStream).Pointer()
	assert.Equal(t, got, want)
}

func TestEnableFastGzipDecompressionSwapsDispatcher(t *testing.T) {
	withFastGzipDecompression(t)
	got := reflect.ValueOf(decompressStream).Pointer()
	want := reflect.ValueOf(fastDecompressStream).Pointer()
	assert.Equal(t, got, want)
}

// TestFastDecompressStreamRoundTrip covers the gzip path that this PR
// adds and two delegation paths (zstd, uncompressed) that must keep
// working unchanged.
func TestFastDecompressStreamRoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 1024)

	tests := []struct {
		name string
		enc  func(t *testing.T, p []byte) []byte
	}{
		{
			name: "gzip",
			enc: func(t *testing.T, p []byte) []byte {
				var buf bytes.Buffer
				w := gzip.NewWriter(&buf)
				_, err := w.Write(p)
				assert.NilError(t, err)
				assert.NilError(t, w.Close())
				return buf.Bytes()
			},
		},
		{
			name: "zstd_delegation",
			enc: func(t *testing.T, p []byte) []byte {
				var buf bytes.Buffer
				w, err := kzstd.NewWriter(&buf)
				assert.NilError(t, err)
				_, err = w.Write(p)
				assert.NilError(t, err)
				assert.NilError(t, w.Close())
				return buf.Bytes()
			},
		},
		{
			name: "uncompressed_delegation",
			enc:  func(_ *testing.T, p []byte) []byte { return p },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded := tc.enc(t, payload)
			rc, err := fastDecompressStream(bytes.NewReader(encoded))
			assert.NilError(t, err)
			defer rc.Close()

			got, err := io.ReadAll(rc)
			assert.NilError(t, err)
			assert.Assert(t, is.DeepEqual(payload, got))
		})
	}
}

// TestFastDecompressStreamMatchesDefault pins the safety guarantee that
// the dispatcher swap does not change layer digests.
func TestFastDecompressStreamMatchesDefault(t *testing.T) {
	payload := bytes.Repeat([]byte("docker layer payload bytes\n"), 4096)

	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, err := w.Write(payload)
	assert.NilError(t, err)
	assert.NilError(t, w.Close())
	encoded := gz.Bytes()

	defaultRC, err := compression.DecompressStream(bytes.NewReader(encoded))
	assert.NilError(t, err)
	defer defaultRC.Close()
	defaultOut, err := io.ReadAll(defaultRC)
	assert.NilError(t, err)

	fastRC, err := fastDecompressStream(bytes.NewReader(encoded))
	assert.NilError(t, err)
	defer fastRC.Close()
	fastOut, err := io.ReadAll(fastRC)
	assert.NilError(t, err)

	assert.Assert(t, is.DeepEqual(defaultOut, fastOut))
}

// TestFastDecompressStreamErrorCompat covers a few malformed inputs and
// asserts that fastDecompressStream's success/failure pattern matches
// compression.DecompressStream. The exact error wording is not pinned;
// only whether the operation fails.
func TestFastDecompressStreamErrorCompat(t *testing.T) {
	var fullGz bytes.Buffer
	w := gzip.NewWriter(&fullGz)
	_, err := w.Write(bytes.Repeat([]byte("abc"), 1024))
	assert.NilError(t, err)
	assert.NilError(t, w.Close())
	truncated := fullGz.Bytes()[:len(fullGz.Bytes())/2]

	cases := []struct {
		name  string
		input []byte
	}{
		{"truncated_gzip", truncated},
		{"corrupt_gzip_header", []byte{0x1f, 0x8b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{"empty", nil},
		{"sub_peek", []byte{0x00, 0x01, 0x02}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defaultErr := runAndDiscard(compression.DecompressStream, tc.input)
			fastErr := runAndDiscard(fastDecompressStream, tc.input)
			assert.Equal(t, defaultErr == nil, fastErr == nil,
				"fast=%v default=%v", fastErr, defaultErr)
		})
	}
}

func runAndDiscard(dec func(io.Reader) (io.ReadCloser, error), input []byte) error {
	rc, err := dec(bytes.NewReader(input))
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(io.Discard, rc)
	return err
}
