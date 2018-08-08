package secrets

import (
	"context"

	"github.com/moby/buildkit/session"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SecretStore interface {
	GetSecret(context.Context, string) ([]byte, error)
}

var ErrNotFound = errors.Errorf("not found")

func GetSecret(ctx context.Context, c session.Caller, id string) ([]byte, error) {
	client := NewSecretsClient(c.Conn())
	resp, err := client.GetSecret(ctx, &GetSecretRequest{
		ID: id,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && (st.Code() == codes.Unimplemented || st.Code() == codes.NotFound) {
			return nil, errors.Wrapf(ErrNotFound, "secret %s not found", id)
		}
		return nil, err
	}
	return resp.Data, nil
}
