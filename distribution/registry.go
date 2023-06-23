package distribution // import "github.com/docker/docker/distribution"

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/registry"
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
		// (see also https://github.com/docker/docker/issues/22378,
		// https://github.com/docker/docker/issues/30083)
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
	ctx context.Context, repoInfo *registry.RepositoryInfo, endpoint registry.APIEndpoint,
	metaHeaders http.Header, authConfig *registrytypes.AuthConfig, actions ...string,
) (repo distribution.Repository, err error) {
	repoName := repoInfo.Name.Name()
	// If endpoint does not support CanonicalName, use the RemoteName instead
	if endpoint.TrimHostname {
		repoName = reference.Path(repoInfo.Name)
	}

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

	// Using distributionRepositoryWithManifestInfo as a wrapper for the distribution.Repository, to add the manifest
	// tag header to all requests during push/pull. This implementation assumes the repository instance returned by this
	// function is used for a single push/pull at a time (not in concurrent)
	distRepo := &distributionRepositoryWithManifestInfo{}

	modifiers := registry.Headers(dockerversion.DockerUserAgent(ctx), metaHeaders)
	modifiers = append(modifiers, distRepo)
	authTransport := transport.NewTransport(base, modifiers...)

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
		passThruTokenHandler := &existingTokenHandler{token: authConfig.RegistryToken}
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, passThruTokenHandler))
	} else {
		scope := auth.RepositoryScope{
			Repository: repoName,
			Actions:    actions,
			Class:      repoInfo.Class,
		}

		creds := registry.NewStaticCredentialStore(authConfig)
		tokenHandlerOptions := auth.TokenHandlerOptions{
			Transport:   authTransport,
			Credentials: creds,
			Scopes:      []auth.Scope{scope},
			ClientID:    registry.AuthClientID,
		}
		tokenHandler := auth.NewTokenHandlerWithOptions(tokenHandlerOptions)
		basicHandler := auth.NewBasicHandler(creds)
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler))
	}
	tr := transport.NewTransport(base, modifiers...)

	repoNameRef, err := reference.WithName(repoName)
	if err != nil {
		return nil, fallbackError{
			err:         err,
			transportOK: true,
		}
	}

	distRepo.Repository, err = client.NewRepository(repoNameRef, endpoint.URL.String(), tr)
	if err != nil {
		err = fallbackError{
			err:         err,
			transportOK: true,
		}
	}
	repo = distRepo
	return
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
