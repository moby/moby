/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker/schema1"
	"github.com/containerd/containerd/version"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context/ctxhttp"
)

var (
	// ErrNoToken is returned if a request is successful but the body does not
	// contain an authorization token.
	ErrNoToken = errors.New("authorization server did not include a token in the response")

	// ErrInvalidAuthorization is used when credentials are passed to a server but
	// those credentials are rejected.
	ErrInvalidAuthorization = errors.New("authorization failed")

	// MaxManifestSize represents the largest size accepted from a registry
	// during resolution. Larger manifests may be accepted using a
	// resolution method other than the registry.
	//
	// NOTE: The max supported layers by some runtimes is 128 and individual
	// layers will not contribute more than 256 bytes, making a
	// reasonable limit for a large image manifests of 32K bytes.
	// 4M bytes represents a much larger upper bound for images which may
	// contain large annotations or be non-images. A proper manifest
	// design puts large metadata in subobjects, as is consistent the
	// intent of the manifest design.
	MaxManifestSize int64 = 4 * 1048 * 1048
)

// Authorizer is used to authorize HTTP requests based on 401 HTTP responses.
// An Authorizer is responsible for caching tokens or credentials used by
// requests.
type Authorizer interface {
	// Authorize sets the appropriate `Authorization` header on the given
	// request.
	//
	// If no authorization is found for the request, the request remains
	// unmodified. It may also add an `Authorization` header as
	//  "bearer <some bearer token>"
	//  "basic <base64 encoded credentials>"
	Authorize(context.Context, *http.Request) error

	// AddResponses adds a 401 response for the authorizer to consider when
	// authorizing requests. The last response should be unauthorized and
	// the previous requests are used to consider redirects and retries
	// that may have led to the 401.
	//
	// If response is not handled, returns `ErrNotImplemented`
	AddResponses(context.Context, []*http.Response) error
}

// ResolverOptions are used to configured a new Docker register resolver
type ResolverOptions struct {
	// Hosts returns registry host configurations for a namespace.
	Hosts RegistryHosts

	// Headers are the HTTP request header fields sent by the resolver
	Headers http.Header

	// Tracker is used to track uploads to the registry. This is used
	// since the registry does not have upload tracking and the existing
	// mechanism for getting blob upload status is expensive.
	Tracker StatusTracker

	// Authorizer is used to authorize registry requests
	// Deprecated: use Hosts
	Authorizer Authorizer

	// Credentials provides username and secret given a host.
	// If username is empty but a secret is given, that secret
	// is interpreted as a long lived token.
	// Deprecated: use Hosts
	Credentials func(string) (string, string, error)

	// Host provides the hostname given a namespace.
	// Deprecated: use Hosts
	Host func(string) (string, error)

	// PlainHTTP specifies to use plain http and not https
	// Deprecated: use Hosts
	PlainHTTP bool

	// Client is the http client to used when making registry requests
	// Deprecated: use Hosts
	Client *http.Client
}

// DefaultHost is the default host function.
func DefaultHost(ns string) (string, error) {
	if ns == "docker.io" {
		return "registry-1.docker.io", nil
	}
	return ns, nil
}

type dockerResolver struct {
	hosts         RegistryHosts
	header        http.Header
	resolveHeader http.Header
	tracker       StatusTracker
}

// NewResolver returns a new resolver to a Docker registry
func NewResolver(options ResolverOptions) remotes.Resolver {
	if options.Tracker == nil {
		options.Tracker = NewInMemoryTracker()
	}

	if options.Headers == nil {
		options.Headers = make(http.Header)
	}
	if _, ok := options.Headers["User-Agent"]; !ok {
		options.Headers.Set("User-Agent", "containerd/"+version.Version)
	}

	resolveHeader := http.Header{}
	if _, ok := options.Headers["Accept"]; !ok {
		// set headers for all the types we support for resolution.
		resolveHeader.Set("Accept", strings.Join([]string{
			images.MediaTypeDockerSchema2Manifest,
			images.MediaTypeDockerSchema2ManifestList,
			ocispec.MediaTypeImageManifest,
			ocispec.MediaTypeImageIndex, "*/*"}, ", "))
	} else {
		resolveHeader["Accept"] = options.Headers["Accept"]
		delete(options.Headers, "Accept")
	}

	if options.Hosts == nil {
		opts := []RegistryOpt{}
		if options.Host != nil {
			opts = append(opts, WithHostTranslator(options.Host))
		}

		if options.Authorizer == nil {
			options.Authorizer = NewDockerAuthorizer(
				WithAuthClient(options.Client),
				WithAuthHeader(options.Headers),
				WithAuthCreds(options.Credentials))
		}
		opts = append(opts, WithAuthorizer(options.Authorizer))

		if options.Client != nil {
			opts = append(opts, WithClient(options.Client))
		}
		if options.PlainHTTP {
			opts = append(opts, WithPlainHTTP(MatchAllHosts))
		} else {
			opts = append(opts, WithPlainHTTP(MatchLocalhost))
		}
		options.Hosts = ConfigureDefaultRegistries(opts...)
	}
	return &dockerResolver{
		hosts:         options.Hosts,
		header:        options.Headers,
		resolveHeader: resolveHeader,
		tracker:       options.Tracker,
	}
}

func getManifestMediaType(resp *http.Response) string {
	// Strip encoding data (manifests should always be ascii JSON)
	contentType := resp.Header.Get("Content-Type")
	if sp := strings.IndexByte(contentType, ';'); sp != -1 {
		contentType = contentType[0:sp]
	}

	// As of Apr 30 2019 the registry.access.redhat.com registry does not specify
	// the content type of any data but uses schema1 manifests.
	if contentType == "text/plain" {
		contentType = images.MediaTypeDockerSchema1Manifest
	}
	return contentType
}

type countingReader struct {
	reader    io.Reader
	bytesRead int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.bytesRead += int64(n)
	return n, err
}

var _ remotes.Resolver = &dockerResolver{}

func (r *dockerResolver) Resolve(ctx context.Context, ref string) (string, ocispec.Descriptor, error) {
	refspec, err := reference.Parse(ref)
	if err != nil {
		return "", ocispec.Descriptor{}, err
	}

	if refspec.Object == "" {
		return "", ocispec.Descriptor{}, reference.ErrObjectRequired
	}

	base, err := r.base(refspec)
	if err != nil {
		return "", ocispec.Descriptor{}, err
	}

	var (
		lastErr error
		paths   [][]string
		dgst    = refspec.Digest()
		caps    = HostCapabilityPull
	)

	if dgst != "" {
		if err := dgst.Validate(); err != nil {
			// need to fail here, since we can't actually resolve the invalid
			// digest.
			return "", ocispec.Descriptor{}, err
		}

		// turns out, we have a valid digest, make a url.
		paths = append(paths, []string{"manifests", dgst.String()})

		// fallback to blobs on not found.
		paths = append(paths, []string{"blobs", dgst.String()})
	} else {
		// Add
		paths = append(paths, []string{"manifests", refspec.Object})
		caps |= HostCapabilityResolve
	}

	hosts := base.filterHosts(caps)
	if len(hosts) == 0 {
		return "", ocispec.Descriptor{}, errors.Wrap(errdefs.ErrNotFound, "no resolve hosts")
	}

	ctx, err = contextWithRepositoryScope(ctx, refspec, false)
	if err != nil {
		return "", ocispec.Descriptor{}, err
	}

	for _, u := range paths {
		for _, host := range hosts {
			ctx := log.WithLogger(ctx, log.G(ctx).WithField("host", host.Host))

			req := base.request(host, http.MethodHead, u...)
			for key, value := range r.resolveHeader {
				req.header[key] = append(req.header[key], value...)
			}

			log.G(ctx).Debug("resolving")
			resp, err := req.doWithRetries(ctx, nil)
			if err != nil {
				if errors.Is(err, ErrInvalidAuthorization) {
					err = errors.Wrapf(err, "pull access denied, repository does not exist or may require authorization")
				}
				// Store the error for referencing later
				if lastErr == nil {
					lastErr = err
				}
				continue // try another host
			}
			resp.Body.Close() // don't care about body contents.

			if resp.StatusCode > 299 {
				if resp.StatusCode == http.StatusNotFound {
					continue
				}
				return "", ocispec.Descriptor{}, errors.Errorf("unexpected status code %v: %v", u, resp.Status)
			}
			size := resp.ContentLength
			contentType := getManifestMediaType(resp)

			// if no digest was provided, then only a resolve
			// trusted registry was contacted, in this case use
			// the digest header (or content from GET)
			if dgst == "" {
				// this is the only point at which we trust the registry. we use the
				// content headers to assemble a descriptor for the name. when this becomes
				// more robust, we mostly get this information from a secure trust store.
				dgstHeader := digest.Digest(resp.Header.Get("Docker-Content-Digest"))

				if dgstHeader != "" && size != -1 {
					if err := dgstHeader.Validate(); err != nil {
						return "", ocispec.Descriptor{}, errors.Wrapf(err, "%q in header not a valid digest", dgstHeader)
					}
					dgst = dgstHeader
				}
			}
			if dgst == "" || size == -1 {
				log.G(ctx).Debug("no Docker-Content-Digest header, fetching manifest instead")

				req = base.request(host, http.MethodGet, u...)
				for key, value := range r.resolveHeader {
					req.header[key] = append(req.header[key], value...)
				}

				resp, err := req.doWithRetries(ctx, nil)
				if err != nil {
					return "", ocispec.Descriptor{}, err
				}
				defer resp.Body.Close()

				bodyReader := countingReader{reader: resp.Body}

				contentType = getManifestMediaType(resp)
				if dgst == "" {
					if contentType == images.MediaTypeDockerSchema1Manifest {
						b, err := schema1.ReadStripSignature(&bodyReader)
						if err != nil {
							return "", ocispec.Descriptor{}, err
						}

						dgst = digest.FromBytes(b)
					} else {
						dgst, err = digest.FromReader(&bodyReader)
						if err != nil {
							return "", ocispec.Descriptor{}, err
						}
					}
				} else if _, err := io.Copy(ioutil.Discard, &bodyReader); err != nil {
					return "", ocispec.Descriptor{}, err
				}
				size = bodyReader.bytesRead
			}
			// Prevent resolving to excessively large manifests
			if size > MaxManifestSize {
				if lastErr == nil {
					lastErr = errors.Wrapf(errdefs.ErrNotFound, "rejecting %d byte manifest for %s", size, ref)
				}
				continue
			}

			desc := ocispec.Descriptor{
				Digest:    dgst,
				MediaType: contentType,
				Size:      size,
			}

			log.G(ctx).WithField("desc.digest", desc.Digest).Debug("resolved")
			return ref, desc, nil
		}
	}

	if lastErr == nil {
		lastErr = errors.Wrap(errdefs.ErrNotFound, ref)
	}

	return "", ocispec.Descriptor{}, lastErr
}

func (r *dockerResolver) Fetcher(ctx context.Context, ref string) (remotes.Fetcher, error) {
	refspec, err := reference.Parse(ref)
	if err != nil {
		return nil, err
	}

	base, err := r.base(refspec)
	if err != nil {
		return nil, err
	}

	return dockerFetcher{
		dockerBase: base,
	}, nil
}

func (r *dockerResolver) Pusher(ctx context.Context, ref string) (remotes.Pusher, error) {
	refspec, err := reference.Parse(ref)
	if err != nil {
		return nil, err
	}

	base, err := r.base(refspec)
	if err != nil {
		return nil, err
	}

	return dockerPusher{
		dockerBase: base,
		object:     refspec.Object,
		tracker:    r.tracker,
	}, nil
}

type dockerBase struct {
	refspec   reference.Spec
	namespace string
	hosts     []RegistryHost
	header    http.Header
}

func (r *dockerResolver) base(refspec reference.Spec) (*dockerBase, error) {
	host := refspec.Hostname()
	hosts, err := r.hosts(host)
	if err != nil {
		return nil, err
	}
	return &dockerBase{
		refspec:   refspec,
		namespace: strings.TrimPrefix(refspec.Locator, host+"/"),
		hosts:     hosts,
		header:    r.header,
	}, nil
}

func (r *dockerBase) filterHosts(caps HostCapabilities) (hosts []RegistryHost) {
	for _, host := range r.hosts {
		if host.Capabilities.Has(caps) {
			hosts = append(hosts, host)
		}
	}
	return
}

func (r *dockerBase) request(host RegistryHost, method string, ps ...string) *request {
	header := http.Header{}
	for key, value := range r.header {
		header[key] = append(header[key], value...)
	}
	for key, value := range host.Header {
		header[key] = append(header[key], value...)
	}
	parts := append([]string{"/", host.Path, r.namespace}, ps...)
	p := path.Join(parts...)
	// Join strips trailing slash, re-add ending "/" if included
	if len(parts) > 0 && strings.HasSuffix(parts[len(parts)-1], "/") {
		p = p + "/"
	}
	return &request{
		method: method,
		path:   p,
		header: header,
		host:   host,
	}
}

func (r *request) authorize(ctx context.Context, req *http.Request) error {
	// Check if has header for host
	if r.host.Authorizer != nil {
		if err := r.host.Authorizer.Authorize(ctx, req); err != nil {
			return err
		}
	}

	return nil
}

type request struct {
	method string
	path   string
	header http.Header
	host   RegistryHost
	body   func() (io.ReadCloser, error)
	size   int64
}

func (r *request) do(ctx context.Context) (*http.Response, error) {
	u := r.host.Scheme + "://" + r.host.Host + r.path
	req, err := http.NewRequest(r.method, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header = r.header
	if r.body != nil {
		body, err := r.body()
		if err != nil {
			return nil, err
		}
		req.Body = body
		req.GetBody = r.body
		if r.size > 0 {
			req.ContentLength = r.size
		}
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithField("url", u))
	log.G(ctx).WithFields(requestFields(req)).Debug("do request")
	if err := r.authorize(ctx, req); err != nil {
		return nil, errors.Wrap(err, "failed to authorize")
	}
	resp, err := ctxhttp.Do(ctx, r.host.Client, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to do request")
	}
	log.G(ctx).WithFields(responseFields(resp)).Debug("fetch response received")
	return resp, nil
}

func (r *request) doWithRetries(ctx context.Context, responses []*http.Response) (*http.Response, error) {
	resp, err := r.do(ctx)
	if err != nil {
		return nil, err
	}

	responses = append(responses, resp)
	retry, err := r.retryRequest(ctx, responses)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	if retry {
		resp.Body.Close()
		return r.doWithRetries(ctx, responses)
	}
	return resp, err
}

func (r *request) retryRequest(ctx context.Context, responses []*http.Response) (bool, error) {
	if len(responses) > 5 {
		return false, nil
	}
	last := responses[len(responses)-1]
	switch last.StatusCode {
	case http.StatusUnauthorized:
		log.G(ctx).WithField("header", last.Header.Get("WWW-Authenticate")).Debug("Unauthorized")
		if r.host.Authorizer != nil {
			if err := r.host.Authorizer.AddResponses(ctx, responses); err == nil {
				return true, nil
			} else if !errdefs.IsNotImplemented(err) {
				return false, err
			}
		}

		return false, nil
	case http.StatusMethodNotAllowed:
		// Support registries which have not properly implemented the HEAD method for
		// manifests endpoint
		if r.method == http.MethodHead && strings.Contains(r.path, "/manifests/") {
			r.method = http.MethodGet
			return true, nil
		}
	case http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true, nil
	}

	// TODO: Handle 50x errors accounting for attempt history
	return false, nil
}

func (r *request) String() string {
	return r.host.Scheme + "://" + r.host.Host + r.path
}

func requestFields(req *http.Request) logrus.Fields {
	fields := map[string]interface{}{
		"request.method": req.Method,
	}
	for k, vals := range req.Header {
		k = strings.ToLower(k)
		if k == "authorization" {
			continue
		}
		for i, v := range vals {
			field := "request.header." + k
			if i > 0 {
				field = fmt.Sprintf("%s.%d", field, i)
			}
			fields[field] = v
		}
	}

	return logrus.Fields(fields)
}

func responseFields(resp *http.Response) logrus.Fields {
	fields := map[string]interface{}{
		"response.status": resp.Status,
	}
	for k, vals := range resp.Header {
		k = strings.ToLower(k)
		for i, v := range vals {
			field := "response.header." + k
			if i > 0 {
				field = fmt.Sprintf("%s.%d", field, i)
			}
			fields[field] = v
		}
	}

	return logrus.Fields(fields)
}
