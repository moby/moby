package convert

import (
	swarmtypes "github.com/docker/docker/api/types/swarm"
	swarmapi "github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/protobuf/ptypes"
)

// SecretFromGRPC converts a grpc Secret to a Secret.
func SecretFromGRPC(s *swarmapi.Secret) swarmtypes.Secret {
	secret := swarmtypes.Secret{
		ID:         s.ID,
		Digest:     s.Digest,
		SecretSize: s.SecretSize,
		Spec: swarmtypes.SecretSpec{
			Annotations: swarmtypes.Annotations{
				Name:   s.Spec.Annotations.Name,
				Labels: s.Spec.Annotations.Labels,
			},
			Data: s.Spec.Data,
		},
	}

	secret.Version.Index = s.Meta.Version.Index
	// Meta
	secret.CreatedAt, _ = ptypes.Timestamp(s.Meta.CreatedAt)
	secret.UpdatedAt, _ = ptypes.Timestamp(s.Meta.UpdatedAt)

	return secret
}

// SecretSpecToGRPC converts Secret to a grpc Secret.
func SecretSpecToGRPC(s swarmtypes.SecretSpec) swarmapi.SecretSpec {
	return swarmapi.SecretSpec{
		Annotations: swarmapi.Annotations{
			Name:   s.Name,
			Labels: s.Labels,
		},
		Data: s.Data,
	}
}
