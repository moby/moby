package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/docker/distribution/manifest"

	"github.com/docker/distribution/digest"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/api/v2"
)

// NewRepository creates a new Repository for the given repository name and endpoint
func NewRepository(ctx context.Context, name, endpoint string, transport http.RoundTripper) (distribution.Repository, error) {
	if err := v2.ValidateRespositoryName(name); err != nil {
		return nil, err
	}

	ub, err := v2.NewURLBuilderFromString(endpoint)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   1 * time.Minute,
		// TODO(dmcgowan): create cookie jar
	}

	return &repository{
		client:  client,
		ub:      ub,
		name:    name,
		context: ctx,
	}, nil
}

type repository struct {
	client  *http.Client
	ub      *v2.URLBuilder
	context context.Context
	name    string
}

func (r *repository) Name() string {
	return r.name
}

func (r *repository) Blobs(ctx context.Context) distribution.BlobService {
	return &blobs{
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
	u, err := ms.ub.BuildTagsURL(ms.name)
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
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		tagsResponse := struct {
			Tags []string `json:"tags"`
		}{}
		if err := json.Unmarshal(b, &tagsResponse); err != nil {
			return nil, err
		}

		return tagsResponse.Tags, nil
	case resp.StatusCode == http.StatusNotFound:
		return nil, nil
	default:
		return nil, handleErrorResponse(resp)
	}
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
	default:
		return false, handleErrorResponse(resp)
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
	default:
		return nil, handleErrorResponse(resp)
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
	default:
		return handleErrorResponse(resp)
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
	default:
		return handleErrorResponse(resp)
	}
}

type blobs struct {
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

func (ls *blobs) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	desc, err := ls.Stat(ctx, dgst)
	if err != nil {
		return nil, err
	}
	reader, err := ls.Open(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return ioutil.ReadAll(reader)
}

func (ls *blobs) Open(ctx context.Context, desc distribution.Descriptor) (distribution.ReadSeekCloser, error) {
	return &httpBlob{
		repository: ls.repository,
		desc:       desc,
	}, nil
}

func (ls *blobs) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, desc distribution.Descriptor) error {
	return nil
}

func (ls *blobs) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	writer, err := ls.Writer(ctx)
	if err != nil {
		return distribution.Descriptor{}, err
	}
	dgstr := digest.NewCanonicalDigester()
	n, err := io.Copy(writer, io.TeeReader(bytes.NewReader(p), dgstr))
	if err != nil {
		return distribution.Descriptor{}, err
	}
	if n < int64(len(p)) {
		return distribution.Descriptor{}, fmt.Errorf("short copy: wrote %d of %d", n, len(p))
	}

	desc := distribution.Descriptor{
		MediaType: mediaType,
		Length:    int64(len(p)),
		Digest:    dgstr.Digest(),
	}

	return writer.Commit(ctx, desc)
}

func (ls *blobs) Writer(ctx context.Context) (distribution.BlobWriter, error) {
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

		return &httpBlobUpload{
			repo:      ls.repository,
			client:    ls.client,
			uuid:      uuid,
			startedAt: time.Now(),
			location:  location,
		}, nil
	default:
		return nil, handleErrorResponse(resp)
	}
}

func (ls *blobs) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	panic("not implemented")
}

func (ls *blobs) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	u, err := ls.ub.BuildBlobURL(ls.name, dgst)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	resp, err := ls.client.Head(u)
	if err != nil {
		return distribution.Descriptor{}, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		lengthHeader := resp.Header.Get("Content-Length")
		length, err := strconv.ParseInt(lengthHeader, 10, 64)
		if err != nil {
			return distribution.Descriptor{}, fmt.Errorf("error parsing content-length: %v", err)
		}

		return distribution.Descriptor{
			MediaType: resp.Header.Get("Content-Type"),
			Length:    length,
			Digest:    dgst,
		}, nil
	case resp.StatusCode == http.StatusNotFound:
		return distribution.Descriptor{}, distribution.ErrBlobUnknown
	default:
		return distribution.Descriptor{}, handleErrorResponse(resp)
	}
}
