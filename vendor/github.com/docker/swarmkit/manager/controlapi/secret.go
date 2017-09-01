package controlapi

import (
	"crypto/subtle"
	"strings"

	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/api/validation"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/log"
	"github.com/docker/swarmkit/manager/state/store"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// assumes spec is not nil
func secretFromSecretSpec(spec *api.SecretSpec) *api.Secret {
	return &api.Secret{
		ID:   identity.NewID(),
		Spec: *spec,
	}
}

// GetSecret returns a `GetSecretResponse` with a `Secret` with the same
// id as `GetSecretRequest.SecretID`
// - Returns `NotFound` if the Secret with the given id is not found.
// - Returns `InvalidArgument` if the `GetSecretRequest.SecretID` is empty.
// - Returns an error if getting fails.
func (s *Server) GetSecret(ctx context.Context, request *api.GetSecretRequest) (*api.GetSecretResponse, error) {
	if request.SecretID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "secret ID must be provided")
	}

	var secret *api.Secret
	s.store.View(func(tx store.ReadTx) {
		secret = store.GetSecret(tx, request.SecretID)
	})

	if secret == nil {
		return nil, grpc.Errorf(codes.NotFound, "secret %s not found", request.SecretID)
	}

	secret.Spec.Data = nil // clean the actual secret data so it's never returned
	return &api.GetSecretResponse{Secret: secret}, nil
}

// UpdateSecret updates a Secret referenced by SecretID with the given SecretSpec.
// - Returns `NotFound` if the Secret is not found.
// - Returns `InvalidArgument` if the SecretSpec is malformed or anything other than Labels is changed
// - Returns an error if the update fails.
func (s *Server) UpdateSecret(ctx context.Context, request *api.UpdateSecretRequest) (*api.UpdateSecretResponse, error) {
	if request.SecretID == "" || request.SecretVersion == nil {
		return nil, grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}
	var secret *api.Secret
	err := s.store.Update(func(tx store.Tx) error {
		secret = store.GetSecret(tx, request.SecretID)
		if secret == nil {
			return grpc.Errorf(codes.NotFound, "secret %s not found", request.SecretID)
		}

		// Check if the Name is different than the current name, or the secret is non-nil and different
		// than the current secret
		if secret.Spec.Annotations.Name != request.Spec.Annotations.Name ||
			(request.Spec.Data != nil && subtle.ConstantTimeCompare(request.Spec.Data, secret.Spec.Data) == 0) {
			return grpc.Errorf(codes.InvalidArgument, "only updates to Labels are allowed")
		}

		// We only allow updating Labels
		secret.Meta.Version = *request.SecretVersion
		secret.Spec.Annotations.Labels = request.Spec.Annotations.Labels

		return store.UpdateSecret(tx, secret)
	})
	if err != nil {
		return nil, err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"secret.ID":   request.SecretID,
		"secret.Name": request.Spec.Annotations.Name,
		"method":      "UpdateSecret",
	}).Debugf("secret updated")

	// WARN: we should never return the actual secret data here. We need to redact the private fields first.
	secret.Spec.Data = nil
	return &api.UpdateSecretResponse{
		Secret: secret,
	}, nil
}

// ListSecrets returns a `ListSecretResponse` with a list all non-internal `Secret`s being
// managed, or all secrets matching any name in `ListSecretsRequest.Names`, any
// name prefix in `ListSecretsRequest.NamePrefixes`, any id in
// `ListSecretsRequest.SecretIDs`, or any id prefix in `ListSecretsRequest.IDPrefixes`.
// - Returns an error if listing fails.
func (s *Server) ListSecrets(ctx context.Context, request *api.ListSecretsRequest) (*api.ListSecretsResponse, error) {
	var (
		secrets     []*api.Secret
		respSecrets []*api.Secret
		err         error
		byFilters   []store.By
		by          store.By
		labels      map[string]string
	)

	// return all secrets that match either any of the names or any of the name prefixes (why would you give both?)
	if request.Filters != nil {
		for _, name := range request.Filters.Names {
			byFilters = append(byFilters, store.ByName(name))
		}
		for _, prefix := range request.Filters.NamePrefixes {
			byFilters = append(byFilters, store.ByNamePrefix(prefix))
		}
		for _, prefix := range request.Filters.IDPrefixes {
			byFilters = append(byFilters, store.ByIDPrefix(prefix))
		}
		labels = request.Filters.Labels
	}

	switch len(byFilters) {
	case 0:
		by = store.All
	case 1:
		by = byFilters[0]
	default:
		by = store.Or(byFilters...)
	}

	s.store.View(func(tx store.ReadTx) {
		secrets, err = store.FindSecrets(tx, by)
	})
	if err != nil {
		return nil, err
	}

	// strip secret data from the secret, filter by label, and filter out all internal secrets
	for _, secret := range secrets {
		if secret.Internal || !filterMatchLabels(secret.Spec.Annotations.Labels, labels) {
			continue
		}
		secret.Spec.Data = nil // clean the actual secret data so it's never returned
		respSecrets = append(respSecrets, secret)
	}

	return &api.ListSecretsResponse{Secrets: respSecrets}, nil
}

// CreateSecret creates and returns a `CreateSecretResponse` with a `Secret` based
// on the provided `CreateSecretRequest.SecretSpec`.
// - Returns `InvalidArgument` if the `CreateSecretRequest.SecretSpec` is malformed,
//   or if the secret data is too long or contains invalid characters.
// - Returns an error if the creation fails.
func (s *Server) CreateSecret(ctx context.Context, request *api.CreateSecretRequest) (*api.CreateSecretResponse, error) {
	if err := validateSecretSpec(request.Spec); err != nil {
		return nil, err
	}

	if request.Spec.Driver != nil { // Check that the requested driver is valid
		if _, err := s.dr.NewSecretDriver(request.Spec.Driver); err != nil {
			return nil, err
		}
	}

	secret := secretFromSecretSpec(request.Spec) // the store will handle name conflicts
	err := s.store.Update(func(tx store.Tx) error {
		return store.CreateSecret(tx, secret)
	})

	switch err {
	case store.ErrNameConflict:
		return nil, grpc.Errorf(codes.AlreadyExists, "secret %s already exists", request.Spec.Annotations.Name)
	case nil:
		secret.Spec.Data = nil // clean the actual secret data so it's never returned
		log.G(ctx).WithFields(logrus.Fields{
			"secret.Name": request.Spec.Annotations.Name,
			"method":      "CreateSecret",
		}).Debugf("secret created")

		return &api.CreateSecretResponse{Secret: secret}, nil
	default:
		return nil, err
	}
}

// RemoveSecret removes the secret referenced by `RemoveSecretRequest.ID`.
// - Returns `InvalidArgument` if `RemoveSecretRequest.ID` is empty.
// - Returns `NotFound` if the a secret named `RemoveSecretRequest.ID` is not found.
// - Returns `SecretInUse` if the secret is currently in use
// - Returns an error if the deletion fails.
func (s *Server) RemoveSecret(ctx context.Context, request *api.RemoveSecretRequest) (*api.RemoveSecretResponse, error) {
	if request.SecretID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "secret ID must be provided")
	}

	err := s.store.Update(func(tx store.Tx) error {
		// Check if the secret exists
		secret := store.GetSecret(tx, request.SecretID)
		if secret == nil {
			return grpc.Errorf(codes.NotFound, "could not find secret %s", request.SecretID)
		}

		// Check if any services currently reference this secret, return error if so
		services, err := store.FindServices(tx, store.ByReferencedSecretID(request.SecretID))
		if err != nil {
			return grpc.Errorf(codes.Internal, "could not find services using secret %s: %v", request.SecretID, err)
		}

		if len(services) != 0 {
			serviceNames := make([]string, 0, len(services))
			for _, service := range services {
				serviceNames = append(serviceNames, service.Spec.Annotations.Name)
			}

			secretName := secret.Spec.Annotations.Name
			serviceNameStr := strings.Join(serviceNames, ", ")
			serviceStr := "services"
			if len(serviceNames) == 1 {
				serviceStr = "service"
			}

			return grpc.Errorf(codes.InvalidArgument, "secret '%s' is in use by the following %s: %v", secretName, serviceStr, serviceNameStr)
		}

		return store.DeleteSecret(tx, request.SecretID)
	})
	switch err {
	case store.ErrNotExist:
		return nil, grpc.Errorf(codes.NotFound, "secret %s not found", request.SecretID)
	case nil:
		log.G(ctx).WithFields(logrus.Fields{
			"secret.ID": request.SecretID,
			"method":    "RemoveSecret",
		}).Debugf("secret removed")

		return &api.RemoveSecretResponse{}, nil
	default:
		return nil, err
	}
}

func validateSecretSpec(spec *api.SecretSpec) error {
	if spec == nil {
		return grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}
	if err := validateConfigOrSecretAnnotations(spec.Annotations); err != nil {
		return err
	}
	// Check if secret driver is defined
	if spec.Driver != nil {
		// Ensure secret driver has a name
		if spec.Driver.Name == "" {
			return grpc.Errorf(codes.InvalidArgument, "secret driver must have a name")
		}
		return nil
	}
	if err := validation.ValidateSecretPayload(spec.Data); err != nil {
		return grpc.Errorf(codes.InvalidArgument, "%s", err.Error())
	}
	return nil
}
