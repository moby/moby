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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/log"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type dockerFetcher struct {
	*dockerBase
}

func (r dockerFetcher) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("digest", desc.Digest))

	hosts := r.filterHosts(HostCapabilityPull)
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no pull hosts: %w", errdefs.ErrNotFound)
	}

	ctx, err := ContextWithRepositoryScope(ctx, r.refspec, false)
	if err != nil {
		return nil, err
	}

	return newHTTPReadSeeker(desc.Size, func(offset int64) (io.ReadCloser, error) {
		// firstly try fetch via external urls
		for _, us := range desc.URLs {
			u, err := url.Parse(us)
			if err != nil {
				log.G(ctx).WithError(err).Debugf("failed to parse %q", us)
				continue
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				log.G(ctx).Debug("non-http(s) alternative url is unsupported")
				continue
			}
			ctx = log.WithLogger(ctx, log.G(ctx).WithField("url", u))
			log.G(ctx).Info("request")

			// Try this first, parse it
			host := RegistryHost{
				Client:       http.DefaultClient,
				Host:         u.Host,
				Scheme:       u.Scheme,
				Path:         u.Path,
				Capabilities: HostCapabilityPull,
			}
			req := r.request(host, http.MethodGet)
			// Strip namespace from base
			req.path = u.Path
			if u.RawQuery != "" {
				req.path = req.path + "?" + u.RawQuery
			}

			rc, err := r.open(ctx, req, desc.MediaType, offset)
			if err != nil {
				if errdefs.IsNotFound(err) {
					continue // try one of the other urls.
				}

				return nil, err
			}

			return rc, nil
		}

		// Try manifests endpoints for manifests types
		switch desc.MediaType {
		case images.MediaTypeDockerSchema2Manifest, images.MediaTypeDockerSchema2ManifestList,
			images.MediaTypeDockerSchema1Manifest,
			ocispec.MediaTypeImageManifest, ocispec.MediaTypeImageIndex:

			var firstErr error
			for _, host := range r.hosts {
				req := r.request(host, http.MethodGet, "manifests", desc.Digest.String())
				if err := req.addNamespace(r.refspec.Hostname()); err != nil {
					return nil, err
				}

				rc, err := r.open(ctx, req, desc.MediaType, offset)
				if err != nil {
					// Store the error for referencing later
					if firstErr == nil {
						firstErr = err
					}
					continue // try another host
				}

				return rc, nil
			}

			return nil, firstErr
		}

		// Finally use blobs endpoints
		var firstErr error
		for _, host := range r.hosts {
			req := r.request(host, http.MethodGet, "blobs", desc.Digest.String())
			if err := req.addNamespace(r.refspec.Hostname()); err != nil {
				return nil, err
			}

			rc, err := r.open(ctx, req, desc.MediaType, offset)
			if err != nil {
				// Store the error for referencing later
				if firstErr == nil {
					firstErr = err
				}
				continue // try another host
			}

			return rc, nil
		}

		if errdefs.IsNotFound(firstErr) {
			firstErr = fmt.Errorf("could not fetch content descriptor %v (%v) from remote: %w",
				desc.Digest, desc.MediaType, errdefs.ErrNotFound,
			)
		}

		return nil, firstErr

	})
}

func (r dockerFetcher) createGetReq(ctx context.Context, host RegistryHost, ps ...string) (*request, int64, error) {
	headReq := r.request(host, http.MethodHead, ps...)
	if err := headReq.addNamespace(r.refspec.Hostname()); err != nil {
		return nil, 0, err
	}

	headResp, err := headReq.doWithRetries(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	if headResp.Body != nil {
		headResp.Body.Close()
	}
	if headResp.StatusCode > 299 {
		return nil, 0, fmt.Errorf("unexpected HEAD status code %v: %s", headReq.String(), headResp.Status)
	}

	getReq := r.request(host, http.MethodGet, ps...)
	if err := getReq.addNamespace(r.refspec.Hostname()); err != nil {
		return nil, 0, err
	}
	return getReq, headResp.ContentLength, nil
}

func (r dockerFetcher) FetchByDigest(ctx context.Context, dgst digest.Digest) (io.ReadCloser, ocispec.Descriptor, error) {
	var desc ocispec.Descriptor
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("digest", dgst))

	hosts := r.filterHosts(HostCapabilityPull)
	if len(hosts) == 0 {
		return nil, desc, fmt.Errorf("no pull hosts: %w", errdefs.ErrNotFound)
	}

	ctx, err := ContextWithRepositoryScope(ctx, r.refspec, false)
	if err != nil {
		return nil, desc, err
	}

	var (
		getReq   *request
		sz       int64
		firstErr error
	)

	for _, host := range r.hosts {
		getReq, sz, err = r.createGetReq(ctx, host, "blobs", dgst.String())
		if err == nil {
			break
		}
		// Store the error for referencing later
		if firstErr == nil {
			firstErr = err
		}
	}

	if getReq == nil {
		// Fall back to the "manifests" endpoint
		for _, host := range r.hosts {
			getReq, sz, err = r.createGetReq(ctx, host, "manifests", dgst.String())
			if err == nil {
				break
			}
			// Store the error for referencing later
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if getReq == nil {
		if errdefs.IsNotFound(firstErr) {
			firstErr = fmt.Errorf("could not fetch content %v from remote: %w", dgst, errdefs.ErrNotFound)
		}
		if firstErr == nil {
			firstErr = fmt.Errorf("could not fetch content %v from remote: (unknown)", dgst)
		}
		return nil, desc, firstErr
	}

	seeker, err := newHTTPReadSeeker(sz, func(offset int64) (io.ReadCloser, error) {
		return r.open(ctx, getReq, "", offset)
	})
	if err != nil {
		return nil, desc, err
	}

	desc = ocispec.Descriptor{
		MediaType: "application/octet-stream",
		Digest:    dgst,
		Size:      sz,
	}
	return seeker, desc, nil
}

func (r dockerFetcher) open(ctx context.Context, req *request, mediatype string, offset int64) (_ io.ReadCloser, retErr error) {
	if mediatype == "" {
		req.header.Set("Accept", "*/*")
	} else {
		req.header.Set("Accept", strings.Join([]string{mediatype, `*/*`}, ", "))
	}

	if offset > 0 {
		// Note: "Accept-Ranges: bytes" cannot be trusted as some endpoints
		// will return the header without supporting the range. The content
		// range must always be checked.
		req.header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := req.doWithRetries(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			resp.Body.Close()
		}
	}()

	if resp.StatusCode > 299 {
		// TODO(stevvooe): When doing a offset specific request, we should
		// really distinguish between a 206 and a 200. In the case of 200, we
		// can discard the bytes, hiding the seek behavior from the
		// implementation.

		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("content at %v not found: %w", req.String(), errdefs.ErrNotFound)
		}
		var registryErr Errors
		if err := json.NewDecoder(resp.Body).Decode(&registryErr); err != nil || registryErr.Len() < 1 {
			return nil, fmt.Errorf("unexpected status code %v: %v", req.String(), resp.Status)
		}
		return nil, fmt.Errorf("unexpected status code %v: %s - Server message: %s", req.String(), resp.Status, registryErr.Error())
	}
	if offset > 0 {
		cr := resp.Header.Get("content-range")
		if cr != "" {
			if !strings.HasPrefix(cr, fmt.Sprintf("bytes %d-", offset)) {
				return nil, fmt.Errorf("unhandled content range in response: %v", cr)

			}
		} else {
			// TODO: Should any cases where use of content range
			// without the proper header be considered?
			// 206 responses?

			// Discard up to offset
			// Could use buffer pool here but this case should be rare
			n, err := io.Copy(io.Discard, io.LimitReader(resp.Body, offset))
			if err != nil {
				return nil, fmt.Errorf("failed to discard to offset: %w", err)
			}
			if n != offset {
				return nil, errors.New("unable to discard to offset")
			}

		}
	}

	return resp.Body, nil
}
