package auth

import (
	"context"

	"github.com/moby/buildkit/session"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func CredentialsFunc(ctx context.Context, c session.Caller) func(string) (string, string, error) {
	return func(host string) (string, string, error) {
		client := NewAuthClient(c.Conn())

		resp, err := client.Credentials(ctx, &CredentialsRequest{
			Host: host,
		})
		if err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == codes.Unimplemented {
				return "", "", nil
			}
			return "", "", err
		}
		return resp.Username, resp.Secret, nil
	}
}
