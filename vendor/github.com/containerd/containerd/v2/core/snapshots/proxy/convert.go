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

package proxy

import (
	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/pkg/protobuf"
)

// KindToProto converts from [Kind] to the protobuf definition [snapshots.Kind].
func KindToProto(kind snapshots.Kind) snapshotsapi.Kind {
	switch kind {
	case snapshots.KindActive:
		return snapshotsapi.Kind_ACTIVE
	case snapshots.KindView:
		return snapshotsapi.Kind_VIEW
	default:
		return snapshotsapi.Kind_COMMITTED
	}
}

// KindFromProto converts from the protobuf definition [snapshots.Kind] to
// [Kind].
func KindFromProto(kind snapshotsapi.Kind) snapshots.Kind {
	switch kind {
	case snapshotsapi.Kind_ACTIVE:
		return snapshots.KindActive
	case snapshotsapi.Kind_VIEW:
		return snapshots.KindView
	default:
		return snapshots.KindCommitted
	}
}

// InfoToProto converts from [Info] to the protobuf definition [snapshots.Info].
func InfoToProto(info snapshots.Info) *snapshotsapi.Info {
	return &snapshotsapi.Info{
		Name:      info.Name,
		Parent:    info.Parent,
		Kind:      KindToProto(info.Kind),
		CreatedAt: protobuf.ToTimestamp(info.Created),
		UpdatedAt: protobuf.ToTimestamp(info.Updated),
		Labels:    info.Labels,
	}
}

// InfoFromProto converts from the protobuf definition [snapshots.Info] to
// [Info].
func InfoFromProto(info *snapshotsapi.Info) snapshots.Info {
	return snapshots.Info{
		Name:    info.Name,
		Parent:  info.Parent,
		Kind:    KindFromProto(info.Kind),
		Created: protobuf.FromTimestamp(info.CreatedAt),
		Updated: protobuf.FromTimestamp(info.UpdatedAt),
		Labels:  info.Labels,
	}
}

// UsageFromProto converts from the protobuf definition [snapshots.Usage] to
// [Usage].
func UsageFromProto(resp *snapshotsapi.UsageResponse) snapshots.Usage {
	return snapshots.Usage{
		Inodes: resp.Inodes,
		Size:   resp.Size,
	}
}

// UsageToProto converts from [Usage] to the protobuf definition [snapshots.Usage].
func UsageToProto(usage snapshots.Usage) *snapshotsapi.UsageResponse {
	return &snapshotsapi.UsageResponse{
		Inodes: usage.Inodes,
		Size:   usage.Size,
	}
}
