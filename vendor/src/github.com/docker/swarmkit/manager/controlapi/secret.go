package controlapi

import (
	"regexp"

	"github.com/docker/distribution/digest"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/identity"
	"github.com/docker/swarmkit/manager/state/store"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// Currently this is contains the unimplemented secret functions in order to satisfy the interface

// MaxSecretSize is the maximum byte length of the `Secret.Spec.Data` field.
const MaxSecretSize = 500 * 1024 // 500KB

var validSecretNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9]+(?:[a-zA-Z0-9-_.]*[a-zA-Z0-9])?$`)

// assumes spec is not nil
func secretFromSecretSpec(spec *api.SecretSpec) *api.Secret {
	return &api.Secret{
		ID:         identity.NewID(),
		Spec:       *spec,
		SecretSize: int64(len(spec.Data)),
		Digest:     digest.FromBytes(spec.Data).String(),
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

	return &api.GetSecretResponse{Secret: secret}, nil
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

// CreateSecret creates and return a `CreateSecretResponse` with a `Secret` based
// on the provided `CreateSecretRequest.SecretSpec`.
// - Returns `InvalidArgument` if the `CreateSecretRequest.SecretSpec` is malformed,
//   or if the secret data is too long or contains invalid characters.
// - Returns an error if the creation fails.
func (s *Server) CreateSecret(ctx context.Context, request *api.CreateSecretRequest) (*api.CreateSecretResponse, error) {
	if err := validateSecretSpec(request.Spec); err != nil {
		return nil, err
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
		return &api.CreateSecretResponse{Secret: secret}, nil
	default:
		return nil, err
	}
}

// RemoveSecret removes the secret referenced by `RemoveSecretRequest.ID`.
// - Returns `InvalidArgument` if `RemoveSecretRequest.ID` is empty.
// - Returns `NotFound` if the a secret named `RemoveSecretRequest.ID` is not found.
// - Returns an error if the deletion fails.
func (s *Server) RemoveSecret(ctx context.Context, request *api.RemoveSecretRequest) (*api.RemoveSecretResponse, error) {
	if request.SecretID == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "secret ID must be provided")
	}

	err := s.store.Update(func(tx store.Tx) error {
		return store.DeleteSecret(tx, request.SecretID)
	})
	switch err {
	case store.ErrNotExist:
		return nil, grpc.Errorf(codes.NotFound, "secret %s not found", request.SecretID)
	case nil:
		return &api.RemoveSecretResponse{}, nil
	default:
		return nil, err
	}
}

func validateSecretSpec(spec *api.SecretSpec) error {
	if spec == nil {
		return grpc.Errorf(codes.InvalidArgument, errInvalidArgument.Error())
	}
	if err := validateSecretAnnotations(spec.Annotations); err != nil {
		return err
	}

	if len(spec.Data) >= MaxSecretSize || len(spec.Data) < 1 {
		return grpc.Errorf(codes.InvalidArgument, "secret data must be larger than 0 and less than %d bytes", MaxSecretSize)
	}
	return nil
}

func validateSecretAnnotations(m api.Annotations) error {
	if m.Name == "" {
		return grpc.Errorf(codes.InvalidArgument, "name must be provided")
	} else if len(m.Name) > 64 || !validSecretNameRegexp.MatchString(m.Name) {
		// if the name doesn't match the regex
		return grpc.Errorf(codes.InvalidArgument,
			"invalid name, only 64 [a-zA-Z0-9-_.] characters allowed, and the start and end character must be [a-zA-Z0-9]")
	}
	return nil
}
