package auth

import (
	"context"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/grpcerrors"
	"google.golang.org/grpc/codes"
)

func CredentialsFunc(ctx context.Context, c session.Caller) func(string) (string, string, error) {
	return func(host string) (string, string, error) {
		client := NewAuthClient(c.Conn())

		resp, err := client.Credentials(ctx, &CredentialsRequest{
			Host: host,
		})
		if err != nil {
			if grpcerrors.Code(err) == codes.Unimplemented {
				return "", "", nil
			}
			return "", "", err
		}
		return resp.Username, resp.Secret, nil
	}
}
