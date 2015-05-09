package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
)

type httpLayerUpload struct {
	repo   distribution.Repository
	client *http.Client

	uuid      string
	startedAt time.Time

	location string // always the last value of the location header.
	offset   int64
	closed   bool
}

func (hlu *httpLayerUpload) handleErrorResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusNotFound {
		return &BlobUploadNotFoundError{Location: hlu.location}
	}
	return handleErrorResponse(resp)
}

func (hlu *httpLayerUpload) ReadFrom(r io.Reader) (n int64, err error) {
	req, err := http.NewRequest("PATCH", hlu.location, r)
	if err != nil {
		return 0, err
	}
	defer req.Body.Close()

	resp, err := hlu.client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusAccepted {
		return 0, hlu.handleErrorResponse(resp)
	}

	// TODO(dmcgowan): Validate headers
	hlu.uuid = resp.Header.Get("Docker-Upload-UUID")
	hlu.location, err = sanitizeLocation(resp.Header.Get("Location"), hlu.location)
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

func (hlu *httpLayerUpload) Write(p []byte) (n int, err error) {
	req, err := http.NewRequest("PATCH", hlu.location, bytes.NewReader(p))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Range", fmt.Sprintf("%d-%d", hlu.offset, hlu.offset+int64(len(p)-1)))
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(p)))
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := hlu.client.Do(req)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusAccepted {
		return 0, hlu.handleErrorResponse(resp)
	}

	// TODO(dmcgowan): Validate headers
	hlu.uuid = resp.Header.Get("Docker-Upload-UUID")
	hlu.location, err = sanitizeLocation(resp.Header.Get("Location"), hlu.location)
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

func (hlu *httpLayerUpload) Seek(offset int64, whence int) (int64, error) {
	newOffset := hlu.offset

	switch whence {
	case os.SEEK_CUR:
		newOffset += int64(offset)
	case os.SEEK_END:
		return newOffset, errors.New("Cannot seek from end on incomplete upload")
	case os.SEEK_SET:
		newOffset = int64(offset)
	}

	hlu.offset = newOffset

	return hlu.offset, nil
}

func (hlu *httpLayerUpload) UUID() string {
	return hlu.uuid
}

func (hlu *httpLayerUpload) StartedAt() time.Time {
	return hlu.startedAt
}

func (hlu *httpLayerUpload) Finish(digest digest.Digest) (distribution.Layer, error) {
	// TODO(dmcgowan): Check if already finished, if so just fetch
	req, err := http.NewRequest("PUT", hlu.location, nil)
	if err != nil {
		return nil, err
	}

	values := req.URL.Query()
	values.Set("digest", digest.String())
	req.URL.RawQuery = values.Encode()

	resp, err := hlu.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, hlu.handleErrorResponse(resp)
	}

	return hlu.repo.Layers().Fetch(digest)
}

func (hlu *httpLayerUpload) Cancel() error {
	panic("not implemented")
}

func (hlu *httpLayerUpload) Close() error {
	hlu.closed = true
	return nil
}
