package registry

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/tlsconfig"
	"github.com/docker/notary/client"
	"github.com/docker/notary/pkg/passphrase"
	"github.com/docker/notary/trustmanager"
)

// WrapNotaryError returns an error object with a description customized for Docker's use of the notary
// instead of a generic text.
func WrapNotaryError(err error) error {
	switch err.(type) {
	case *json.SyntaxError:
		logrus.Debugf("Notary syntax error: %s", err)
		return errors.New("no trust data available for remote repository")
	case client.ErrExpired:
		return fmt.Errorf("remote repository out-of-date: %v", err)
	case trustmanager.ErrKeyNotFound:
		return fmt.Errorf("signing keys not found: %v", err)
	case *net.OpError:
		return fmt.Errorf("error contacting notary server: %v", err)
	}

	return err
}

func trustServer(index *IndexInfo) (string, error) {
	if s := os.Getenv("DOCKER_CONTENT_TRUST_SERVER"); s != "" {
		urlObj, err := url.Parse(s)
		if err != nil || urlObj.Scheme != "https" {
			return "", fmt.Errorf("valid https URL required for trust server, got %s", s)
		}

		return s, nil
	}
	if index.Official {
		return NotaryServer, nil
	}
	return "https://" + index.Name, nil
}

type simpleCredentialStore struct {
	auth *cliconfig.AuthConfig
}

func (scs simpleCredentialStore) Basic(u *url.URL) (string, string) {
	return scs.auth.Username, scs.auth.Password
}

// GetNotaryRepository returns a fully configured notary client object for a specific repository
// and environment-dependent paths, authentication and passphrase configuration.
func GetNotaryRepository(repoInfo *RepositoryInfo, trustDir, certificateRootDir string,
	authConfig *cliconfig.AuthConfig, retriever passphrase.Retriever) (*client.NotaryRepository, error) {
	server, err := trustServer(repoInfo.Index)
	if err != nil {
		return nil, err
	}

	var cfg = tlsconfig.ClientDefault
	cfg.InsecureSkipVerify = !repoInfo.Index.Secure

	// Get certificate base directory
	u, err := url.Parse(server)
	if err != nil {
		return nil, err
	}
	certDir := filepath.Join(certificateRootDir, u.Host)
	logrus.Debugf("reading certificate directory: %s", certDir)

	if err := ReadCertsDirectory(&cfg, certDir); err != nil {
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
		TLSClientConfig:     &cfg,
		DisableKeepAlives:   true,
	}

	// Skip configuration headers since request is not going to Docker daemon
	modifiers := DockerHeaders(http.Header{})
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

	challengeManager := auth.NewSimpleChallengeManager()

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

	creds := simpleCredentialStore{auth: authConfig}
	tokenHandler := auth.NewTokenHandler(authTransport, creds, repoInfo.CanonicalName, "push", "pull")
	basicHandler := auth.NewBasicHandler(creds)
	modifiers = append(modifiers, transport.RequestModifier(auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler)))
	tr := transport.NewTransport(base, modifiers...)

	return client.NewNotaryRepository(trustDir, repoInfo.CanonicalName, server, tr, retriever)
}

// ResolvedTag associates a Reference (normally with !HasDigest()) with a digest and size, either as an item to be signed
// or as a verified association.
type ResolvedTag struct {
	Reference Reference
	Digest    digest.Digest
	Size      int64
}

func convertTarget(t client.Target) (ResolvedTag, error) {
	h, ok := t.Hashes["sha256"]
	if !ok {
		return ResolvedTag{}, errors.New("no valid hash, expecting sha256")
	}
	return ResolvedTag{
		Reference: ParseReference(t.Name),
		Digest:    digest.NewDigestFromHex("sha256", hex.EncodeToString(h)),
		Size:      t.Length,
	}, nil
}

// ResolveTagByNotary resolves a tag to a verified digest and size
func ResolveTagByNotary(notaryRepo *client.NotaryRepository, tag string) (ResolvedTag, error) {
	t, err := notaryRepo.GetTargetByName(tag)
	if err != nil {
		return ResolvedTag{}, WrapNotaryError(err)
	}
	return convertTarget(*t)
}

// ResolveTagSetByNotary resolves a tag to a verified digest and size resolves a tag or "" (meaning all tags
// in the repository) into a list of tags with the signed digest and size. The treatment of an empty tag
// is intended to mirror behavior of (docker pull) when not given a tag.
func ResolveTagSetByNotary(notaryRepo *client.NotaryRepository, tag string) ([]ResolvedTag, error) {
	refs := []ResolvedTag{}
	if tag == "" {
		// List all targets
		targets, err := notaryRepo.ListTargets()
		if err != nil {
			return nil, WrapNotaryError(err)
		}
		for _, tgt := range targets {
			t, err := convertTarget(*tgt)
			if err != nil {
				logrus.Debugf("Skipping target for %v\n", *tgt)
				continue
			}
			refs = append(refs, t)
		}
	} else {
		t, err := notaryRepo.GetTargetByName(tag)
		if err != nil {
			return nil, WrapNotaryError(err)
		}
		r, err := convertTarget(*t)
		if err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, nil
}
