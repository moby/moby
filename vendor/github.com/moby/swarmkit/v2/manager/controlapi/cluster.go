package controlapi

import (
	"context"
	"strings"
	"time"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/ca"
	"github.com/moby/swarmkit/v2/log"
	"github.com/moby/swarmkit/v2/manager/encryption"
	"github.com/moby/swarmkit/v2/manager/state/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// expiredCertGrace is the amount of time to keep a node in the
	// blacklist beyond its certificate expiration timestamp.
	expiredCertGrace = 24 * time.Hour * 7
	// inbuilt default subnet size
	inbuiltSubnetSize = 24
	// VXLAN default port
	defaultVXLANPort = 4789
)

var (
	// inbuilt default address pool
	inbuiltDefaultAddressPool = []string{"10.0.0.0/8"}
)

func validateClusterSpec(spec *api.ClusterSpec) error {
	if spec == nil {
		return status.Error(codes.InvalidArgument, errInvalidArgument.Error())
	}

	// Validate that expiry time being provided is valid, and over our minimum
	if expiry := spec.GetCaConfig().GetNodeCertExpiry(); expiry != nil {
		if err := expiry.CheckValid(); err != nil {
			return status.Error(codes.InvalidArgument, errInvalidArgument.Error())
		}
		if expiry.AsDuration() < ca.MinNodeCertExpiration {
			return status.Errorf(codes.InvalidArgument, "minimum certificate expiry time is: %s", ca.MinNodeCertExpiration)
		}
	}

	// Validate that AcceptancePolicies only include Secrets that are bcrypted
	// TODO(diogo): Add a global list of acceptance algorithms. We only support bcrypt for now.
	if len(spec.GetAcceptancePolicy().GetPolicies()) > 0 {
		for _, policy := range spec.GetAcceptancePolicy().GetPolicies() {
			if policy.Secret != nil && strings.ToLower(policy.Secret.Alg) != "bcrypt" {
				return status.Errorf(codes.InvalidArgument, "hashing algorithm is not supported: %s", policy.Secret.Alg)
			}
		}
	}

	// Validate that heartbeatPeriod time being provided is valid
	if hb := spec.GetDispatcher().GetHeartbeatPeriod(); hb != nil {
		if err := hb.CheckValid(); err != nil {
			return status.Error(codes.InvalidArgument, errInvalidArgument.Error())
		}
		if hb.AsDuration() < 0 {
			return status.Errorf(codes.InvalidArgument, "heartbeat time period cannot be a negative duration")
		}
	}

	if spec.GetAnnotations().GetName() != store.DefaultClusterName {
		return status.Errorf(codes.InvalidArgument, "modification of cluster name is not allowed")
	}

	return nil
}

// GetCluster returns a Cluster given a ClusterID.
// - Returns `InvalidArgument` if ClusterID is not provided.
// - Returns `NotFound` if the Cluster is not found.
func (s *Server) GetCluster(_ context.Context, request *api.GetClusterRequest) (*api.GetClusterResponse, error) {
	if request.ClusterId == "" {
		return nil, status.Error(codes.InvalidArgument, errInvalidArgument.Error())
	}

	var cluster *api.Cluster
	s.store.View(func(tx store.ReadTx) {
		cluster = store.GetCluster(tx, request.ClusterId)
	})
	if cluster == nil {
		return nil, status.Errorf(codes.NotFound, "cluster %s not found", request.ClusterId)
	}

	redactedClusters := redactClusters([]*api.Cluster{cluster})

	// WARN: we should never return cluster here. We need to redact the private fields first.
	return &api.GetClusterResponse{
		Cluster: redactedClusters[0],
	}, nil
}

// UpdateCluster updates a Cluster referenced by ClusterID with the given ClusterSpec.
// - Returns `NotFound` if the Cluster is not found.
// - Returns `InvalidArgument` if the ClusterSpec is malformed.
// - Returns `Unimplemented` if the ClusterSpec references unimplemented features.
// - Returns an error if the update fails.
func (s *Server) UpdateCluster(ctx context.Context, request *api.UpdateClusterRequest) (*api.UpdateClusterResponse, error) {
	if request.ClusterId == "" || request.ClusterVersion == nil {
		return nil, status.Error(codes.InvalidArgument, errInvalidArgument.Error())
	}
	if err := validateClusterSpec(request.Spec); err != nil {
		return nil, err
	}

	var cluster *api.Cluster
	err := s.store.Update(func(tx store.Tx) error {
		cluster = store.GetCluster(tx, request.ClusterId)
		if cluster == nil {
			return status.Errorf(codes.NotFound, "cluster %s not found", request.ClusterId)
		}
		// This ensures that we have the current rootCA with which to generate tokens (expiration doesn't matter
		// for generating the tokens)
		rootCA, err := ca.RootCAFromAPI(cluster.RootCa, ca.DefaultNodeCertExpiration)
		if err != nil {
			log.G(ctx).WithField(
				"method", "(*controlapi.Server).UpdateCluster").WithError(err).Error("invalid cluster root CA")
			return status.Errorf(codes.Internal, "error loading cluster rootCA for update")
		}

		cluster.Meta.Version = request.ClusterVersion
		cluster.Spec = request.Spec.Copy()

		expireBlacklistedCerts(cluster)

		if request.GetRotation().GetWorkerJoinToken() {
			cluster.RootCa.GetJoinTokens().Worker = ca.GenerateJoinToken(&rootCA, cluster.Fips)
		}
		if request.GetRotation().GetManagerJoinToken() {
			cluster.RootCa.GetJoinTokens().Manager = ca.GenerateJoinToken(&rootCA, cluster.Fips)
		}

		updatedRootCA, err := validateCAConfig(ctx, s.securityConfig, cluster)
		if err != nil {
			return err
		}
		cluster.RootCa = updatedRootCA

		var unlockKeys []*api.EncryptionKey
		var managerKey *api.EncryptionKey
		for _, eKey := range cluster.UnlockKeys {
			if eKey.Subsystem == ca.ManagerRole {
				if !cluster.GetSpec().GetEncryptionConfig().GetAutoLockManagers() {
					continue
				}
				managerKey = eKey
			}
			unlockKeys = append(unlockKeys, eKey)
		}

		switch {
		case !cluster.GetSpec().GetEncryptionConfig().GetAutoLockManagers():
			break
		case managerKey == nil:
			unlockKeys = append(unlockKeys, &api.EncryptionKey{
				Subsystem: ca.ManagerRole,
				Key:       encryption.GenerateSecretKey(),
			})
		case request.GetRotation().GetManagerUnlockKey():
			managerKey.Key = encryption.GenerateSecretKey()
		}
		cluster.UnlockKeys = unlockKeys

		return store.UpdateCluster(tx, cluster)
	})
	if err != nil {
		return nil, err
	}

	redactedClusters := redactClusters([]*api.Cluster{cluster})

	// WARN: we should never return cluster here. We need to redact the private fields first.
	return &api.UpdateClusterResponse{
		Cluster: redactedClusters[0],
	}, nil
}

func filterClusters(candidates []*api.Cluster, filters ...func(*api.Cluster) bool) []*api.Cluster {
	result := []*api.Cluster{}

	for _, c := range candidates {
		match := true
		for _, f := range filters {
			if !f(c) {
				match = false
				break
			}
		}
		if match {
			result = append(result, c)
		}
	}

	return result
}

// ListClusters returns a list of all clusters.
func (s *Server) ListClusters(_ context.Context, request *api.ListClustersRequest) (*api.ListClustersResponse, error) {
	var (
		clusters []*api.Cluster
		err      error
	)
	s.store.View(func(tx store.ReadTx) {
		switch {
		case request.Filters != nil && len(request.Filters.Names) > 0:
			clusters, err = store.FindClusters(tx, buildFilters(store.ByName, request.Filters.Names))
		case request.Filters != nil && len(request.Filters.NamePrefixes) > 0:
			clusters, err = store.FindClusters(tx, buildFilters(store.ByNamePrefix, request.Filters.NamePrefixes))
		case request.Filters != nil && len(request.Filters.IdPrefixes) > 0:
			clusters, err = store.FindClusters(tx, buildFilters(store.ByIDPrefix, request.Filters.IdPrefixes))
		default:
			clusters, err = store.FindClusters(tx, store.All)
		}
	})
	if err != nil {
		return nil, err
	}

	if request.Filters != nil {
		clusters = filterClusters(clusters,
			func(e *api.Cluster) bool {
				return filterContains(e.GetSpec().GetAnnotations().GetName(), request.Filters.Names)
			},
			func(e *api.Cluster) bool {
				return filterContainsPrefix(e.GetSpec().GetAnnotations().GetName(), request.Filters.NamePrefixes)
			},
			func(e *api.Cluster) bool {
				return filterContainsPrefix(e.Id, request.Filters.IdPrefixes)
			},
			func(e *api.Cluster) bool {
				return filterMatchLabels(e.GetSpec().GetAnnotations().GetLabels(), request.Filters.Labels)
			},
		)
	}

	// WARN: we should never return cluster here. We need to redact the private fields first.
	return &api.ListClustersResponse{
		Clusters: redactClusters(clusters),
	}, nil
}

// redactClusters is a method that enforces a whitelist of fields that are ok to be
// returned in the Cluster object. It should filter out all sensitive information.
func redactClusters(clusters []*api.Cluster) []*api.Cluster {
	var redactedClusters []*api.Cluster
	// Only add public fields to the new clusters
	for _, cluster := range clusters {
		// Copy all the mandatory fields
		// Do not copy secret keys
		redactedSpec := cluster.Spec.Copy()
		if redactedSpec == nil {
			redactedSpec = &api.ClusterSpec{}
		}
		if redactedSpec.CaConfig == nil {
			// CaConfig used to be a non-nullable embedded message.
			redactedSpec.CaConfig = &api.CAConfig{}
		}
		redactedSpec.CaConfig.SigningCaKey = nil
		// the cert is not a secret, but if API users get the cluster spec and then update,
		// then because the cert is included but not the key, the user can get update errors
		// or unintended consequences (such as telling swarm to forget about the key so long
		// as there is a corresponding external CA)
		redactedSpec.CaConfig.SigningCaCert = nil

		redactedRootCA := cluster.RootCa.Copy()
		if redactedRootCA == nil {
			redactedRootCA = &api.RootCA{}
		}
		redactedRootCA.CaKey = nil
		if r := redactedRootCA.RootRotation; r != nil {
			r.CaKey = nil
		}
		newCluster := &api.Cluster{
			Id:                      cluster.Id,
			Meta:                    cluster.Meta,
			Spec:                    redactedSpec,
			RootCa:                  redactedRootCA,
			BlacklistedCertificates: cluster.BlacklistedCertificates,
			DefaultAddressPool:      cluster.DefaultAddressPool,
			SubnetSize:              cluster.SubnetSize,
			VXLANUDPPort:            cluster.VXLANUDPPort,
		}
		if newCluster.DefaultAddressPool == nil {
			// This is just for CLI display. Set the inbuilt default pool for
			// user reference.
			newCluster.DefaultAddressPool = inbuiltDefaultAddressPool
			newCluster.SubnetSize = inbuiltSubnetSize
		}
		if newCluster.VXLANUDPPort == 0 {
			newCluster.VXLANUDPPort = defaultVXLANPort
		}
		redactedClusters = append(redactedClusters, newCluster)
	}

	return redactedClusters
}

func expireBlacklistedCerts(cluster *api.Cluster) {
	nowMinusGrace := time.Now().Add(-expiredCertGrace)

	for cn, blacklistedCert := range cluster.BlacklistedCertificates {
		if blacklistedCert.Expiry == nil {
			continue
		}

		// AsTime cannot fail, so test validity explicitly to keep skipping
		// certificates whose expiry we cannot make sense of.
		if ts := blacklistedCert.Expiry; ts.IsValid() && nowMinusGrace.After(ts.AsTime()) {
			delete(cluster.BlacklistedCertificates, cn)
		}
	}
}
