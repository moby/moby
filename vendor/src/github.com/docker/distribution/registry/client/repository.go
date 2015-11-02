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
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/distribution/registry/storage/cache"
	"github.com/docker/distribution/registry/storage/cache/memory"
)

// Registry provides an interface for calling Repositories, which returns a catalog of repositories.
type Registry interface {
	Repositories(ctx context.Context, repos []string, last string) (n int, err error)
}

// NewRegistry creates a registry namespace which can be used to get a listing of repositories
func NewRegistry(ctx context.Context, baseURL string, transport http.RoundTripper) (Registry, error) {
	ub, err := v2.NewURLBuilderFromString(baseURL)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   1 * time.Minute,
	}

	return &registry{
		client:  client,
		ub:      ub,
		context: ctx,
	}, nil
}

type registry struct {
	client  *http.Client
	ub      *v2.URLBuilder
	context context.Context
}

// Repositories returns a lexigraphically sorted catalog given a base URL.  The 'entries' slice will be filled up to the size
// of the slice, starting at the value provided in 'last'.  The number of entries will be returned along with io.EOF if there
// are no more entries
func (r *registry) Repositories(ctx context.Context, entries []string, last string) (int, error) {
	var numFilled int
	var returnErr error

	values := buildCatalogValues(len(entries), last)
	u, err := r.ub.BuildCatalogURL(values)
	if err != nil {
		return 0, err
	}

	resp, err := r.client.Get(u)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if SuccessStatus(resp.StatusCode) {
		var ctlg struct {
			Repositories []string `json:"repositories"`
		}
		decoder := json.NewDecoder(resp.Body)

		if err := decoder.Decode(&ctlg); err != nil {
			return 0, err
		}

		for cnt := range ctlg.Repositories {
			entries[cnt] = ctlg.Repositories[cnt]
		}
		numFilled = len(ctlg.Repositories)

		link := resp.Header.Get("Link")
		if link == "" {
			returnErr = io.EOF
		}
	} else {
		return 0, handleErrorResponse(resp)
	}

	return numFilled, returnErr
}

// NewRepository creates a new Repository for the given repository name and base URL.
func NewRepository(ctx context.Context, name, baseURL string, transport http.RoundTripper) (distribution.Repository, error) {
	if _, err := reference.ParseNamed(name); err != nil {
		return nil, err
	}

	ub, err := v2.NewURLBuilderFromString(baseURL)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: transport,
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
		name:   r.Name(),
		ub:     r.ub,
		client: r.client,
	}
	return &blobs{
		name:    r.Name(),
		ub:      r.ub,
		client:  r.client,
		statter: cache.NewCachedBlobStatter(memory.NewInMemoryBlobDescriptorCacheProvider(), statter),
	}
}

func (r *repository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	// todo(richardscothern): options should be sent over the wire
	return &manifests{
		name:   r.Name(),
		ub:     r.ub,
		client: r.client,
		etags:  make(map[string]string),
	}, nil
}

func (r *repository) Signatures() distribution.SignatureService {
	ms, _ := r.Manifests(r.context)
	return &signatures{
		manifests: ms,
	}
}

type signatures struct {
	manifests distribution.ManifestService
}

func (s *signatures) Get(dgst digest.Digest) ([][]byte, error) {
	m, err := s.manifests.Get(dgst)
	if err != nil {
		return nil, err
	}
	return m.Signatures()
}

func (s *signatures) Put(dgst digest.Digest, signatures ...[]byte) error {
	panic("not implemented")
}

type manifests struct {
	name   string
	ub     *v2.URLBuilder
	client *http.Client
	etags  map[string]string
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

	if SuccessStatus(resp.StatusCode) {
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
	}
	return nil, handleErrorResponse(resp)
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

	if SuccessStatus(resp.StatusCode) {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, handleErrorResponse(resp)
}

func (ms *manifests) Get(dgst digest.Digest) (*schema1.SignedManifest, error) {
	// Call by Tag endpoint since the API uses the same
	// URL endpoint for tags and digests.
	return ms.GetByTag(dgst.String())
}

// AddEtagToTag allows a client to supply an eTag to GetByTag which will be
// used for a conditional HTTP request.  If the eTag matches, a nil manifest
// and nil error will be returned. etag is automatically quoted when added to
// this map.
func AddEtagToTag(tag, etag string) distribution.ManifestServiceOption {
	return func(ms distribution.ManifestService) error {
		if ms, ok := ms.(*manifests); ok {
			ms.etags[tag] = fmt.Sprintf(`"%s"`, etag)
			return nil
		}
		return fmt.Errorf("etag options is a client-only option")
	}
}

func (ms *manifests) GetByTag(tag string, options ...distribution.ManifestServiceOption) (*schema1.SignedManifest, error) {
	for _, option := range options {
		err := option(ms)
		if err != nil {
			return nil, err
		}
	}

	u, err := ms.ub.BuildManifestURL(ms.name, tag)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	if _, ok := ms.etags[tag]; ok {
		req.Header.Set("If-None-Match", ms.etags[tag])
	}
	resp, err := ms.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return nil, distribution.ErrManifestNotModified
	} else if SuccessStatus(resp.StatusCode) {
		var sm schema1.SignedManifest
		decoder := json.NewDecoder(resp.Body)

		if err := decoder.Decode(&sm); err != nil {
			return nil, err
		}
		return &sm, nil
	}
	return nil, handleErrorResponse(resp)
}

func (ms *manifests) Put(m *schema1.SignedManifest) error {
	manifestURL, err := ms.ub.BuildManifestURL(ms.name, m.Tag)
	if err != nil {
		return err
	}

	// todo(richardscothern): do something with options here when they become applicable

	putRequest, err := http.NewRequest("PUT", manifestURL, bytes.NewReader(m.Raw))
	if err != nil {
		return err
	}

	resp, err := ms.client.Do(putRequest)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if SuccessStatus(resp.StatusCode) {
		// TODO(dmcgowan): make use of digest header
		return nil
	}
	return handleErrorResponse(resp)
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

	if SuccessStatus(resp.StatusCode) {
		return nil
	}
	return handleErrorResponse(resp)
}

type blobs struct {
	name   string
	ub     *v2.URLBuilder
	client *http.Client

	statter distribution.BlobDescriptorService
	distribution.BlobDeleter
}

func sanitizeLocation(location, base string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	locationURL, err := url.Parse(location)
	if err != nil {
		return "", err
	}

	return baseURL.ResolveReference(locationURL).String(), nil
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

	blobURL, err := bs.ub.BuildBlobURL(bs.name, stat.Digest)
	if err != nil {
		return nil, err
	}

	return transport.NewHTTPReadSeeker(bs.client, blobURL, stat.Size), nil
}

func (bs *blobs) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	panic("not implemented")
}

func (bs *blobs) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	writer, err := bs.Create(ctx)
	if err != nil {
		return distribution.Descriptor{}, err
	}
	dgstr := digest.Canonical.New()
	n, err := io.Copy(writer, io.TeeReader(bytes.NewReader(p), dgstr.Hash()))
	if err != nil {
		return distribution.Descriptor{}, err
	}
	if n < int64(len(p)) {
		return distribution.Descriptor{}, fmt.Errorf("short copy: wrote %d of %d", n, len(p))
	}

	desc := distribution.Descriptor{
		MediaType: mediaType,
		Size:      int64(len(p)),
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

	if SuccessStatus(resp.StatusCode) {
		// TODO(dmcgowan): Check for invalid UUID
		uuid := resp.Header.Get("Docker-Upload-UUID")
		location, err := sanitizeLocation(resp.Header.Get("Location"), u)
		if err != nil {
			return nil, err
		}

		return &httpBlobUpload{
			statter:   bs.statter,
			client:    bs.client,
			uuid:      uuid,
			startedAt: time.Now(),
			location:  location,
		}, nil
	}
	return nil, handleErrorResponse(resp)
}

func (bs *blobs) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	panic("not implemented")
}

func (bs *blobs) Delete(ctx context.Context, dgst digest.Digest) error {
	return bs.statter.Clear(ctx, dgst)
}

type blobStatter struct {
	name   string
	ub     *v2.URLBuilder
	client *http.Client
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

	if SuccessStatus(resp.StatusCode) {
		lengthHeader := resp.Header.Get("Content-Length")
		length, err := strconv.ParseInt(lengthHeader, 10, 64)
		if err != nil {
			return distribution.Descriptor{}, fmt.Errorf("error parsing content-length: %v", err)
		}

		return distribution.Descriptor{
			MediaType: resp.Header.Get("Content-Type"),
			Size:      length,
			Digest:    dgst,
		}, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return distribution.Descriptor{}, distribution.ErrBlobUnknown
	}
	return distribution.Descriptor{}, handleErrorResponse(resp)
}

func buildCatalogValues(maxEntries int, last string) url.Values {
	values := url.Values{}

	if maxEntries > 0 {
		values.Add("n", strconv.Itoa(maxEntries))
	}

	if last != "" {
		values.Add("last", last)
	}

	return values
}

func (bs *blobStatter) Clear(ctx context.Context, dgst digest.Digest) error {
	blobURL, err := bs.ub.BuildBlobURL(bs.name, dgst)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", blobURL, nil)
	if err != nil {
		return err
	}

	resp, err := bs.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if SuccessStatus(resp.StatusCode) {
		return nil
	}
	return handleErrorResponse(resp)
}

func (bs *blobStatter) SetDescriptor(ctx context.Context, dgst digest.Digest, desc distribution.Descriptor) error {
	return nil
}
