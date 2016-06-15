package convert

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

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
		p := types.Policy{
			Role:       types.NodeRole(strings.ToLower(policy.Role.String())),
			Autoaccept: policy.Autoaccept,
		}
		if policy.Secret != nil {
			secret := string(policy.Secret.Data)
			p.Secret = &secret
		}
		swarm.Spec.AcceptancePolicy.Policies = append(swarm.Spec.AcceptancePolicy.Policies, p)
	}

	return swarm
}

// SwarmSpecToGRPCandMerge converts a Spec to a grpc ClusterSpec and merge AcceptancePolicy from an existing grpc ClusterSpec if provided.
func SwarmSpecToGRPCandMerge(s types.Spec, existingSpec *swarmapi.ClusterSpec) (swarmapi.ClusterSpec, error) {
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
			HeartbeatPeriod: s.Dispatcher.HeartbeatPeriod,
		},
		CAConfig: swarmapi.CAConfig{
			NodeCertExpiry: ptypes.DurationProto(s.CAConfig.NodeCertExpiry),
		},
	}

	if err := SwarmSpecUpdateAcceptancePolicy(&spec, s.AcceptancePolicy, existingSpec); err != nil {
		return swarmapi.ClusterSpec{}, err
	}

	return spec, nil
}

// SwarmSpecUpdateAcceptancePolicy updates a grpc ClusterSpec using AcceptancePolicy.
func SwarmSpecUpdateAcceptancePolicy(spec *swarmapi.ClusterSpec, acceptancePolicy types.AcceptancePolicy, oldSpec *swarmapi.ClusterSpec) error {
	spec.AcceptancePolicy.Policies = nil
	for _, p := range acceptancePolicy.Policies {
		role, ok := swarmapi.NodeRole_value[strings.ToUpper(string(p.Role))]
		if !ok {
			return fmt.Errorf("invalid Role: %q", p.Role)
		}

		policy := &swarmapi.AcceptancePolicy_RoleAdmissionPolicy{
			Role:       swarmapi.NodeRole(role),
			Autoaccept: p.Autoaccept,
		}

		if p.Secret != nil {
			if *p.Secret == "" { // if provided secret is empty, it means erase previous secret.
				policy.Secret = nil
			} else { // if provided secret is not empty, we generate a new one.
				hashPwd, _ := bcrypt.GenerateFromPassword([]byte(*p.Secret), 0)
				policy.Secret = &swarmapi.AcceptancePolicy_RoleAdmissionPolicy_HashedSecret{
					Data: hashPwd,
					Alg:  "bcrypt",
				}
			}
		} else if oldSecret := getOldSecret(oldSpec, policy.Role); oldSecret != nil { // else use the old one.
			policy.Secret = &swarmapi.AcceptancePolicy_RoleAdmissionPolicy_HashedSecret{
				Data: oldSecret.Data,
				Alg:  oldSecret.Alg,
			}
		}

		spec.AcceptancePolicy.Policies = append(spec.AcceptancePolicy.Policies, policy)
	}
	return nil
}

func getOldSecret(oldSpec *swarmapi.ClusterSpec, role swarmapi.NodeRole) *swarmapi.AcceptancePolicy_RoleAdmissionPolicy_HashedSecret {
	if oldSpec == nil {
		return nil
	}
	for _, p := range oldSpec.AcceptancePolicy.Policies {
		if p.Role == role {
			return p.Secret
		}
	}
	return nil
}
