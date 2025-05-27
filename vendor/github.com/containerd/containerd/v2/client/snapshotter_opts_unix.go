//go:build !windows

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

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/internal/userns"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const (
	capaRemapIDs     = "remap-ids"
	capaOnlyRemapIDs = "only-remap-ids"
)

// WithRemapperLabels creates the labels used by any supporting snapshotter
// to shift the filesystem ownership (user namespace mapping) automatically; currently
// supported by the fuse-overlayfs and overlay snapshotters
func WithRemapperLabels(ctrUID, hostUID, ctrGID, hostGID, length uint32) snapshots.Opt {
	return snapshots.WithLabels(map[string]string{
		snapshots.LabelSnapshotUIDMapping: fmt.Sprintf("%d:%d:%d", ctrUID, hostUID, length),
		snapshots.LabelSnapshotGIDMapping: fmt.Sprintf("%d:%d:%d", ctrGID, hostGID, length)})
}

func resolveSnapshotOptions(ctx context.Context, client *Client, snapshotterName string, snapshotter snapshots.Snapshotter, parent string, opts ...snapshots.Opt) (string, error) {
	capabs, err := client.GetSnapshotterCapabilities(ctx, snapshotterName)
	if err != nil {
		return "", err
	}

	for _, capab := range capabs {
		if capab == capaRemapIDs {
			// Snapshotter supports ID remapping, we don't need to do anything.
			return parent, nil
		}
	}

	var local snapshots.Info
	for _, opt := range opts {
		opt(&local)
	}

	needsRemap := false
	var uidMapLabel, gidMapLabel string

	if value, ok := local.Labels[snapshots.LabelSnapshotUIDMapping]; ok {
		needsRemap = true
		uidMapLabel = value
	}
	if value, ok := local.Labels[snapshots.LabelSnapshotGIDMapping]; ok {
		needsRemap = true
		gidMapLabel = value
	}

	if !needsRemap {
		return parent, nil
	}

	capaOnlyRemap := false
	for _, capa := range capabs {
		if capa == capaOnlyRemapIDs {
			capaOnlyRemap = true
		}
	}

	if capaOnlyRemap {
		return "", fmt.Errorf("snapshotter %q doesn't support idmap mounts on this host, configure `slow_chown` to allow a slower and expensive fallback", snapshotterName)
	}

	var uidMap, gidMap specs.LinuxIDMapping
	_, err = fmt.Sscanf(uidMapLabel, "%d:%d:%d", &uidMap.ContainerID, &uidMap.HostID, &uidMap.Size)
	if err != nil {
		return "", fmt.Errorf("uidMapLabel unparsable: %w", err)
	}
	_, err = fmt.Sscanf(gidMapLabel, "%d:%d:%d", &gidMap.ContainerID, &gidMap.HostID, &gidMap.Size)
	if err != nil {
		return "", fmt.Errorf("gidMapLabel unparsable: %w", err)
	}

	if uidMap.ContainerID != 0 || gidMap.ContainerID != 0 {
		return "", fmt.Errorf("Container UID/GID of 0 only supported currently (%d/%d)", uidMap.ContainerID, gidMap.ContainerID)
	}

	rsn := remappedSnapshot{
		Parent: parent,
		IDMap: userns.IDMap{
			UidMap: []specs.LinuxIDMapping{uidMap},
			GidMap: []specs.LinuxIDMapping{gidMap},
		},
	}
	usernsID, err := rsn.ID()
	if err != nil {
		return "", fmt.Errorf("failed to remap snapshot: %w", err)
	}

	if _, err := snapshotter.Stat(ctx, usernsID); err == nil {
		return usernsID, nil
	}
	mounts, err := snapshotter.Prepare(ctx, usernsID+"-remap", parent)
	if err != nil {
		return "", err
	}

	if err := remapRootFS(ctx, mounts, rsn.IDMap); err != nil {
		snapshotter.Remove(ctx, usernsID+"-remap")
		return "", err
	}
	if err := snapshotter.Commit(ctx, usernsID, usernsID+"-remap"); err != nil {
		return "", err
	}

	return usernsID, nil
}

type remappedSnapshot struct {
	Parent string       `json:"Parent"`
	IDMap  userns.IDMap `json:"IDMap"`
}

func (s *remappedSnapshot) ID() (string, error) {
	compare := func(a, b specs.LinuxIDMapping) int {
		if a.ContainerID < b.ContainerID {
			return -1
		} else if a.ContainerID == b.ContainerID {
			return 0
		}
		return 1
	}
	slices.SortStableFunc(s.IDMap.UidMap, compare)
	slices.SortStableFunc(s.IDMap.GidMap, compare)

	buf, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return digest.FromBytes(buf).String(), nil
}
