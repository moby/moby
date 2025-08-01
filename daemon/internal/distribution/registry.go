package distribution

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	registrytypes "github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/v2/daemon/pkg/registry"
	"github.com/moby/moby/v2/dockerversion"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	// supportedMediaTypes represents acceptable media-type(-prefixes)
	// we use this list to prevent obscure errors when trying to pull
	// OCI artifacts.
	supportedMediaTypes = []string{
		// valid prefixes
		"application/vnd.oci.image",
		"application/vnd.docker",

		// these types may occur on old images, and are copied from
		// defaultImageTypes below.
		"application/octet-stream",
		"application/json",
		"text/html",
		"",
	}

	// defaultImageTypes represents the schema2 config types for images
	defaultImageTypes = []string{
		schema2.MediaTypeImageConfig,
		ocispec.MediaTypeImageConfig,
		// Handle unexpected values from https://github.com/docker/distribution/issues/1621
		// (see also https://github.com/moby/moby/issues/22378,
		// https://github.com/moby/moby/issues/30083)
		"application/octet-stream",
		"application/json",
		"text/html",
		// Treat defaulted values as images, newer types cannot be implied
		"",
	}

	// pluginTypes represents the schema2 config types for plugins
	pluginTypes = []string{
		schema2.MediaTypePluginConfig,
	}

	mediaTypeClasses map[string]string
)

func init() {
	// initialize media type classes with all know types for images and plugins.
	mediaTypeClasses = map[string]string{}
	for _, t := range defaultImageTypes {
		mediaTypeClasses[t] = "image"
	}
	for _, t := range pluginTypes {
		mediaTypeClasses[t] = "plugin"
	}
}

// newRepository returns a repository (v2 only). It creates an HTTP transport
// providing timeout settings and authentication support, and also verifies the
// remote API version.
func newRepository(
	ctx context.Context, ref reference.Named, endpoint registry.APIEndpoint,
	metaHeaders http.Header, authConfig *registrytypes.AuthConfig, actions ...string,
) (distribution.Repository, error) {
	// Trim the hostname to form the RemoteName
	repoName := reference.Path(ref)

	direct := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// TODO(dmcgowan): Call close idle connections when complete, use keep alive
	base := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         direct.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     endpoint.TLSConfig,
		// TODO(dmcgowan): Call close idle connections when complete and use keep alive
		DisableKeepAlives: true,
	}

	modifiers := registry.Headers(dockerversion.DockerUserAgent(ctx), metaHeaders)
	authTransport := newTransport(base, modifiers...)

	challengeManager, err := registry.PingV2Registry(endpoint.URL, authTransport)
	if err != nil {
		transportOK := false
		if responseErr, ok := err.(registry.PingResponseError); ok {
			transportOK = true
			err = responseErr.Err
		}
		return nil, fallbackError{
			err:         err,
			transportOK: transportOK,
		}
	}

	if authConfig.RegistryToken != "" {
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, &passThruTokenHandler{token: authConfig.RegistryToken}))
	} else {
		creds := registry.NewStaticCredentialStore(authConfig)
		tokenHandler := auth.NewTokenHandlerWithOptions(auth.TokenHandlerOptions{
			Transport:   authTransport,
			Credentials: creds,
			Scopes: []auth.Scope{auth.RepositoryScope{
				Repository: repoName,
				Actions:    actions,
			}},
			ClientID: registry.AuthClientID,
		})
		basicHandler := auth.NewBasicHandler(creds)
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler))
	}

	tr := newTransport(base, modifiers...)

	// FIXME(thaJeztah): should this just take the original repoInfo.Name instead of converting the remote name back to a named reference?
	repoNameRef, err := reference.WithName(repoName)
	if err != nil {
		return nil, fallbackError{
			err:         err,
			transportOK: true,
		}
	}

	repo, err := client.NewRepository(repoNameRef, endpoint.URL.String(), tr)
	if err != nil {
		return nil, fallbackError{
			err:         err,
			transportOK: true,
		}
	}

	return repo, nil
}

type passThruTokenHandler struct {
	token string
}

func (th *passThruTokenHandler) Scheme() string {
	return "bearer"
}

func (th *passThruTokenHandler) AuthorizeRequest(req *http.Request, params map[string]string) error {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", th.token))
	return nil
}
