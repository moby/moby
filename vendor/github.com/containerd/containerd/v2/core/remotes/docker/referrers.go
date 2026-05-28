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
	"strings"

	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (r dockerFetcher) FetchReferrers(ctx context.Context, dgst digest.Digest, opts ...remotes.FetchReferrersOpt) ([]ocispec.Descriptor, error) {
	var config remotes.FetchReferrersConfig
	for _, opt := range opts {
		opt(ctx, &config)
	}
	rc, size, err := r.openReferrers(ctx, dgst, config)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return []ocispec.Descriptor{}, nil
		}
		return nil, err
	}
	defer rc.Close()
	if size < 0 {
		size = MaxManifestSize
	} else if size > MaxManifestSize {
		return nil, fmt.Errorf("referrers index size %d exceeds maximum allowed %d: %w", size, MaxManifestSize, errdefs.ErrNotFound)
	}

	var index ocispec.Index
	dec := json.NewDecoder(io.LimitReader(rc, size))
	if err := dec.Decode(&index); err != nil {
		return nil, fmt.Errorf("failed to decode referrers index: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("unexpected data after JSON object")
	}

	if len(config.ArtifactTypes) == 0 {
		return index.Manifests, nil
	}

	var referrers []ocispec.Descriptor
	tFilter := map[string]struct{}{}
	for _, t := range config.ArtifactTypes {
		tFilter[t] = struct{}{}
	}
	for _, desc := range index.Manifests {
		if _, ok := tFilter[desc.ArtifactType]; ok {
			referrers = append(referrers, desc)
		}
	}
	return referrers, nil
}

func (r dockerFetcher) openReferrers(ctx context.Context, dgst digest.Digest, config remotes.FetchReferrersConfig) (io.ReadCloser, int64, error) {
	mediaType := ocispec.MediaTypeImageIndex
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("digest", dgst))

	hosts := r.filterHosts(HostCapabilityReferrers)
	var fallbackHosts []RegistryHost
	if len(hosts) == 0 {
		fallbackHosts = r.filterHosts(HostCapabilityResolve)
		if len(fallbackHosts) == 0 {
			return nil, 0, fmt.Errorf("no referrers hosts: %w", errdefs.ErrNotFound)
		}
	} else {
		// If referrers are defined, use same hosts for fallback
		fallbackHosts = hosts
	}

	ctx, err := ContextWithRepositoryScope(ctx, r.refspec, false)
	if err != nil {
		return nil, 0, err
	}

	var firstErr error
	for i, host := range hosts {
		req := r.request(host, http.MethodGet, "referrers", dgst.String())
		for _, artifactType := range config.ArtifactTypes {
			if err := req.addQuery("artifactType", artifactType); err != nil {
				return nil, 0, err
			}
		}
		for k, vs := range config.QueryFilters {
			for _, v := range vs {
				if err := req.addQuery(k, v); err != nil {
					return nil, 0, err
				}
			}
		}
		if err := req.addNamespace(r.refspec.Hostname()); err != nil {
			return nil, 0, err
		}

		rc, cl, err := r.open(ctx, req, mediaType, 0, i == len(hosts)-1)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				log.G(ctx).WithError(err).WithField("host", host.Host).Debug("error fetching referrers")
				if firstErr == nil {
					firstErr = err
				}
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
			if errdefs.IsNotFound(err) {
				// Equivalent to empty referrers list
				firstErr = err
				break
			}
			log.G(ctx).WithError(err).WithField("host", host.Host).Debug("error fetching referrers via fallback")
			if firstErr == nil {
				firstErr = err
			}
		} else {
			return rc, cl, nil
		}
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("could not be found at any host: %w", errdefs.ErrNotFound)
	}

	return nil, 0, firstErr
}
