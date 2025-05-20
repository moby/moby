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

package local

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/containerd/containerd/v2/core/transfer"
	"github.com/containerd/containerd/v2/core/unpack"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (ts *localTransferService) pull(ctx context.Context, ir transfer.ImageFetcher, is transfer.ImageStorer, tops *transfer.Config) error {
	ctx, done, err := ts.withLease(ctx)
	if err != nil {
		return err
	}
	defer done(ctx)

	if tops.Progress != nil {
		tops.Progress(transfer.Progress{
			Event: fmt.Sprintf("Resolving from %s", ir),
		})
	}

	name, desc, err := ir.Resolve(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve image: %w", err)
	}
	if desc.MediaType == images.MediaTypeDockerSchema1Manifest {
		// Explicitly call out schema 1 as deprecated and not supported
		return fmt.Errorf("schema 1 image manifests are no longer supported: %w", errdefs.ErrInvalidArgument)
	}

	// Verify image before pulling.
	for vfName, vf := range ts.config.Verifiers {
		logger := log.G(ctx).WithFields(log.Fields{
			"name":     name,
			"digest":   desc.Digest.String(),
			"verifier": vfName,
		})
		logger.Debug("Verifying image pull")

		jdg, err := vf.VerifyImage(ctx, name, desc)
		if err != nil {
			logger.WithError(err).Error("No judgement received from verifier")
			return fmt.Errorf("blocking pull of %v with digest %v: image verifier %v returned error: %w", name, desc.Digest.String(), vfName, err)
		}
		logger = logger.WithFields(log.Fields{
			"ok":     jdg.OK,
			"reason": jdg.Reason,
		})

		if !jdg.OK {
			logger.Warn("Image verifier blocked pull")
			return fmt.Errorf("image verifier %s blocked pull of %v with digest %v for reason: %v", vfName, name, desc.Digest.String(), jdg.Reason)
		}
		logger.Debug("Image verifier allowed pull")
	}

	// TODO: Handle already exists
	if tops.Progress != nil {
		tops.Progress(transfer.Progress{
			Event: fmt.Sprintf("Pulling from %s", ir),
		})
		tops.Progress(transfer.Progress{
			Event: "fetching image content",
			Name:  name,
			// Digest: img.Target.Digest.String(),
		})
	}

	fetcher, err := ir.Fetcher(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get fetcher for %q: %w", name, err)
	}

	var (
		handler images.Handler

		baseHandlers []images.Handler

		unpacker *unpack.Unpacker

		// has a config media type bug (distribution#1622)
		hasMediaTypeBug1622 bool

		store           = ts.content
		progressTracker *ProgressTracker
	)

	ctx, cancel := context.WithCancel(ctx)
	if tops.Progress != nil {
		progressTracker = NewProgressTracker(name, "downloading") // Pass in first name as root
		go progressTracker.HandleProgress(ctx, tops.Progress, NewContentStatusTracker(store))
		defer progressTracker.Wait()
	}
	defer cancel()

	// Get all the children for a descriptor
	childrenHandler := images.ChildrenHandler(store)

	if f, ok := is.(transfer.ImageFilterer); ok {
		childrenHandler = f.ImageFilter(childrenHandler, store)
	}

	checkNeedsFix := images.HandlerFunc(
		func(_ context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			// set to true if there is application/octet-stream media type
			if desc.MediaType == docker.LegacyConfigMediaType {
				hasMediaTypeBug1622 = true
			}

			return []ocispec.Descriptor{}, nil
		},
	)

	appendDistSrcLabelHandler, err := docker.AppendDistributionSourceLabel(store, name)
	if err != nil {
		return err
	}

	// Set up baseHandlers from service configuration if present or create a new one
	if ts.config.BaseHandlers != nil {
		baseHandlers = ts.config.BaseHandlers
	} else {
		baseHandlers = []images.Handler{}
	}

	if tops.Progress != nil {
		baseHandlers = append(baseHandlers, images.HandlerFunc(
			func(_ context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
				progressTracker.Add(desc)

				return []ocispec.Descriptor{}, nil
			},
		))

		baseChildrenHandler := childrenHandler
		childrenHandler = images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) (children []ocispec.Descriptor, err error) {
			children, err = baseChildrenHandler(ctx, desc)
			if err != nil {
				return
			}
			progressTracker.AddChildren(desc, children)
			return
		})
	}

	handler = images.Handlers(append(baseHandlers,
		fetchHandler(store, fetcher, progressTracker),
		checkNeedsFix,
		childrenHandler, // List children to track hierarchy
		appendDistSrcLabelHandler,
	)...)

	// First find suitable platforms to unpack into
	// If image storer is also an unpacker type, i.e implemented UnpackPlatforms() func
	if iu, ok := is.(transfer.ImageUnpacker); ok {
		unpacks := iu.UnpackPlatforms()
		if len(unpacks) > 0 {
			uopts := []unpack.UnpackerOpt{}
			// Only unpack if requested unpackconfig matches default/supported unpackconfigs
			for _, u := range unpacks {
				matched, mu := getSupportedPlatform(u, ts.config.UnpackPlatforms)
				if matched {
					uopts = append(uopts, unpack.WithUnpackPlatform(mu))
				}
			}

			if ts.limiterD != nil {
				uopts = append(uopts, unpack.WithLimiter(ts.limiterD))
			}

			if ts.config.DuplicationSuppressor != nil {
				uopts = append(uopts, unpack.WithDuplicationSuppressor(ts.config.DuplicationSuppressor))
			}

			unpacker, err = unpack.NewUnpacker(ctx, ts.content, uopts...)
			if err != nil {
				return fmt.Errorf("unable to initialize unpacker: %w", err)
			}
			handler = unpacker.Unpack(handler)
		}
	}

	if err := images.Dispatch(ctx, handler, ts.limiterD, desc); err != nil {
		if unpacker != nil {
			// wait for unpacker to cleanup
			unpacker.Wait()
		}
		return err
	}

	// NOTE(fuweid): unpacker defers blobs download. before create image
	// record in ImageService, should wait for unpacking(including blobs
	// download).
	if unpacker != nil {
		if _, err = unpacker.Wait(); err != nil {
			return err
		}
		// TODO: Check results to make sure unpack was successful
	}

	if hasMediaTypeBug1622 {
		if desc, err = docker.ConvertManifest(ctx, store, desc); err != nil {
			return err
		}
	}

	imgs, err := is.Store(ctx, desc, ts.images)
	if err != nil {
		return err
	}

	if tops.Progress != nil {
		for _, img := range imgs {
			tops.Progress(transfer.Progress{
				Event: "saved",
				Name:  img.Name,
			})
		}
	}

	if tops.Progress != nil {
		tops.Progress(transfer.Progress{
			Event: fmt.Sprintf("Completed pull from %s", ir),
		})
	}

	return nil
}

func fetchHandler(ingester content.Ingester, fetcher remotes.Fetcher, pt *ProgressTracker) images.HandlerFunc {
	return func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
			"digest":    desc.Digest,
			"mediatype": desc.MediaType,
			"size":      desc.Size,
		}))

		if desc.MediaType == images.MediaTypeDockerSchema1Manifest {
			return nil, fmt.Errorf("%v not supported", desc.MediaType)
		}
		err := remotes.Fetch(ctx, ingester, fetcher, desc)
		if errdefs.IsAlreadyExists(err) {
			pt.MarkExists(desc)
			return nil, nil
		}
		return nil, err
	}
}

// getSupportedPlatform returns a matched platform comparing input UnpackConfiguration to the supported platform/snapshotter combinations
// If input platform didn't specify snapshotter, default will be used if there is a match on platform.
func getSupportedPlatform(uc transfer.UnpackConfiguration, supportedPlatforms []unpack.Platform) (bool, unpack.Platform) {
	var u unpack.Platform
	for _, sp := range supportedPlatforms {
		// If both platform and snapshotter match, return the supportPlatform
		// If platform matched and SnapshotterKey is empty, we assume client didn't pass SnapshotterKey
		// use default Snapshotter
		if sp.Platform.Match(uc.Platform) {
			// Assume sp.SnapshotterKey is not empty
			if uc.Snapshotter == sp.SnapshotterKey {
				return true, sp
			} else if uc.Snapshotter == "" && sp.SnapshotterKey == defaults.DefaultSnapshotter {
				return true, sp
			}
		}
	}
	return false, u
}
