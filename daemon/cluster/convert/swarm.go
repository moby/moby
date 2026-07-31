package convert

import (
	"fmt"
	"net/netip"
	"strings"

	types "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/v2/internal/sliceutil"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/ca"
	"google.golang.org/protobuf/types/known/durationpb"
)

// SwarmFromGRPC converts a grpc Cluster to a Swarm.
func SwarmFromGRPC(c *swarmapi.Cluster) types.Swarm {
	swarm := types.Swarm{
		ClusterInfo: types.ClusterInfo{
			ID: c.Id,
			Spec: types.Spec{
				Orchestration: types.OrchestrationConfig{
					TaskHistoryRetentionLimit: &c.Spec.Orchestration.TaskHistoryRetentionLimit,
				},
				Raft: types.RaftConfig{
					SnapshotInterval:           c.Spec.Raft.SnapshotInterval,
					KeepOldSnapshots:           &c.Spec.Raft.KeepOldSnapshots,
					LogEntriesForSlowFollowers: c.Spec.Raft.LogEntriesForSlowFollowers,
					HeartbeatTick:              int(c.Spec.Raft.HeartbeatTick),
					ElectionTick:               int(c.Spec.Raft.ElectionTick),
				},
				EncryptionConfig: types.EncryptionConfig{
					AutoLockManagers: c.Spec.EncryptionConfig.AutoLockManagers,
				},
				CAConfig: types.CAConfig{
					// do not include the signing CA cert or key (it should already be redacted via the swarm APIs) -
					// the key because it's secret, and the cert because otherwise doing a get + update on the spec
					// can cause issues because the key would be missing and the cert wouldn't
					ForceRotate: c.Spec.CaConfig.ForceRotate,
				},
			},
			TLSInfo: types.TLSInfo{
				TrustRoot: string(c.RootCa.CaCert),
			},
			RootRotationInProgress: c.RootCa.RootRotation != nil,
			DefaultAddrPool:        sliceutil.Map(c.DefaultAddressPool, func(s string) netip.Prefix { pfx, _ := netip.ParsePrefix(s); return pfx }),
			SubnetSize:             c.SubnetSize,
			DataPathPort:           c.VXLANUDPPort,
		},
		JoinTokens: types.JoinTokens{
			Worker:  c.RootCa.JoinTokens.Worker,
			Manager: c.RootCa.JoinTokens.Manager,
		},
	}

	issuerInfo, err := ca.IssuerFromAPIRootCA(c.RootCa)
	if err == nil && issuerInfo != nil {
		swarm.TLSInfo.CertIssuerSubject = issuerInfo.Subject
		swarm.TLSInfo.CertIssuerPublicKey = issuerInfo.PublicKey
	}

	heartbeatPeriod := c.Spec.Dispatcher.HeartbeatPeriod.AsDuration()
	swarm.Spec.Dispatcher.HeartbeatPeriod = heartbeatPeriod

	swarm.Spec.CAConfig.NodeCertExpiry = c.Spec.CaConfig.NodeCertExpiry.AsDuration()

	for _, ca := range c.Spec.CaConfig.ExternalCas {
		swarm.Spec.CAConfig.ExternalCAs = append(swarm.Spec.CAConfig.ExternalCAs, &types.ExternalCA{
			Protocol: types.ExternalCAProtocol(strings.ToLower(ca.Protocol.String())),
			URL:      ca.Url,
			Options:  ca.Options,
			CACert:   string(ca.CaCert),
		})
	}

	// Meta
	swarm.Version.Index = c.Meta.Version.Index
	swarm.CreatedAt = c.Meta.CreatedAt.AsTime()
	swarm.UpdatedAt = c.Meta.UpdatedAt.AsTime()

	// Annotations
	swarm.Spec.Annotations = annotationsFromGRPC(c.Spec.Annotations)

	return swarm
}

// SwarmSpecToGRPC converts a Spec to a grpc ClusterSpec.
func SwarmSpecToGRPC(s types.Spec) (*swarmapi.ClusterSpec, error) {
	return MergeSwarmSpecToGRPC(s, &swarmapi.ClusterSpec{})
}

// MergeSwarmSpecToGRPC merges a Spec with an initial grpc ClusterSpec
func MergeSwarmSpecToGRPC(s types.Spec, spec *swarmapi.ClusterSpec) (*swarmapi.ClusterSpec, error) {
	// We take the initSpec (either created from scratch, or returned by swarmkit),
	// and will only change the value if the one taken from types.Spec is not nil or 0.
	// In other words, if the value taken from types.Spec is nil or 0, we will maintain the status quo.
	if s.Annotations.Name != "" {
		spec.Annotations.Name = s.Annotations.Name
	}
	if len(s.Annotations.Labels) != 0 {
		spec.Annotations.Labels = s.Annotations.Labels
	}

	if s.Orchestration.TaskHistoryRetentionLimit != nil {
		spec.Orchestration.TaskHistoryRetentionLimit = *s.Orchestration.TaskHistoryRetentionLimit
	}
	if s.Raft.SnapshotInterval != 0 {
		spec.Raft.SnapshotInterval = s.Raft.SnapshotInterval
	}
	if s.Raft.KeepOldSnapshots != nil {
		spec.Raft.KeepOldSnapshots = *s.Raft.KeepOldSnapshots
	}
	if s.Raft.LogEntriesForSlowFollowers != 0 {
		spec.Raft.LogEntriesForSlowFollowers = s.Raft.LogEntriesForSlowFollowers
	}
	if s.Raft.HeartbeatTick != 0 {
		spec.Raft.HeartbeatTick = uint32(s.Raft.HeartbeatTick)
	}
	if s.Raft.ElectionTick != 0 {
		spec.Raft.ElectionTick = uint32(s.Raft.ElectionTick)
	}
	if s.Dispatcher.HeartbeatPeriod != 0 {
		spec.Dispatcher.HeartbeatPeriod = durationpb.New(s.Dispatcher.HeartbeatPeriod)
	}
	if s.CAConfig.NodeCertExpiry != 0 {
		spec.CaConfig.NodeCertExpiry = durationpb.New(s.CAConfig.NodeCertExpiry)
	}
	if s.CAConfig.SigningCACert != "" {
		spec.CaConfig.SigningCaCert = []byte(s.CAConfig.SigningCACert)
	}
	if s.CAConfig.SigningCAKey != "" {
		// do propagate the signing CA key here because we want to provide it TO the swarm APIs
		spec.CaConfig.SigningCaKey = []byte(s.CAConfig.SigningCAKey)
	}
	spec.CaConfig.ForceRotate = s.CAConfig.ForceRotate

	for _, ca := range s.CAConfig.ExternalCAs {
		protocol, ok := swarmapi.ExternalCA_CAProtocol_value[strings.ToUpper(string(ca.Protocol))]
		if !ok {
			return nil, fmt.Errorf("invalid protocol: %q", ca.Protocol)
		}
		spec.CaConfig.ExternalCas = append(spec.CaConfig.ExternalCas, &swarmapi.ExternalCA{
			Protocol: swarmapi.ExternalCA_CAProtocol(protocol),
			Url:      ca.URL,
			Options:  ca.Options,
			CaCert:   []byte(ca.CACert),
		})
	}

	spec.EncryptionConfig.AutoLockManagers = s.EncryptionConfig.AutoLockManagers

	return spec, nil
}
