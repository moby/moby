package convert

import (
	"fmt"
	"strings"

	types "github.com/docker/engine-api/types/swarm"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/protobuf/ptypes"
)

// SwarmFromGRPC converts a grpc Cluster to a Swarm.
func SwarmFromGRPC(c swarmapi.Cluster) types.Swarm {
	swarm := types.Swarm{
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
			Dispatcher: types.DispatcherConfig{
				HeartbeatPeriod: c.Spec.Dispatcher.HeartbeatPeriod,
			},
		},
	}

	swarm.Spec.CAConfig.NodeCertExpiry, _ = ptypes.Duration(c.Spec.CAConfig.NodeCertExpiry)

	// Meta
	swarm.Version.Index = c.Meta.Version.Index
	swarm.CreatedAt, _ = ptypes.Timestamp(c.Meta.CreatedAt)
	swarm.UpdatedAt, _ = ptypes.Timestamp(c.Meta.UpdatedAt)

	// Annotations
	swarm.Spec.Name = c.Spec.Annotations.Name
	swarm.Spec.Labels = c.Spec.Annotations.Labels

	for _, policy := range c.Spec.AcceptancePolicy.Policies {
		swarm.Spec.AcceptancePolicy.Policies = append(swarm.Spec.AcceptancePolicy.Policies, types.Policy{
			Role:       policy.Role.String(),
			Autoaccept: policy.Autoaccept,
			Secret:     policy.Secret,
		})
	}

	return swarm
}

// SwarmSpecToGRPC converts a Swarm to a grpc ClusterSpec.
func SwarmSpecToGRPC(s types.Swarm) (swarmapi.ClusterSpec, error) {
	spec := swarmapi.ClusterSpec{
		Annotations: swarmapi.Annotations{
			Name:   s.Spec.Name,
			Labels: s.Spec.Labels,
		},
		Orchestration: swarmapi.OrchestrationConfig{
			TaskHistoryRetentionLimit: s.Spec.Orchestration.TaskHistoryRetentionLimit,
		},
		Raft: swarmapi.RaftConfig{
			SnapshotInterval:           s.Spec.Raft.SnapshotInterval,
			KeepOldSnapshots:           s.Spec.Raft.KeepOldSnapshots,
			LogEntriesForSlowFollowers: s.Spec.Raft.LogEntriesForSlowFollowers,
			HeartbeatTick:              s.Spec.Raft.HeartbeatTick,
			ElectionTick:               s.Spec.Raft.ElectionTick,
		},
		Dispatcher: swarmapi.DispatcherConfig{
			HeartbeatPeriod: s.Spec.Dispatcher.HeartbeatPeriod,
		},
		CAConfig: swarmapi.CAConfig{
			NodeCertExpiry: ptypes.DurationProto(s.Spec.CAConfig.NodeCertExpiry),
		},
	}

	if err := SwarmSpecUpdateAcceptancePolicy(&spec, s.Spec.AcceptancePolicy); err != nil {
		return swarmapi.ClusterSpec{}, err
	}
	return spec, nil
}

// SwarmSpecUpdateAcceptancePolicy updates a grpc ClusterSpec using AcceptancePolicy.
func SwarmSpecUpdateAcceptancePolicy(spec *swarmapi.ClusterSpec, acceptancePolicy types.AcceptancePolicy) error {
	spec.AcceptancePolicy.Policies = nil
	for _, p := range acceptancePolicy.Policies {
		role, ok := swarmapi.NodeRole_value[strings.ToUpper(p.Role)]
		if !ok {
			return fmt.Errorf("invalid Role: %q", p.Role)
		}
		policy := &swarmapi.AcceptancePolicy_RoleAdmissionPolicy{
			Role:       swarmapi.NodeRole(role),
			Autoaccept: p.Autoaccept,
			Secret:     p.Secret,
		}
		spec.AcceptancePolicy.Policies = append(spec.AcceptancePolicy.Policies, policy)
	}
	return nil
}
