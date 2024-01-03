package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"sync"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/pkg/errors"
	"golang.org/x/crypto/nacl/sign"
	"google.golang.org/grpc/codes"
)

var salt []byte
var saltOnce sync.Once

// getSalt returns unique component per daemon restart to avoid persistent keys
func getSalt() []byte {
	saltOnce.Do(func() {
		salt = make([]byte, 32)
		rand.Read(salt)
	})
	return salt
}

func CredentialsFunc(sm *session.Manager, g session.Group) func(string) (session, username, secret string, err error) {
	return func(host string) (string, string, string, error) {
		var sessionID, user, secret string
		err := sm.Any(context.TODO(), g, func(ctx context.Context, id string, c session.Caller) error {
			client := NewAuthClient(c.Conn())

			resp, err := client.Credentials(ctx, &CredentialsRequest{
				Host: host,
			})
			if err != nil {
				if grpcerrors.Code(err) == codes.Unimplemented {
					return nil
				}
				return err
			}
			sessionID = id
			user = resp.Username
			secret = resp.Secret
			return nil
		})
		if err != nil {
			return "", "", "", err
		}
		return sessionID, user, secret, nil
	}
}

func FetchToken(ctx context.Context, req *FetchTokenRequest, sm *session.Manager, g session.Group) (resp *FetchTokenResponse, err error) {
	err = sm.Any(ctx, g, func(ctx context.Context, id string, c session.Caller) error {
		client := NewAuthClient(c.Conn())

		resp, err = client.FetchToken(ctx, req)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func VerifyTokenAuthority(ctx context.Context, host string, pubKey *[32]byte, sm *session.Manager, g session.Group) (sessionID string, ok bool, err error) {
	var verified bool
	err = sm.Any(ctx, g, func(ctx context.Context, id string, c session.Caller) error {
		client := NewAuthClient(c.Conn())

		payload := make([]byte, 32)
		rand.Read(payload)
		resp, err := client.VerifyTokenAuthority(ctx, &VerifyTokenAuthorityRequest{
			Host:    host,
			Salt:    getSalt(),
			Payload: payload,
		})
		if err != nil {
			if grpcerrors.Code(err) == codes.Unimplemented {
				return nil
			}
			return err
		}
		var dt []byte
		dt, ok = sign.Open(nil, resp.Signed, pubKey)
		if ok && subtle.ConstantTimeCompare(dt, payload) == 1 {
			verified = true
		}
		sessionID = id
		return nil
	})
	if err != nil {
		return "", false, err
	}
	return sessionID, verified, nil
}

func GetTokenAuthority(ctx context.Context, host string, sm *session.Manager, g session.Group) (sessionID string, pubKey *[32]byte, err error) {
	err = sm.Any(ctx, g, func(ctx context.Context, id string, c session.Caller) error {
		client := NewAuthClient(c.Conn())

		resp, err := client.GetTokenAuthority(ctx, &GetTokenAuthorityRequest{
			Host: host,
			Salt: getSalt(),
		})
		if err != nil {
			if grpcerrors.Code(err) == codes.Unimplemented || grpcerrors.Code(err) == codes.Unavailable {
				return nil
			}
			return err
		}
		if len(resp.PublicKey) != 32 {
			return errors.Errorf("invalid pubkey length %d", len(pubKey))
		}

		sessionID = id
		pubKey = new([32]byte)
		copy((*pubKey)[:], resp.PublicKey)
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	return sessionID, pubKey, nil
}
