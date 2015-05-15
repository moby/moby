package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
)

type httpBlobUpload struct {
	repo   distribution.Repository
	client *http.Client

	uuid      string
	startedAt time.Time

	location string // always the last value of the location header.
	offset   int64
	closed   bool
}

func (hbu *httpBlobUpload) handleErrorResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusNotFound {
		return &BlobUploadNotFoundError{Location: hbu.location}
	}
	return handleErrorResponse(resp)
}

func (hbu *httpBlobUpload) ReadFrom(r io.Reader) (n int64, err error) {
	req, err := http.NewRequest("PATCH", hbu.location, ioutil.NopCloser(r))
	if err != nil {
		return 0, err
	}
	defer req.Body.Close()

	resp, err := hbu.client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusAccepted {
		return 0, hbu.handleErrorResponse(resp)
	}

	// TODO(dmcgowan): Validate headers
	hbu.uuid = resp.Header.Get("Docker-Upload-UUID")
	hbu.location, err = sanitizeLocation(resp.Header.Get("Location"), hbu.location)
	if err != nil {
		return 0, err
	}
	rng := resp.Header.Get("Range")
	var start, end int64
	if n, err := fmt.Sscanf(rng, "%d-%d", &start, &end); err != nil {
		return 0, err
	} else if n != 2 || end < start {
		return 0, fmt.Errorf("bad range format: %s", rng)
	}

	return (end - start + 1), nil

}

func (hbu *httpBlobUpload) Write(p []byte) (n int, err error) {
	req, err := http.NewRequest("PATCH", hbu.location, bytes.NewReader(p))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Range", fmt.Sprintf("%d-%d", hbu.offset, hbu.offset+int64(len(p)-1)))
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(p)))
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := hbu.client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusAccepted {
		return 0, hbu.handleErrorResponse(resp)
	}

	// TODO(dmcgowan): Validate headers
	hbu.uuid = resp.Header.Get("Docker-Upload-UUID")
	hbu.location, err = sanitizeLocation(resp.Header.Get("Location"), hbu.location)
	if err != nil {
		return 0, err
	}
	rng := resp.Header.Get("Range")
	var start, end int
	if n, err := fmt.Sscanf(rng, "%d-%d", &start, &end); err != nil {
		return 0, err
	} else if n != 2 || end < start {
		return 0, fmt.Errorf("bad range format: %s", rng)
	}

	return (end - start + 1), nil

}

func (hbu *httpBlobUpload) Seek(offset int64, whence int) (int64, error) {
	newOffset := hbu.offset

	switch whence {
	case os.SEEK_CUR:
		newOffset += int64(offset)
	case os.SEEK_END:
		return newOffset, errors.New("Cannot seek from end on incomplete upload")
	case os.SEEK_SET:
		newOffset = int64(offset)
	}

	hbu.offset = newOffset

	return hbu.offset, nil
}

func (hbu *httpBlobUpload) ID() string {
	return hbu.uuid
}

func (hbu *httpBlobUpload) StartedAt() time.Time {
	return hbu.startedAt
}

func (hbu *httpBlobUpload) Commit(ctx context.Context, desc distribution.Descriptor) (distribution.Descriptor, error) {
	// TODO(dmcgowan): Check if already finished, if so just fetch
	req, err := http.NewRequest("PUT", hbu.location, nil)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	values := req.URL.Query()
	values.Set("digest", desc.Digest.String())
	req.URL.RawQuery = values.Encode()

	resp, err := hbu.client.Do(req)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	if resp.StatusCode != http.StatusCreated {
		return distribution.Descriptor{}, hbu.handleErrorResponse(resp)
	}

	return hbu.repo.Blobs(ctx).Stat(ctx, desc.Digest)
}

func (hbu *httpBlobUpload) Cancel(ctx context.Context) error {
	panic("not implemented")
}

func (hbu *httpBlobUpload) Close() error {
	hbu.closed = true
	return nil
}
