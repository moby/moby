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

package v2

import (
	"context"
	"strings"
	"sync"

	apitypes "github.com/containerd/containerd/api/types"
	"github.com/containerd/log"
)

// deprecatedAllowedMounts is the RuntimeInfo annotation a shim could set to
// declare the mount types and transforms it handles itself, before the
// MountCapabilities bootstrap extension existed.
//
// Deprecated: shims should attach a containerd.types.MountCapabilities
// extension to their bootstrap result instead. This is still consulted, as a
// migration path, for a shim that does not: at least one out-of-tree shim
// (Kata's EROFS snapshotter shim, see
// https://github.com/kata-containers/kata-containers/pull/12763) adopted
// this annotation before the extension existed, and needs time to migrate.
const deprecatedAllowedMounts = "containerd.io/runtime-allow-mounts"

// deprecatedNoAnnotationRuntimes are runtime names known to never have set
// deprecatedAllowedMounts, so querying them for it is skipped entirely. This
// mirrors the shortcut that existed for the same two runtimes before this
// migration path was introduced.
var deprecatedNoAnnotationRuntimes = map[string]bool{
	"io.containerd.runc.v2":   true,
	"io.containerd.runhcs.v1": true,
}

// deprecatedMountCapabilities discovers mount capabilities from the
// deprecated runtime-allow-mounts annotation, for a shim that does not
// attach a MountCapabilities extension to its bootstrap result.
//
// This is a migration path, not a second permanent mechanism: the extension
// is authoritative whenever a shim provides it, and this is consulted only
// as a fallback when it does not. A nil *deprecatedMountCapabilities behaves
// as if nothing is configured, so callers that do not wire up migration
// support, such as tests, need no special case.
type deprecatedMountCapabilities struct {
	// queryRuntimeInfo executes the shim binary in -info mode for the given
	// runtime name. Set to query through a *ShimManager by
	// newDeprecatedMountCapabilities; overridable in tests so they do not
	// need a real shim binary.
	queryRuntimeInfo func(ctx context.Context, runtimeName string) (*apitypes.RuntimeInfo, error)

	// cache is a cache of runtime name -> *apitypes.MountCapabilities. A
	// stored nil means the runtime was already checked and has nothing to
	// migrate, so it is not queried again.
	cache sync.Map
}

// newDeprecatedMountCapabilities returns a deprecatedMountCapabilities that
// queries shims for their deprecated runtime-allow-mounts annotation.
func newDeprecatedMountCapabilities(shims *ShimManager) *deprecatedMountCapabilities {
	return &deprecatedMountCapabilities{
		queryRuntimeInfo: func(ctx context.Context, runtimeName string) (*apitypes.RuntimeInfo, error) {
			return getRuntimeInfo(ctx, shims, &apitypes.RuntimeRequest{RuntimePath: runtimeName})
		},
	}
}

// lookup returns the mount capabilities implied by runtimeName's deprecated
// annotation, or nil if it has none, is a runtime known not to set it, or d
// itself is nil.
//
// A successful lookup is cached, so runtimeName is only ever queried once per
// process lifetime for a successful result, matching the caching this
// migration path replaced. A failed query is not cached, so a transient
// failure to exec the shim binary is retried on the next call rather than
// permanently disabling the migration path for that runtime.
func (d *deprecatedMountCapabilities) lookup(ctx context.Context, runtimeName string) *apitypes.MountCapabilities {
	if d == nil || deprecatedNoAnnotationRuntimes[runtimeName] {
		return nil
	}

	if v, ok := d.cache.Load(runtimeName); ok {
		caps, _ := v.(*apitypes.MountCapabilities)
		return caps
	}

	caps, err := d.query(ctx, runtimeName)
	if err != nil {
		log.G(ctx).WithError(err).WithField("runtime", runtimeName).
			Error("failed to query deprecated runtime-allow-mounts annotation")
		return nil
	}

	d.cache.Store(runtimeName, caps)
	return caps
}

func (d *deprecatedMountCapabilities) query(ctx context.Context, runtimeName string) (*apitypes.MountCapabilities, error) {
	rinfo, err := d.queryRuntimeInfo(ctx, runtimeName)
	if err != nil {
		return nil, err
	}

	v, ok := rinfo.GetAnnotations()[deprecatedAllowedMounts]
	if !ok {
		return nil, nil
	}

	log.G(ctx).WithField("runtime", runtimeName).WithField("value", v).
		Warnf("runtime declares handled mounts with the deprecated %q annotation; "+
			"it should attach a MountCapabilities extension to its bootstrap result instead",
			deprecatedAllowedMounts)

	return deprecatedParseAllowedMounts(v), nil
}

// deprecatedParseAllowedMounts parses the deprecated annotation's
// comma-separated value into a MountCapabilities, using its "<transform>/*"
// convention to distinguish a transform from a mount type.
func deprecatedParseAllowedMounts(v string) *apitypes.MountCapabilities {
	caps := &apitypes.MountCapabilities{}
	for entry := range strings.SplitSeq(v, ",") {
		if entry == "" {
			continue
		}
		if transform, ok := strings.CutSuffix(entry, "/*"); ok {
			caps.Transforms = append(caps.Transforms, transform)
			continue
		}
		caps.Types = append(caps.Types, entry)
	}
	return caps
}
