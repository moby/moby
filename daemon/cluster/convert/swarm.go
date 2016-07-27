package convert

import (
	"fmt"
	"strings"
	"time"

	types "github.com/docker/engine-api/types/swarm"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/protobuf/ptypes"
)

// SwarmFromGRPC converts a grpc Cluster to a Swarm.
func SwarmFromGRPC(c swarmapi.Cluster) types.Swarm {
	swarm := types.Swarm{
		ClusterInfo: types.ClusterInfo{
			ID: c.ID,
			Spec: types.Spec{
				Orchestration: types.OrchestrationConfig{
					TaskHistoryRetentionLimit: c.Spec.Orchestration.TaskHistoryRetentionLimit,
				},
				Raft: types.RaftConfig{
					SnapshotInterval:           c.Spec.Raft.SnapshotInterval,
					KeepOldSnapshots:           c.Spec.Raft.KeepOldSnapshots,
					LogEntriesForSlowFollowers: c.Spec.Raft.LogEntriesForSlowFollowers,
					HeartbeatTick:              c.Spec.Raft.HeartbeatTick,
					ElectionTick:               c.Spec.Raft.ElectionTick,
				},
			},
		},
		JoinTokens: types.JoinTokens{
			Worker:  c.RootCA.JoinTokens.Worker,
			Manager: c.RootCA.JoinTokens.Manager,
		},
	}

	heartbeatPeriod, _ := ptypes.Duration(c.Spec.Dispatcher.HeartbeatPeriod)
	swarm.Spec.Dispatcher.HeartbeatPeriod = uint64(heartbeatPeriod)

	swarm.Spec.CAConfig.NodeCertExpiry, _ = ptypes.Duration(c.Spec.CAConfig.NodeCertExpiry)

	for _, ca := range c.Spec.CAConfig.ExternalCAs {
		swarm.Spec.CAConfig.ExternalCAs = append(swarm.Spec.CAConfig.ExternalCAs, &types.ExternalCA{
			Protocol: types.ExternalCAProtocol(strings.ToLower(ca.Protocol.String())),
			URL:      ca.URL,
			Options:  ca.Options,
		})
	}

	// Meta
	swarm.Version.Index = c.Meta.Version.Index
	swarm.CreatedAt, _ = ptypes.Timestamp(c.Meta.CreatedAt)
	swarm.UpdatedAt, _ = ptypes.Timestamp(c.Meta.UpdatedAt)

	// Annotations
	swarm.Spec.Name = c.Spec.Annotations.Name
	swarm.Spec.Labels = c.Spec.Annotations.Labels

	return swarm
}

// SwarmSpecToGRPC converts a Spec to a grpc ClusterSpec.
func SwarmSpecToGRPC(s types.Spec) (swarmapi.ClusterSpec, error) {
	spec := swarmapi.ClusterSpec{
		Annotations: swarmapi.Annotations{
			Name:   s.Name,
			Labels: s.Labels,
		},
		Orchestration: swarmapi.OrchestrationConfig{
			TaskHistoryRetentionLimit: s.Orchestration.TaskHistoryRetentionLimit,
		},
		Raft: swarmapi.RaftConfig{
			SnapshotInterval:           s.Raft.SnapshotInterval,
			KeepOldSnapshots:           s.Raft.KeepOldSnapshots,
			LogEntriesForSlowFollowers: s.Raft.LogEntriesForSlowFollowers,
			HeartbeatTick:              s.Raft.HeartbeatTick,
			ElectionTick:               s.Raft.ElectionTick,
		},
		Dispatcher: swarmapi.DispatcherConfig{
			HeartbeatPeriod: ptypes.DurationProto(time.Duration(s.Dispatcher.HeartbeatPeriod)),
		},
		CAConfig: swarmapi.CAConfig{
			NodeCertExpiry: ptypes.DurationProto(s.CAConfig.NodeCertExpiry),
		},
	}

	for _, ca := range s.CAConfig.ExternalCAs {
		protocol, ok := swarmapi.ExternalCA_CAProtocol_value[strings.ToUpper(string(ca.Protocol))]
		if !ok {
			return swarmapi.ClusterSpec{}, fmt.Errorf("invalid protocol: %q", ca.Protocol)
		}
		spec.CAConfig.ExternalCAs = append(spec.CAConfig.ExternalCAs, &swarmapi.ExternalCA{
			Protocol: swarmapi.ExternalCA_CAProtocol(protocol),
			URL:      ca.URL,
			Options:  ca.Options,
		})
	}

	return spec, nil
}
