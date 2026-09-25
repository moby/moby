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

package manager

import (
	"context"
	"slices"
	"strings"

	"github.com/containerd/log"

	"github.com/containerd/containerd/v2/core/mount"
)

// activationPlan is the result of deciding, for a set of mounts, which
// transforms the mount manager must apply itself and which mounts it must
// perform, honoring the mount types and transforms the caller has claimed.
type activationPlan struct {
	// firstSystemMount is the index of the first mount to return to the
	// caller as a system mount, rather than perform inside the mount
	// manager. -1 means nothing needs the manager at all: the caller should
	// perform every mount itself, unmodified.
	firstSystemMount int

	// transforms[i] are the transforms recorded for mounts[i], in
	// application order (outermost transform first, so the one nearest the
	// original mount type comes first). nil if mounts[i] had no transform
	// prefix.
	transforms [][]mount.Transformer

	// applyCount[i] is how many of transforms[i], from the start, the
	// manager must apply itself: through the last transform in the chain the
	// caller did not claim. Meaningful only for mounts[firstSystemMount],
	// the one mount returned to the caller with some of its own transforms
	// possibly still unapplied; every earlier mount's transforms are always
	// applied in full; they are internal to the manager and never exposed
	// to the caller. A transform, once applied, resolves the input the next
	// one in the chain depends on, so a run of claimed transforms can only
	// be honored as a suffix: transforms strictly after the last unclaimed
	// one in the chain.
	applyCount []int

	// handlers[i] is the Handler to use for mounts[i], or nil to perform a
	// plain system mount.
	handlers []mount.Handler
}

// planActivation decides, for each mount, which transforms the mount manager
// must apply and whether the manager or the caller performs the mount
// itself, honoring the mount types and transforms config.AllowMountTypes and
// config.AllowTransforms claim.
//
// A claimed transform is honored only for mounts[firstSystemMount] in the
// result, and only as a suffix of its transform chain: transforms apply
// outside-in, so an inner transform's input depends on an outer, unclaimed
// transform's output, and the manager must still apply that outer transform
// even though the caller performs the suffix itself. For example, in
// "format/mkdir/overlay", claiming "mkdir" causes the manager to apply
// "format" and hand the caller "mkdir/overlay" to finish; claiming "format"
// alone does nothing, since "mkdir" cannot run without it having already
// run.
func planActivation(ctx context.Context, mounts []mount.Mount, config mount.ActivateOptions, transforms map[string]mount.Transformer, handlers map[string]mount.Handler) activationPlan {
	shouldTransform := func(p string, t string) bool {
		return !slices.Contains(config.AllowTransforms, p) && !slices.Contains(config.AllowMountTypes, t)
	}
	shouldHandle := func(t string) bool {
		return !slices.Contains(config.AllowMountTypes, t)
	}

	plan := activationPlan{firstSystemMount: -1}

	for i := range mounts {
		mountType := mounts[i].Type

		// Check is the source needs transformation, any transform operation requires
		// mounting with the mount manager.
		for transformType, mt, ok := strings.Cut(mountType, "/"); ok; transformType, mt, ok = strings.Cut(mountType, "/") {
			tr, ok := transforms[transformType]
			if !ok {
				log.G(ctx).Warnf("unknown transform %q for mount %v", transformType, mounts[i]) //nolint:gosec // G602: i is bounded by range mounts
				break
			}

			// mustApply reports whether the manager itself must apply this
			// transform, as opposed to the caller having claimed it.
			mustApply := shouldTransform(transformType, mounts[i].Type) //nolint:gosec // G602: i is bounded by range mounts
			if mustApply {
				// At least everything before this must be mounted
				// by the mount manager
				plan.firstSystemMount = i
			}

			if plan.handlers == nil {
				plan.handlers = make([]mount.Handler, len(mounts))
			}
			if plan.transforms == nil {
				plan.transforms = make([][]mount.Transformer, len(mounts))
			}
			if plan.applyCount == nil {
				plan.applyCount = make([]int, len(mounts))
			}

			plan.transforms[i] = append(plan.transforms[i], typeTransformer{
				Transformer: tr,
				mountType:   mt,
			})
			if mustApply {
				// Every transform up to and including this one must be
				// applied by the manager: this one because it is unclaimed,
				// and every one before it because this one's input depends
				// on their output.
				plan.applyCount[i] = len(plan.transforms[i])
			}

			mountType = mt
		}

		var handler mount.Handler
		if handlers != nil {
			handler = handlers[mountType]
		}

		if handler != nil || config.Temporary {
			if plan.handlers == nil {
				plan.handlers = make([]mount.Handler, len(mounts))
			}
			plan.handlers[i] = handler
			if shouldHandle(mountType) || config.Temporary {
				plan.firstSystemMount = i + 1
			}
		}
	}

	return plan
}
