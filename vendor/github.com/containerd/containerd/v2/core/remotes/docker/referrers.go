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
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (r dockerFetcher) FetchReferrers(ctx context.Context, dgst digest.Digest, artifactTypes ...string) ([]ocispec.Descriptor, error) {
	rc, size, err := r.openReferrers(ctx, dgst, artifactTypes...)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var index ocispec.Index
	if err := json.NewDecoder(io.LimitReader(rc, size)).Decode(&index); err != nil {
		return nil, fmt.Errorf("failed to decode referrers index: %w", err)
	}
	return index.Manifests, nil
}

func (r dockerFetcher) openReferrers(ctx context.Context, dgst digest.Digest, artifactTypes ...string) (io.ReadCloser, int64, error) {
	mediaType := ocispec.MediaTypeImageIndex
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("digest", dgst))

	hosts := r.filterHosts(HostCapabilityReferrers)
	fallbackHosts := r.filterHosts(HostCapabilityResolve)
	if len(hosts) == 0 && len(fallbackHosts) == 0 {
		return nil, 0, fmt.Errorf("no pull hosts: %w", errdefs.ErrNotFound)
	}

	ctx, err := ContextWithRepositoryScope(ctx, r.refspec, false)
	if err != nil {
		return nil, 0, err
	}

	for i, host := range hosts {
		req := r.request(host, http.MethodGet, "referrers", dgst.String())
		for _, artifactType := range artifactTypes {
			if err := req.addQuery("artifactType", artifactType); err != nil {
				return nil, 0, err
			}
		}
		if err := req.addNamespace(r.refspec.Hostname()); err != nil {
			return nil, 0, err
		}

		rc, cl, err := r.open(ctx, req, mediaType, 0, i == len(hosts)-1)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				return nil, 0, err
			}
		} else {
			return rc, cl, nil
		}
	}

	for i, host := range fallbackHosts {
		req := r.request(host, http.MethodGet, "manifests", strings.Replace(dgst.String(), ":", "-", 1))
		if err := req.addNamespace(r.refspec.Hostname()); err != nil {
			return nil, 0, err
		}
		rc, cl, err := r.open(ctx, req, mediaType, 0, i == len(fallbackHosts)-1)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				return nil, 0, err
			}
		} else {
			return rc, cl, nil
		}
	}

	return nil, 0, fmt.Errorf("could not be found at any host: %w", errdefs.ErrNotFound)
}
