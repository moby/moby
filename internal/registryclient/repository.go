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

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/distribution/registry/storage/cache"
)

// NewRepository creates a new Repository for the given repository name and base URL
func NewRepository(ctx context.Context, name, baseURL string, transport http.RoundTripper) (distribution.Repository, error) {
	if err := v2.ValidateRespositoryName(name); err != nil {
		return nil, err
	}

	ub, err := v2.NewURLBuilderFromString(baseURL)
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

func (r *repository) Blobs(ctx context.Context) distribution.BlobStore {
	statter := &blobStatter{
		repository: r,
	}
	return &blobs{
		repository: r,
		statter:    cache.NewCachedBlobStatter(cache.NewInMemoryBlobDescriptorCacheProvider(), statter),
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

	switch resp.StatusCode {
	case http.StatusOK:
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
	case http.StatusNotFound:
		return nil, nil
	default:
		return nil, handleErrorResponse(resp)
	}
}

func (ms *manifests) Exists(dgst digest.Digest) (bool, error) {
	// Call by Tag endpoint since the API uses the same
	// URL endpoint for tags and digests.
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

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, handleErrorResponse(resp)
	}
}

func (ms *manifests) Get(dgst digest.Digest) (*manifest.SignedManifest, error) {
	// Call by Tag endpoint since the API uses the same
	// URL endpoint for tags and digests.
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

	switch resp.StatusCode {
	case http.StatusOK:
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

	switch resp.StatusCode {
	case http.StatusAccepted:
		// TODO(dmcgowan): make use of digest header
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

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	default:
		return handleErrorResponse(resp)
	}
}

type blobs struct {
	*repository

	statter distribution.BlobStatter
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

func (bs *blobs) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	return bs.statter.Stat(ctx, dgst)

}

func (bs *blobs) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	desc, err := bs.Stat(ctx, dgst)
	if err != nil {
		return nil, err
	}
	reader, err := bs.Open(ctx, desc.Digest)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return ioutil.ReadAll(reader)
}

func (bs *blobs) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	stat, err := bs.statter.Stat(ctx, dgst)
	if err != nil {
		return nil, err
	}

	blobURL, err := bs.ub.BuildBlobURL(bs.Name(), stat.Digest)
	if err != nil {
		return nil, err
	}

	return transport.NewHTTPReadSeeker(bs.repository.client, blobURL, stat.Length), nil
}

func (bs *blobs) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	panic("not implemented")
}

func (bs *blobs) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	writer, err := bs.Create(ctx)
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

func (bs *blobs) Create(ctx context.Context) (distribution.BlobWriter, error) {
	u, err := bs.ub.BuildBlobUploadURL(bs.name)

	resp, err := bs.client.Post(u, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		// TODO(dmcgowan): Check for invalid UUID
		uuid := resp.Header.Get("Docker-Upload-UUID")
		location, err := sanitizeLocation(resp.Header.Get("Location"), u)
		if err != nil {
			return nil, err
		}

		return &httpBlobUpload{
			repo:      bs.repository,
			client:    bs.client,
			uuid:      uuid,
			startedAt: time.Now(),
			location:  location,
		}, nil
	default:
		return nil, handleErrorResponse(resp)
	}
}

func (bs *blobs) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	panic("not implemented")
}

type blobStatter struct {
	*repository
}

func (bs *blobStatter) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	u, err := bs.ub.BuildBlobURL(bs.name, dgst)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	resp, err := bs.client.Head(u)
	if err != nil {
		return distribution.Descriptor{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
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
	case http.StatusNotFound:
		return distribution.Descriptor{}, distribution.ErrBlobUnknown
	default:
		return distribution.Descriptor{}, handleErrorResponse(resp)
	}
}
