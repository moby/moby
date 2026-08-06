package convert

import (
	"os"

	swarmtypes "github.com/moby/moby/api/types/swarm"
	swarmapi "github.com/moby/swarmkit/v2/api"
)

// SecretFromGRPC converts a grpc Secret to a Secret.
func SecretFromGRPC(s *swarmapi.Secret) swarmtypes.Secret {
	secret := swarmtypes.Secret{
		ID: s.Id,
		Spec: swarmtypes.SecretSpec{
			Annotations: annotationsFromGRPC(s.Spec.Annotations),
			Data:        s.Spec.Data,
			Driver:      driverFromGRPC(s.Spec.Driver),
		},
	}

	secret.Version.Index = s.Meta.Version.Index
	// Meta
	secret.CreatedAt = s.Meta.CreatedAt.AsTime()
	secret.UpdatedAt = s.Meta.UpdatedAt.AsTime()

	if s.Spec.Templating != nil {
		secret.Spec.Templating = &swarmtypes.Driver{
			Name:    s.Spec.Templating.Name,
			Options: s.Spec.Templating.Options,
		}
	}

	return secret
}

// SecretSpecToGRPC converts Secret to a grpc Secret.
func SecretSpecToGRPC(s swarmtypes.SecretSpec) *swarmapi.SecretSpec {
	spec := &swarmapi.SecretSpec{
		Annotations: &swarmapi.Annotations{
			Name:   s.Name,
			Labels: s.Labels,
		},
		Data:   s.Data,
		Driver: driverToGRPC(s.Driver),
	}

	if s.Templating != nil {
		spec.Templating = &swarmapi.Driver{
			Name:    s.Templating.Name,
			Options: s.Templating.Options,
		}
	}

	return spec
}

// SecretReferencesFromGRPC converts a slice of grpc SecretReference to SecretReference
func SecretReferencesFromGRPC(s []*swarmapi.SecretReference) []*swarmtypes.SecretReference {
	refs := []*swarmtypes.SecretReference{}

	for _, r := range s {
		ref := &swarmtypes.SecretReference{
			SecretID:   r.SecretId,
			SecretName: r.SecretName,
		}

		if t, ok := r.Target.(*swarmapi.SecretReference_File); ok {
			ref.File = &swarmtypes.SecretReferenceFileTarget{
				Name: t.File.Name,
				UID:  t.File.Uid,
				GID:  t.File.Gid,
				Mode: os.FileMode(t.File.Mode),
			}
		}

		refs = append(refs, ref)
	}

	return refs
}
