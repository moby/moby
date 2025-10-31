package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	cerrdefs "github.com/containerd/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type emptyIDError string

func (e emptyIDError) InvalidParameter() {}

func (e emptyIDError) Error() string {
	return "invalid " + string(e) + " name or ID: value is empty"
}

// trimID trims the given object-ID / name, returning an error if it's empty.
func trimID(objType, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", emptyIDError(objType)
	}
	return id, nil
}

// encodePlatforms marshals the given platform(s) to JSON format, to
// be used for query-parameters for filtering / selecting platforms.
func encodePlatforms(platform ...ocispec.Platform) ([]string, error) {
	if len(platform) == 0 {
		return []string{}, nil
	}
	if len(platform) == 1 {
		p, err := encodePlatform(&platform[0])
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	}

	seen := make(map[string]struct{}, len(platform))
	out := make([]string, 0, len(platform))
	for i := range platform {
		p, err := encodePlatform(&platform[i])
		if err != nil {
			return nil, err
		}
		if _, ok := seen[p]; !ok {
			out = append(out, p)
			seen[p] = struct{}{}
		}
	}
	return out, nil
}

// encodePlatform marshals the given platform to JSON format, to
// be used for query-parameters for filtering / selecting platforms. It
// is used as a helper for encodePlatforms,
func encodePlatform(platform *ocispec.Platform) (string, error) {
	p, err := json.Marshal(platform)
	if err != nil {
		return "", fmt.Errorf("%w: invalid platform: %v", cerrdefs.ErrInvalidArgument, err)
	}
	return string(p), nil
}

func decodeWithRaw[T any](resp *http.Response, out *T) (raw json.RawMessage, _ error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("empty response")
	}
	defer ensureReaderClosed(resp)

	var buf bytes.Buffer
	tr := io.TeeReader(resp.Body, &buf)
	err := json.NewDecoder(tr).Decode(out)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// newCancelReadCloser wraps rc so it's automatically closed when ctx is canceled.
// Close is idempotent and returns the first error from rc.Close.
func newCancelReadCloser(ctx context.Context, rc io.ReadCloser) io.ReadCloser {
	crc := &cancelReadCloser{
		rc:    rc,
		close: sync.OnceValue(rc.Close),
	}
	context.AfterFunc(ctx, func() { _ = crc.Close() })
	return crc
}

type cancelReadCloser struct {
	rc    io.ReadCloser
	close func() error
}

func (c *cancelReadCloser) Read(p []byte) (int, error) { return c.rc.Read(p) }
func (c *cancelReadCloser) Close() error               { return c.close() }
