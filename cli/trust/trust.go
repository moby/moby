package trust

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/registry"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/docker/notary"
	"github.com/docker/notary/client"
	"github.com/docker/notary/passphrase"
	"github.com/docker/notary/storage"
	"github.com/docker/notary/trustmanager"
	"github.com/docker/notary/trustpinning"
	"github.com/docker/notary/tuf/data"
	"github.com/docker/notary/tuf/signed"
)

var (
	// ReleasesRole is the role named "releases"
	ReleasesRole = path.Join(data.CanonicalTargetsRole, "releases")
)

func trustDirectory() string {
	return filepath.Join(cliconfig.ConfigDir(), "trust")
}

// certificateDirectory returns the directory containing
// TLS certificates for the given server. An error is
// returned if there was an error parsing the server string.
func certificateDirectory(server string) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}

	return filepath.Join(cliconfig.ConfigDir(), "tls", u.Host), nil
}

// Server returns the base URL for the trust server.
func Server(index *registrytypes.IndexInfo) (string, error) {
	if s := os.Getenv("DOCKER_CONTENT_TRUST_SERVER"); s != "" {
		urlObj, err := url.Parse(s)
		if err != nil || urlObj.Scheme != "https" {
			return "", fmt.Errorf("valid https URL required for trust server, got %s", s)
		}

		return s, nil
	}
	if index.Official {
		return registry.NotaryServer, nil
	}
	return "https://" + index.Name, nil
}

type simpleCredentialStore struct {
	auth types.AuthConfig
}

func (scs simpleCredentialStore) Basic(u *url.URL) (string, string) {
	return scs.auth.Username, scs.auth.Password
}

func (scs simpleCredentialStore) RefreshToken(u *url.URL, service string) string {
	return scs.auth.IdentityToken
}

func (scs simpleCredentialStore) SetRefreshToken(*url.URL, string, string) {
}

// GetNotaryRepository returns a NotaryRepository which stores all the
// information needed to operate on a notary repository.
// It creates an HTTP transport providing authentication support.
func GetNotaryRepository(streams command.Streams, repoInfo *registry.RepositoryInfo, authConfig types.AuthConfig, actions ...string) (*client.NotaryRepository, error) {
	server, err := Server(repoInfo.Index)
	if err != nil {
		return nil, err
	}

	var cfg = tlsconfig.ClientDefault()
	cfg.InsecureSkipVerify = !repoInfo.Index.Secure

	// Get certificate base directory
	certDir, err := certificateDirectory(server)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("reading certificate directory: %s", certDir)

	if err := registry.ReadCertsDirectory(cfg, certDir); err != nil {
		return nil, err
	}

	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     cfg,
		DisableKeepAlives:   true,
	}

	// Skip configuration headers since request is not going to Docker daemon
	modifiers := registry.DockerHeaders(command.UserAgent(), http.Header{})
	authTransport := transport.NewTransport(base, modifiers...)
	pingClient := &http.Client{
		Transport: authTransport,
		Timeout:   5 * time.Second,
	}
	endpointStr := server + "/v2/"
	req, err := http.NewRequest("GET", endpointStr, nil)
	if err != nil {
		return nil, err
	}

	challengeManager := challenge.NewSimpleManager()

	resp, err := pingClient.Do(req)
	if err != nil {
		// Ignore error on ping to operate in offline mode
		logrus.Debugf("Error pinging notary server %q: %s", endpointStr, err)
	} else {
		defer resp.Body.Close()

		// Add response to the challenge manager to parse out
		// authentication header and register authentication method
		if err := challengeManager.AddResponse(resp); err != nil {
			return nil, err
		}
	}

	scope := auth.RepositoryScope{
		Repository: repoInfo.FullName(),
		Actions:    actions,
		Class:      repoInfo.Class,
	}
	creds := simpleCredentialStore{auth: authConfig}
	tokenHandlerOptions := auth.TokenHandlerOptions{
		Transport:   authTransport,
		Credentials: creds,
		Scopes:      []auth.Scope{scope},
		ClientID:    registry.AuthClientID,
	}
	tokenHandler := auth.NewTokenHandlerWithOptions(tokenHandlerOptions)
	basicHandler := auth.NewBasicHandler(creds)
	modifiers = append(modifiers, transport.RequestModifier(auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler)))
	tr := transport.NewTransport(base, modifiers...)

	return client.NewNotaryRepository(
		trustDirectory(),
		repoInfo.FullName(),
		server,
		tr,
		getPassphraseRetriever(streams),
		trustpinning.TrustPinConfig{})
}

func getPassphraseRetriever(streams command.Streams) notary.PassRetriever {
	aliasMap := map[string]string{
		"root":     "root",
		"snapshot": "repository",
		"targets":  "repository",
		"default":  "repository",
	}
	baseRetriever := passphrase.PromptRetrieverWithInOut(streams.In(), streams.Out(), aliasMap)
	env := map[string]string{
		"root":     os.Getenv("DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE"),
		"snapshot": os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
		"targets":  os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
		"default":  os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
	}

	return func(keyName string, alias string, createNew bool, numAttempts int) (string, bool, error) {
		if v := env[alias]; v != "" {
			return v, numAttempts > 1, nil
		}
		// For non-root roles, we can also try the "default" alias if it is specified
		if v := env["default"]; v != "" && alias != data.CanonicalRootRole {
			return v, numAttempts > 1, nil
		}
		return baseRetriever(keyName, alias, createNew, numAttempts)
	}
}

// NotaryError formats an error message received from the notary service
func NotaryError(repoName string, err error) error {
	switch err.(type) {
	case *json.SyntaxError:
		logrus.Debugf("Notary syntax error: %s", err)
		return fmt.Errorf("Error: no trust data available for remote repository %s. Try running notary server and setting DOCKER_CONTENT_TRUST_SERVER to its HTTPS address?", repoName)
	case signed.ErrExpired:
		return fmt.Errorf("Error: remote repository %s out-of-date: %v", repoName, err)
	case trustmanager.ErrKeyNotFound:
		return fmt.Errorf("Error: signing keys for remote repository %s not found: %v", repoName, err)
	case storage.NetworkError:
		return fmt.Errorf("Error: error contacting notary server: %v", err)
	case storage.ErrMetaNotFound:
		return fmt.Errorf("Error: trust data missing for remote repository %s or remote repository not found: %v", repoName, err)
	case trustpinning.ErrRootRotationFail, trustpinning.ErrValidationFail, signed.ErrInvalidKeyType:
		return fmt.Errorf("Warning: potential malicious behavior - trust data mismatch for remote repository %s: %v", repoName, err)
	case signed.ErrNoKeys:
		return fmt.Errorf("Error: could not find signing keys for remote repository %s, or could not decrypt signing key: %v", repoName, err)
	case signed.ErrLowVersion:
		return fmt.Errorf("Warning: potential malicious behavior - trust data version is lower than expected for remote repository %s: %v", repoName, err)
	case signed.ErrRoleThreshold:
		return fmt.Errorf("Warning: potential malicious behavior - trust data has insufficient signatures for remote repository %s: %v", repoName, err)
	case client.ErrRepositoryNotExist:
		return fmt.Errorf("Error: remote trust data does not exist for %s: %v", repoName, err)
	case signed.ErrInsufficientSignatures:
		return fmt.Errorf("Error: could not produce valid signature for %s.  If Yubikey was used, was touch input provided?: %v", repoName, err)
	}

	return err
}
