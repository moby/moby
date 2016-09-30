package controlapi

import (
	"github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// Currently this is contains the unimplemented secret functions in order to satisfy the interface

// MaxSecretSize is the maximum byte length of the `Secret.Spec.Data` field.
const MaxSecretSize = 500 * 1024 // 500KB

// GetSecret returns a `GetSecretResponse` with a `Secret` with the same
// id as `GetSecretRequest.SecretID`
// - Returns `NotFound` if the Secret with the given id is not found.
// - Returns `InvalidArgument` if the `GetSecretRequest.SecretID` is empty.
// - Returns an error if getting fails.
func (s *Server) GetSecret(ctx context.Context, request *api.GetSecretRequest) (*api.GetSecretResponse, error) {
	return nil, grpc.Errorf(codes.Unimplemented, "Not yet implemented")
}

// ListSecrets returns a `ListSecretResponse` with a list all non-internal `Secret`s being
// managed, or all secrets matching any name in `ListSecretsRequest.Names`, any
// name prefix in `ListSecretsRequest.NamePrefixes`, any id in
// `ListSecretsRequest.SecretIDs`, or any id prefix in `ListSecretsRequest.IDPrefixes`.
// - Returns an error if listing fails.
func (s *Server) ListSecrets(ctx context.Context, request *api.ListSecretsRequest) (*api.ListSecretsResponse, error) {
	return nil, grpc.Errorf(codes.Unimplemented, "Not yet implemented")
}

// CreateSecret creates and return a `CreateSecretResponse` with a `Secret` based
// on the provided `CreateSecretRequest.SecretSpec`.
// - Returns `InvalidArgument` if the `CreateSecretRequest.SecretSpec` is malformed,
//   or if the secret data is too long or contains invalid characters.
// - Returns `ResourceExhausted` if there are already the maximum number of allowed
//   secrets in the system.
// - Returns an error if the creation fails.
func (s *Server) CreateSecret(ctx context.Context, request *api.CreateSecretRequest) (*api.CreateSecretResponse, error) {
	return nil, grpc.Errorf(codes.Unimplemented, "Not yet implemented")
}

// RemoveSecret removes the secret referenced by `RemoveSecretRequest.ID`.
// - Returns `InvalidArgument` if `RemoveSecretRequest.ID` is empty.
// - Returns `NotFound` if the a secret named `RemoveSecretRequest.ID` is not found.
// - Returns an error if the deletion fails.
func (s *Server) RemoveSecret(ctx context.Context, request *api.RemoveSecretRequest) (*api.RemoveSecretResponse, error) {
	return nil, grpc.Errorf(codes.Unimplemented, "Not yet implemented")
}
