package buildkit

import (
	"context"
	"testing"

	bkauth "github.com/moby/buildkit/session/auth"
	"github.com/moby/moby/api/types/registry"
	registryservice "github.com/moby/moby/v2/daemon/pkg/registry"
	"golang.org/x/crypto/nacl/sign"
	"gotest.tools/v3/assert"
)

func TestStaticRegistryAuthProviderCredentials(t *testing.T) {
	p := newStaticRegistryAuthProvider(map[string]registry.AuthConfig{
		registryservice.IndexServer: {
			Username: "docker-user",
			Password: "docker-pass",
		},
		"https://registry.example.com/v1/": {
			IdentityToken: "identity-token",
		},
		"server-address-entry": {
			Username:      "server-address-user",
			Password:      "server-address-pass",
			ServerAddress: "https://server-address.example.com/v1/",
		},
	}, nil)

	resp, err := p.Credentials(context.Background(), &bkauth.CredentialsRequest{Host: registryservice.DefaultRegistryHost})
	assert.NilError(t, err)
	assert.Equal(t, resp.Username, "docker-user")
	assert.Equal(t, resp.Secret, "docker-pass")

	resp, err = p.Credentials(context.Background(), &bkauth.CredentialsRequest{Host: "registry.example.com"})
	assert.NilError(t, err)
	assert.Equal(t, resp.Username, "")
	assert.Equal(t, resp.Secret, "identity-token")

	resp, err = p.Credentials(context.Background(), &bkauth.CredentialsRequest{Host: "server-address.example.com"})
	assert.NilError(t, err)
	assert.Equal(t, resp.Username, "server-address-user")
	assert.Equal(t, resp.Secret, "server-address-pass")
}

func TestStaticRegistryAuthProviderFetchTokenUsesRegistryToken(t *testing.T) {
	p := newStaticRegistryAuthProvider(map[string]registry.AuthConfig{
		"ghcr.io": {
			RegistryToken: "registry-token",
		},
	}, nil)

	resp, err := p.FetchToken(context.Background(), &bkauth.FetchTokenRequest{Host: "ghcr.io"})
	assert.NilError(t, err)
	assert.Equal(t, resp.Token, "registry-token")
	assert.Equal(t, resp.ExpiresIn, int64(defaultAuthTokenExpiration))
}

func TestStaticRegistryAuthProviderTokenAuthority(t *testing.T) {
	p := newStaticRegistryAuthProvider(map[string]registry.AuthConfig{
		"registry.example.com": {
			Username: "docker-user",
			Password: "docker-pass",
		},
	}, nil)

	salt := []byte("salt")
	payload := []byte("payload")
	pubResp, err := p.GetTokenAuthority(context.Background(), &bkauth.GetTokenAuthorityRequest{
		Host: "registry.example.com",
		Salt: salt,
	})
	assert.NilError(t, err)

	verifyResp, err := p.VerifyTokenAuthority(context.Background(), &bkauth.VerifyTokenAuthorityRequest{
		Host:    "registry.example.com",
		Salt:    salt,
		Payload: payload,
	})
	assert.NilError(t, err)

	var pubKey [32]byte
	copy(pubKey[:], pubResp.PublicKey)
	dt, ok := sign.Open(nil, verifyResp.Signed, &pubKey)
	assert.Assert(t, ok)
	assert.DeepEqual(t, dt, payload)
}
