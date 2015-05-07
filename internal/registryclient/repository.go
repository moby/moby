package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	ctxu "github.com/docker/distribution/context"

	"github.com/docker/distribution/manifest"

	"github.com/docker/distribution/digest"

	"github.com/docker/distribution"
	"github.com/docker/distribution/registry/api/v2"
	"golang.org/x/net/context"
)

// NewRepository creates a new Repository for the given repository name and endpoint
func NewRepository(ctx context.Context, name string, endpoint *RepositoryEndpoint) (distribution.Repository, error) {
	if err := v2.ValidateRespositoryName(name); err != nil {
		return nil, err
	}

	ub, err := endpoint.URLBuilder()
	if err != nil {
		return nil, err
	}

	client, err := endpoint.HTTPClient(name)
	if err != nil {
		return nil, err
	}

	return &repository{
		client:  client,
		ub:      ub,
		name:    name,
		context: ctx,
		mirror:  endpoint.Mirror,
	}, nil
}

type repository struct {
	client  *http.Client
	ub      *v2.URLBuilder
	context context.Context
	name    string
	mirror  bool
}

func (r *repository) Name() string {
	return r.name
}

func (r *repository) Layers() distribution.LayerService {
	return &layers{
		repository: r,
	}
}

func (r *repository) Manifests() distribution.ManifestService {
	return &manifests{
		repository: r,
	}
}

func (r *repository) Signatures() distribution.SignatureService {
	return &signatures{
		repository: r,
	}
}

type signatures struct {
	*repository
}

func (s *signatures) Get(dgst digest.Digest) ([][]byte, error) {
	panic("not implemented")
}

func (s *signatures) Put(dgst digest.Digest, signatures ...[]byte) error {
	panic("not implemented")
}

type manifests struct {
	*repository
}

func (ms *manifests) Tags() ([]string, error) {
	panic("not implemented")
}

func (ms *manifests) Exists(dgst digest.Digest) (bool, error) {
	return ms.ExistsByTag(dgst.String())
}

func (ms *manifests) ExistsByTag(tag string) (bool, error) {
	u, err := ms.ub.BuildManifestURL(ms.name, tag)
	if err != nil {
		return false, err
	}

	resp, err := ms.client.Head(u)
	if err != nil {
		return false, err
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		return true, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return false, parseHTTPErrorResponse(resp)
	default:
		return false, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

func (ms *manifests) Get(dgst digest.Digest) (*manifest.SignedManifest, error) {
	return ms.GetByTag(dgst.String())
}

func (ms *manifests) GetByTag(tag string) (*manifest.SignedManifest, error) {
	u, err := ms.ub.BuildManifestURL(ms.name, tag)
	if err != nil {
		return nil, err
	}

	resp, err := ms.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		var sm manifest.SignedManifest
		decoder := json.NewDecoder(resp.Body)

		if err := decoder.Decode(&sm); err != nil {
			return nil, err
		}

		return &sm, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, parseHTTPErrorResponse(resp)
	default:
		return nil, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

func (ms *manifests) Put(m *manifest.SignedManifest) error {
	manifestURL, err := ms.ub.BuildManifestURL(ms.name, m.Tag)
	if err != nil {
		return err
	}

	putRequest, err := http.NewRequest("PUT", manifestURL, bytes.NewReader(m.Raw))
	if err != nil {
		return err
	}

	resp, err := ms.client.Do(putRequest)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusAccepted:
		// TODO(dmcgowan): Use or check digest header
		return nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return parseHTTPErrorResponse(resp)
	default:
		return &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

func (ms *manifests) Delete(dgst digest.Digest) error {
	u, err := ms.ub.BuildManifestURL(ms.name, dgst.String())
	if err != nil {
		return err
	}
	req, err := http.NewRequest("DELETE", u, nil)
	if err != nil {
		return err
	}

	resp, err := ms.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return parseHTTPErrorResponse(resp)
	default:
		return &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

type layers struct {
	*repository
}

func sanitizeLocation(location, source string) (string, error) {
	locationURL, err := url.Parse(location)
	if err != nil {
		return "", err
	}

	if locationURL.Scheme == "" {
		sourceURL, err := url.Parse(source)
		if err != nil {
			return "", err
		}
		locationURL = &url.URL{
			Scheme: sourceURL.Scheme,
			Host:   sourceURL.Host,
			Path:   location,
		}
		location = locationURL.String()
	}
	return location, nil
}

func (ls *layers) Exists(dgst digest.Digest) (bool, error) {
	_, err := ls.fetchLayer(dgst)
	if err != nil {
		switch err := err.(type) {
		case distribution.ErrUnknownLayer:
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func (ls *layers) Fetch(dgst digest.Digest) (distribution.Layer, error) {
	return ls.fetchLayer(dgst)
}

func (ls *layers) Upload() (distribution.LayerUpload, error) {
	u, err := ls.ub.BuildBlobUploadURL(ls.name)

	resp, err := ls.client.Post(u, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusAccepted:
		// TODO(dmcgowan): Check for invalid UUID
		uuid := resp.Header.Get("Docker-Upload-UUID")
		location, err := sanitizeLocation(resp.Header.Get("Location"), u)
		if err != nil {
			return nil, err
		}

		return &httpLayerUpload{
			layers:    ls,
			uuid:      uuid,
			startedAt: time.Now(),
			location:  location,
		}, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, parseHTTPErrorResponse(resp)
	default:
		return nil, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

func (ls *layers) Resume(uuid string) (distribution.LayerUpload, error) {
	panic("not implemented")
}

func (ls *layers) fetchLayer(dgst digest.Digest) (distribution.Layer, error) {
	u, err := ls.ub.BuildBlobURL(ls.name, dgst)
	if err != nil {
		return nil, err
	}

	resp, err := ls.client.Head(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		lengthHeader := resp.Header.Get("Content-Length")
		length, err := strconv.ParseInt(lengthHeader, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing content-length: %v", err)
		}

		var t time.Time
		lastModified := resp.Header.Get("Last-Modified")
		if lastModified != "" {
			t, err = http.ParseTime(lastModified)
			if err != nil {
				return nil, fmt.Errorf("error parsing last-modified: %v", err)
			}
		}

		return &httpLayer{
			layers:    ls,
			size:      length,
			digest:    dgst,
			createdAt: t,
		}, nil
	case resp.StatusCode == http.StatusNotFound:
		return nil, distribution.ErrUnknownLayer{
			FSLayer: manifest.FSLayer{
				BlobSum: dgst,
			},
		}
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, parseHTTPErrorResponse(resp)
	default:
		return nil, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

type httpLayer struct {
	*layers

	size      int64
	digest    digest.Digest
	createdAt time.Time

	rc     io.ReadCloser // remote read closer
	brd    *bufio.Reader // internal buffered io
	offset int64
	err    error
}

func (hl *httpLayer) CreatedAt() time.Time {
	return hl.createdAt
}

func (hl *httpLayer) Digest() digest.Digest {
	return hl.digest
}

func (hl *httpLayer) Read(p []byte) (n int, err error) {
	if hl.err != nil {
		return 0, hl.err
	}

	rd, err := hl.reader()
	if err != nil {
		return 0, err
	}

	n, err = rd.Read(p)
	hl.offset += int64(n)

	// Simulate io.EOR error if we reach filesize.
	if err == nil && hl.offset >= hl.size {
		err = io.EOF
	}

	return n, err
}

func (hl *httpLayer) Seek(offset int64, whence int) (int64, error) {
	if hl.err != nil {
		return 0, hl.err
	}

	var err error
	newOffset := hl.offset

	switch whence {
	case os.SEEK_CUR:
		newOffset += int64(offset)
	case os.SEEK_END:
		newOffset = hl.size + int64(offset)
	case os.SEEK_SET:
		newOffset = int64(offset)
	}

	if newOffset < 0 {
		err = fmt.Errorf("cannot seek to negative position")
	} else {
		if hl.offset != newOffset {
			hl.reset()
		}

		// No problems, set the offset.
		hl.offset = newOffset
	}

	return hl.offset, err
}

func (hl *httpLayer) Close() error {
	if hl.err != nil {
		return hl.err
	}

	// close and release reader chain
	if hl.rc != nil {
		hl.rc.Close()
	}

	hl.rc = nil
	hl.brd = nil

	hl.err = fmt.Errorf("httpLayer: closed")

	return nil
}

func (hl *httpLayer) reset() {
	if hl.err != nil {
		return
	}
	if hl.rc != nil {
		hl.rc.Close()
		hl.rc = nil
	}
}

func (hl *httpLayer) reader() (io.Reader, error) {
	if hl.err != nil {
		return nil, hl.err
	}

	if hl.rc != nil {
		return hl.brd, nil
	}

	// If the offset is great than or equal to size, return a empty, noop reader.
	if hl.offset >= hl.size {
		return ioutil.NopCloser(bytes.NewReader([]byte{})), nil
	}

	blobURL, err := hl.ub.BuildBlobURL(hl.name, hl.digest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", blobURL, nil)
	if err != nil {
		return nil, err
	}

	if hl.offset > 0 {
		// TODO(stevvooe): Get this working correctly.

		// If we are at different offset, issue a range request from there.
		req.Header.Add("Range", fmt.Sprintf("1-"))
		ctxu.GetLogger(hl.context).Infof("Range: %s", req.Header.Get("Range"))
	}

	resp, err := hl.client.Do(req)
	if err != nil {
		return nil, err
	}

	switch {
	case resp.StatusCode == 200:
		hl.rc = resp.Body
	default:
		defer resp.Body.Close()
		return nil, fmt.Errorf("unexpected status resolving reader: %v", resp.Status)
	}

	if hl.brd == nil {
		hl.brd = bufio.NewReader(hl.rc)
	} else {
		hl.brd.Reset(hl.rc)
	}

	return hl.brd, nil
}

func (hl *httpLayer) Length() int64 {
	return hl.size
}

func (hl *httpLayer) Handler(r *http.Request) (http.Handler, error) {
	panic("Not implemented")
}

type httpLayerUpload struct {
	*layers

	uuid      string
	startedAt time.Time

	location string // always the last value of the location header.
	offset   int64
	closed   bool
}

var _ distribution.LayerUpload = &httpLayerUpload{}

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

	switch {
	case resp.StatusCode == http.StatusAccepted:
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
	case resp.StatusCode == http.StatusNotFound:
		return 0, &BlobUploadNotFoundError{Location: hlu.location}
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return 0, parseHTTPErrorResponse(resp)
	default:
		return 0, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
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

	switch {
	case resp.StatusCode == http.StatusAccepted:
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
	case resp.StatusCode == http.StatusNotFound:
		return 0, &BlobUploadNotFoundError{Location: hlu.location}
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return 0, parseHTTPErrorResponse(resp)
	default:
		return 0, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
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

	switch {
	case resp.StatusCode == http.StatusCreated:
		return hlu.Layers().Fetch(digest)
	case resp.StatusCode == http.StatusNotFound:
		return nil, &BlobUploadNotFoundError{Location: hlu.location}
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, parseHTTPErrorResponse(resp)
	default:
		return nil, &UnexpectedHTTPStatusError{Status: resp.Status}
	}
}

func (hlu *httpLayerUpload) Cancel() error {
	panic("not implemented")
}

func (hlu *httpLayerUpload) Close() error {
	hlu.closed = true
	return nil
}
