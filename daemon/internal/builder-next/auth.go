package buildkit

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	dockerauth "github.com/containerd/containerd/v2/core/remotes/docker/auth"
	remoteserrors "github.com/containerd/containerd/v2/core/remotes/errors"
	"github.com/moby/buildkit/session"
	bkauth "github.com/moby/buildkit/session/auth"
	"github.com/moby/moby/api/types/registry"
	registryservice "github.com/moby/moby/v2/daemon/pkg/registry"
	"github.com/pkg/errors"
	"golang.org/x/crypto/nacl/sign"
	"google.golang.org/grpc"
)

const defaultAuthTokenExpiration = 60

type staticRegistryAuthProvider struct {
	bkauth.UnimplementedAuthServer

	mu            sync.Mutex
	authConfigs   map[string]registry.AuthConfig
	registryHosts docker.RegistryHosts
	seeds         map[string][]byte
}

func newBuildkitAuthSession(ctx context.Context, authConfigs map[string]registry.AuthConfig, registryHosts docker.RegistryHosts) (*session.Session, error) {
	s, err := session.NewSession(ctx, "moby-buildkit-auth")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create buildkit auth session")
	}
	s.Allow(newStaticRegistryAuthProvider(authConfigs, registryHosts))
	return s, nil
}

func newStaticRegistryAuthProvider(authConfigs map[string]registry.AuthConfig, registryHosts docker.RegistryHosts) *staticRegistryAuthProvider {
	authConfigsCopy := make(map[string]registry.AuthConfig, len(authConfigs))
	for k, v := range authConfigs {
		authConfigsCopy[k] = v
	}
	return &staticRegistryAuthProvider{
		authConfigs:   authConfigsCopy,
		registryHosts: registryHosts,
		seeds:         map[string][]byte{},
	}
}

func runBuildkitSession(ctx context.Context, sm *session.Manager, s *session.Session) error {
	return s.Run(ctx, func(ctx context.Context, _ string, meta map[string][]string) (net.Conn, error) {
		clientConn, serverConn := net.Pipe()
		go func() {
			if err := sm.HandleConn(ctx, serverConn, meta); err != nil {
				_ = serverConn.Close()
			}
		}()
		return clientConn, nil
	})
}

func (p *staticRegistryAuthProvider) Register(server *grpc.Server) {
	bkauth.RegisterAuthServer(server, p)
}

func (p *staticRegistryAuthProvider) Credentials(_ context.Context, req *bkauth.CredentialsRequest) (*bkauth.CredentialsResponse, error) {
	return toBuildkitCredentials(p.authConfig(req.Host)), nil
}

func (p *staticRegistryAuthProvider) FetchToken(ctx context.Context, req *bkauth.FetchTokenRequest) (*bkauth.FetchTokenResponse, error) {
	ac := p.authConfig(req.Host)
	if ac.RegistryToken != "" {
		return toBuildkitTokenResponse(ac.RegistryToken, time.Time{}, 0), nil
	}

	creds := toBuildkitCredentials(ac)
	to := dockerauth.TokenOptions{
		Realm:    req.Realm,
		Service:  req.Service,
		Scopes:   req.Scopes,
		Username: creds.Username,
		Secret:   creds.Secret,
	}

	client := p.client(req.Host)
	if to.Secret != "" {
		clientID := req.ClientID
		if clientID == "" {
			clientID = "buildkit-client"
		}
		resp, err := dockerauth.FetchTokenWithOAuth(ctx, client, nil, clientID, to)
		if err != nil {
			var errStatus remoteserrors.ErrUnexpectedStatus
			if errors.As(err, &errStatus) {
				if (errStatus.StatusCode == http.StatusMethodNotAllowed && to.Username != "") || errStatus.StatusCode == http.StatusNotFound || errStatus.StatusCode == http.StatusUnauthorized {
					resp, err := dockerauth.FetchToken(ctx, client, nil, to)
					if err != nil {
						return nil, err
					}
					return toBuildkitTokenResponse(resp.Token, resp.IssuedAt, resp.ExpiresInSeconds), nil
				}
			}
			return nil, err
		}
		return toBuildkitTokenResponse(resp.AccessToken, resp.IssuedAt, resp.ExpiresInSeconds), nil
	}

	resp, err := dockerauth.FetchToken(ctx, client, nil, to)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch anonymous token")
	}
	return toBuildkitTokenResponse(resp.Token, resp.IssuedAt, resp.ExpiresInSeconds), nil
}

func (p *staticRegistryAuthProvider) GetTokenAuthority(_ context.Context, req *bkauth.GetTokenAuthorityRequest) (*bkauth.GetTokenAuthorityResponse, error) {
	key := p.authorityKey(req.Host, req.Salt)
	return &bkauth.GetTokenAuthorityResponse{PublicKey: key[32:]}, nil
}

func (p *staticRegistryAuthProvider) VerifyTokenAuthority(_ context.Context, req *bkauth.VerifyTokenAuthorityRequest) (*bkauth.VerifyTokenAuthorityResponse, error) {
	key := p.authorityKey(req.Host, req.Salt)
	priv := new([64]byte)
	copy((*priv)[:], key)
	return &bkauth.VerifyTokenAuthorityResponse{Signed: sign.Sign(nil, req.Payload, priv)}, nil
}

func (p *staticRegistryAuthProvider) authConfig(host string) registry.AuthConfig {
	p.mu.Lock()
	defer p.mu.Unlock()

	if isDockerHubHost(host) {
		if ac, ok := p.authConfigs[registryservice.IndexServer]; ok {
			return ac
		}
	}

	if ac, ok := p.authConfigs[host]; ok {
		return ac
	}

	for registryURL, ac := range p.authConfigs {
		if registryservice.ConvertToHostname(registryURL) == host {
			return ac
		}
		if ac.ServerAddress != "" && registryservice.ConvertToHostname(ac.ServerAddress) == host {
			return ac
		}
	}
	return registry.AuthConfig{}
}

func (p *staticRegistryAuthProvider) client(host string) *http.Client {
	if p.registryHosts != nil {
		hosts, err := p.registryHosts(host)
		if err == nil && len(hosts) > 0 && hosts[0].Client != nil {
			return hosts[0].Client
		}
	}
	return http.DefaultClient
}

func (p *staticRegistryAuthProvider) authorityKey(host string, salt []byte) ed25519.PrivateKey {
	p.mu.Lock()
	seed := p.seeds[host]
	if seed == nil {
		seed = make([]byte, 16)
		_, _ = rand.Read(seed)
		p.seeds[host] = seed
	}
	p.mu.Unlock()

	mac := hmac.New(sha256.New, salt)
	if toBuildkitCredentials(p.authConfig(host)).Secret != "" {
		mac.Write(seed)
	}

	sum := mac.Sum(nil)
	return ed25519.NewKeyFromSeed(sum[:ed25519.SeedSize])
}

func isDockerHubHost(host string) bool {
	return host == registryservice.DefaultRegistryHost || host == registryservice.IndexHostname || host == registryservice.IndexName
}

func toBuildkitCredentials(ac registry.AuthConfig) *bkauth.CredentialsResponse {
	res := &bkauth.CredentialsResponse{}
	if ac.IdentityToken != "" {
		res.Secret = ac.IdentityToken
	} else {
		res.Username = ac.Username
		res.Secret = ac.Password
	}
	return res
}

func toBuildkitTokenResponse(token string, issuedAt time.Time, expires int) *bkauth.FetchTokenResponse {
	if expires == 0 {
		expires = defaultAuthTokenExpiration
	}
	resp := &bkauth.FetchTokenResponse{
		Token:     token,
		ExpiresIn: int64(expires),
	}
	if !issuedAt.IsZero() {
		resp.IssuedAt = issuedAt.Unix()
	}
	return resp
}
