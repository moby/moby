package distribution

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/distribution/xfer"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

// fallbackError wraps an error that can possibly allow fallback to a different
// endpoint.
type fallbackError struct {
	// err is the error being wrapped.
	err error
	// confirmedV2 is set to true if it was confirmed that the registry
	// supports the v2 protocol. This is used to limit fallbacks to the v1
	// protocol.
	confirmedV2 bool
}

// Error renders the FallbackError as a string.
func (f fallbackError) Error() string {
	return f.err.Error()
}

type dumbCredentialStore struct {
	auth *types.AuthConfig
}

func (dcs dumbCredentialStore) Basic(*url.URL) (string, string) {
	return dcs.auth.Username, dcs.auth.Password
}

// NewV2Repository returns a repository (v2 only). It creates a HTTP transport
// providing timeout settings and authentication support, and also verifies the
// remote API version.
func NewV2Repository(ctx context.Context, repoInfo *registry.RepositoryInfo, endpoint registry.APIEndpoint, metaHeaders http.Header, authConfig *types.AuthConfig, actions ...string) (repo distribution.Repository, foundVersion bool, err error) {
	repoName := repoInfo.FullName()
	// If endpoint does not support CanonicalName, use the RemoteName instead
	if endpoint.TrimHostname {
		repoName = repoInfo.RemoteName()
	}

	// TODO(dmcgowan): Call close idle connections when complete, use keep alive
	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     endpoint.TLSConfig,
		// TODO(dmcgowan): Call close idle connections when complete and use keep alive
		DisableKeepAlives: true,
	}

	modifiers := registry.DockerHeaders(metaHeaders)
	authTransport := transport.NewTransport(base, modifiers...)
	pingClient := &http.Client{
		Transport: authTransport,
		Timeout:   15 * time.Second,
	}
	endpointStr := strings.TrimRight(endpoint.URL, "/") + "/v2/"
	req, err := http.NewRequest("GET", endpointStr, nil)
	if err != nil {
		return nil, false, err
	}
	resp, err := pingClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	v2Version := auth.APIVersion{
		Type:    "registry",
		Version: "2.0",
	}

	versions := auth.APIVersions(resp, registry.DefaultRegistryVersionHeader)
	for _, pingVersion := range versions {
		if pingVersion == v2Version {
			// The version header indicates we're definitely
			// talking to a v2 registry. So don't allow future
			// fallbacks to the v1 protocol.

			foundVersion = true
			break
		}
	}

	challengeManager := auth.NewSimpleChallengeManager()
	if err := challengeManager.AddResponse(resp); err != nil {
		return nil, foundVersion, err
	}

	if authConfig.RegistryToken != "" {
		passThruTokenHandler := &existingTokenHandler{token: authConfig.RegistryToken}
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, passThruTokenHandler))
	} else {
		creds := dumbCredentialStore{auth: authConfig}
		tokenHandler := auth.NewTokenHandler(authTransport, creds, repoName, actions...)
		basicHandler := auth.NewBasicHandler(creds)
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler))
	}
	tr := transport.NewTransport(base, modifiers...)

	repo, err = client.NewRepository(ctx, repoName, endpoint.URL, tr)
	return repo, foundVersion, err
}

type existingTokenHandler struct {
	token string
}

func (th *existingTokenHandler) Scheme() string {
	return "bearer"
}

func (th *existingTokenHandler) AuthorizeRequest(req *http.Request, params map[string]string) error {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", th.token))
	return nil
}

// retryOnError wraps the error in xfer.DoNotRetry if we should not retry the
// operation after this error.
func retryOnError(err error) error {
	switch v := err.(type) {
	case errcode.Errors:
		return retryOnError(v[0])
	case errcode.Error:
		switch v.Code {
		case errcode.ErrorCodeUnauthorized, errcode.ErrorCodeUnsupported, errcode.ErrorCodeDenied:
			return xfer.DoNotRetry{Err: err}
		}
	case *client.UnexpectedHTTPResponseError:
		return xfer.DoNotRetry{Err: err}
	case error:
		if strings.Contains(err.Error(), strings.ToLower(syscall.ENOSPC.Error())) {
			return xfer.DoNotRetry{Err: err}
		}
	}
	// let's be nice and fallback if the error is a completely
	// unexpected one.
	// If new errors have to be handled in some way, please
	// add them to the switch above.
	return err
}
