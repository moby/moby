package client

import (
	"bytes"
	"encoding/json"
	"errors"
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
		return 0, HandleErrorResponse(resp)
	}

	return numFilled, returnErr
}

// NewRepository creates a new Repository for the given repository name and base URL.
func NewRepository(ctx context.Context, name reference.Named, baseURL string, transport http.RoundTripper) (distribution.Repository, error) {
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
	name    reference.Named
}

func (r *repository) Name() reference.Named {
	return r.name
}

func (r *repository) Blobs(ctx context.Context) distribution.BlobStore {
	statter := &blobStatter{
		name:   r.name,
		ub:     r.ub,
		client: r.client,
	}
	return &blobs{
		name:    r.name,
		ub:      r.ub,
		client:  r.client,
		statter: cache.NewCachedBlobStatter(memory.NewInMemoryBlobDescriptorCacheProvider(), statter),
	}
}

func (r *repository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	// todo(richardscothern): options should be sent over the wire
	return &manifests{
		name:   r.name,
		ub:     r.ub,
		client: r.client,
		etags:  make(map[string]string),
	}, nil
}

func (r *repository) Tags(ctx context.Context) distribution.TagService {
	return &tags{
		client:  r.client,
		ub:      r.ub,
		context: r.context,
		name:    r.Name(),
	}
}

// tags implements remote tagging operations.
type tags struct {
	client  *http.Client
	ub      *v2.URLBuilder
	context context.Context
	name    reference.Named
}

// All returns all tags
func (t *tags) All(ctx context.Context) ([]string, error) {
	var tags []string

	u, err := t.ub.BuildTagsURL(t.name)
	if err != nil {
		return tags, err
	}

	resp, err := t.client.Get(u)
	if err != nil {
		return tags, err
	}
	defer resp.Body.Close()

	if SuccessStatus(resp.StatusCode) {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return tags, err
		}

		tagsResponse := struct {
			Tags []string `json:"tags"`
		}{}
		if err := json.Unmarshal(b, &tagsResponse); err != nil {
			return tags, err
		}
		tags = tagsResponse.Tags
		return tags, nil
	}
	return tags, HandleErrorResponse(resp)
}

func descriptorFromResponse(response *http.Response) (distribution.Descriptor, error) {
	desc := distribution.Descriptor{}
	headers := response.Header

	ctHeader := headers.Get("Content-Type")
	if ctHeader == "" {
		return distribution.Descriptor{}, errors.New("missing or empty Content-Type header")
	}
	desc.MediaType = ctHeader

	digestHeader := headers.Get("Docker-Content-Digest")
	if digestHeader == "" {
		bytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return distribution.Descriptor{}, err
		}
		_, desc, err := distribution.UnmarshalManifest(ctHeader, bytes)
		if err != nil {
			return distribution.Descriptor{}, err
		}
		return desc, nil
	}

	dgst, err := digest.ParseDigest(digestHeader)
	if err != nil {
		return distribution.Descriptor{}, err
	}
	desc.Digest = dgst

	lengthHeader := headers.Get("Content-Length")
	if lengthHeader == "" {
		return distribution.Descriptor{}, errors.New("missing or empty Content-Length header")
	}
	length, err := strconv.ParseInt(lengthHeader, 10, 64)
	if err != nil {
		return distribution.Descriptor{}, err
	}
	desc.Size = length

	return desc, nil

}

// Get issues a HEAD request for a Manifest against its named endpoint in order
// to construct a descriptor for the tag.  If the registry doesn't support HEADing
// a manifest, fallback to GET.
func (t *tags) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	u, err := t.ub.BuildManifestURL(t.name, tag)
	if err != nil {
		return distribution.Descriptor{}, err
	}
	var attempts int
	resp, err := t.client.Head(u)

check:
	if err != nil {
		return distribution.Descriptor{}, err
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		return descriptorFromResponse(resp)
	case resp.StatusCode == http.StatusMethodNotAllowed:
		resp, err = t.client.Get(u)
		attempts++
		if attempts > 1 {
			return distribution.Descriptor{}, err
		}
		goto check
	default:
		return distribution.Descriptor{}, HandleErrorResponse(resp)
	}
}

func (t *tags) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	panic("not implemented")
}

func (t *tags) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
	panic("not implemented")
}

func (t *tags) Untag(ctx context.Context, tag string) error {
	panic("not implemented")
}

type manifests struct {
	name   reference.Named
	ub     *v2.URLBuilder
	client *http.Client
	etags  map[string]string
}

func (ms *manifests) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	u, err := ms.ub.BuildManifestURL(ms.name, dgst.String())
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
	return false, HandleErrorResponse(resp)
}

// AddEtagToTag allows a client to supply an eTag to Get which will be
// used for a conditional HTTP request.  If the eTag matches, a nil manifest
// and ErrManifestNotModified error will be returned. etag is automatically
// quoted when added to this map.
func AddEtagToTag(tag, etag string) distribution.ManifestServiceOption {
	return etagOption{tag, etag}
}

type etagOption struct{ tag, etag string }

func (o etagOption) Apply(ms distribution.ManifestService) error {
	if ms, ok := ms.(*manifests); ok {
		ms.etags[o.tag] = fmt.Sprintf(`"%s"`, o.etag)
		return nil
	}
	return fmt.Errorf("etag options is a client-only option")
}

func (ms *manifests) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {

	var tag string
	for _, option := range options {
		if opt, ok := option.(withTagOption); ok {
			tag = opt.tag
		} else {
			err := option.Apply(ms)
			if err != nil {
				return nil, err
			}
		}
	}

	var ref string
	if tag != "" {
		ref = tag
	} else {
		ref = dgst.String()
	}

	u, err := ms.ub.BuildManifestURL(ms.name, ref)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	for _, t := range distribution.ManifestMediaTypes() {
		req.Header.Add("Accept", t)
	}

	if _, ok := ms.etags[ref]; ok {
		req.Header.Set("If-None-Match", ms.etags[ref])
	}

	resp, err := ms.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return nil, distribution.ErrManifestNotModified
	} else if SuccessStatus(resp.StatusCode) {
		mt := resp.Header.Get("Content-Type")
		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			return nil, err
		}
		m, _, err := distribution.UnmarshalManifest(mt, body)
		if err != nil {
			return nil, err
		}
		return m, nil
	}
	return nil, HandleErrorResponse(resp)
}

// WithTag allows a tag to be passed into Put which enables the client
// to build a correct URL.
func WithTag(tag string) distribution.ManifestServiceOption {
	return withTagOption{tag}
}

type withTagOption struct{ tag string }

func (o withTagOption) Apply(m distribution.ManifestService) error {
	if _, ok := m.(*manifests); ok {
		return nil
	}
	return fmt.Errorf("withTagOption is a client-only option")
}

// Put puts a manifest.  A tag can be specified using an options parameter which uses some shared state to hold the
// tag name in order to build the correct upload URL.  This state is written and read under a lock.
func (ms *manifests) Put(ctx context.Context, m distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	var tag string

	for _, option := range options {
		if opt, ok := option.(withTagOption); ok {
			tag = opt.tag
		} else {
			err := option.Apply(ms)
			if err != nil {
				return "", err
			}
		}
	}

	manifestURL, err := ms.ub.BuildManifestURL(ms.name, tag)
	if err != nil {
		return "", err
	}

	mediaType, p, err := m.Payload()
	if err != nil {
		return "", err
	}

	putRequest, err := http.NewRequest("PUT", manifestURL, bytes.NewReader(p))
	if err != nil {
		return "", err
	}

	putRequest.Header.Set("Content-Type", mediaType)

	resp, err := ms.client.Do(putRequest)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if SuccessStatus(resp.StatusCode) {
		dgstHeader := resp.Header.Get("Docker-Content-Digest")
		dgst, err := digest.ParseDigest(dgstHeader)
		if err != nil {
			return "", err
		}

		return dgst, nil
	}

	return "", HandleErrorResponse(resp)
}

func (ms *manifests) Delete(ctx context.Context, dgst digest.Digest) error {
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
	return HandleErrorResponse(resp)
}

// todo(richardscothern): Restore interface and implementation with merge of #1050
/*func (ms *manifests) Enumerate(ctx context.Context, manifests []distribution.Manifest, last distribution.Manifest) (n int, err error) {
	panic("not supported")
}*/

type blobs struct {
	name   reference.Named
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
	reader, err := bs.Open(ctx, dgst)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return ioutil.ReadAll(reader)
}

func (bs *blobs) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	blobURL, err := bs.ub.BuildBlobURL(bs.name, dgst)
	if err != nil {
		return nil, err
	}

	return transport.NewHTTPReadSeeker(bs.client, blobURL,
		func(resp *http.Response) error {
			if resp.StatusCode == http.StatusNotFound {
				return distribution.ErrBlobUnknown
			}
			return HandleErrorResponse(resp)
		}), nil
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

// createOptions is a collection of blob creation modifiers relevant to general
// blob storage intended to be configured by the BlobCreateOption.Apply method.
type createOptions struct {
	Mount struct {
		ShouldMount bool
		From        reference.Canonical
	}
}

type optionFunc func(interface{}) error

func (f optionFunc) Apply(v interface{}) error {
	return f(v)
}

// WithMountFrom returns a BlobCreateOption which designates that the blob should be
// mounted from the given canonical reference.
func WithMountFrom(ref reference.Canonical) distribution.BlobCreateOption {
	return optionFunc(func(v interface{}) error {
		opts, ok := v.(*createOptions)
		if !ok {
			return fmt.Errorf("unexpected options type: %T", v)
		}

		opts.Mount.ShouldMount = true
		opts.Mount.From = ref

		return nil
	})
}

func (bs *blobs) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	var opts createOptions

	for _, option := range options {
		err := option.Apply(&opts)
		if err != nil {
			return nil, err
		}
	}

	var values []url.Values

	if opts.Mount.ShouldMount {
		values = append(values, url.Values{"from": {opts.Mount.From.Name()}, "mount": {opts.Mount.From.Digest().String()}})
	}

	u, err := bs.ub.BuildBlobUploadURL(bs.name, values...)
	if err != nil {
		return nil, err
	}

	resp, err := bs.client.Post(u, "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		desc, err := bs.statter.Stat(ctx, opts.Mount.From.Digest())
		if err != nil {
			return nil, err
		}
		return nil, distribution.ErrBlobMounted{From: opts.Mount.From, Descriptor: desc}
	case http.StatusAccepted:
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
	default:
		return nil, HandleErrorResponse(resp)
	}
}

func (bs *blobs) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	panic("not implemented")
}

func (bs *blobs) Delete(ctx context.Context, dgst digest.Digest) error {
	return bs.statter.Clear(ctx, dgst)
}

type blobStatter struct {
	name   reference.Named
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
		if lengthHeader == "" {
			return distribution.Descriptor{}, fmt.Errorf("missing content-length header for request: %s", u)
		}

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
	return distribution.Descriptor{}, HandleErrorResponse(resp)
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
	return HandleErrorResponse(resp)
}

func (bs *blobStatter) SetDescriptor(ctx context.Context, dgst digest.Digest, desc distribution.Descriptor) error {
	return nil
}
