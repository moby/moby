package client

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/ansiescape"
	"github.com/docker/docker/pkg/ioutils"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/tlsconfig"
	"github.com/docker/docker/registry"
	"github.com/docker/notary/client"
	"github.com/docker/notary/pkg/passphrase"
	"github.com/docker/notary/trustmanager"
	"github.com/endophage/gotuf/data"
)

var untrusted bool

func addTrustedFlags(fs *flag.FlagSet, verify bool) {
	var trusted bool
	if e := os.Getenv("DOCKER_CONTENT_TRUST"); e != "" {
		if t, err := strconv.ParseBool(e); t || err != nil {
			// treat any other value as true
			trusted = true
		}
	}
	message := "Skip image signing"
	if verify {
		message = "Skip image verification"
	}
	fs.BoolVar(&untrusted, []string{"-disable-content-trust"}, !trusted, message)
}

func isTrusted() bool {
	return !untrusted
}

var targetRegexp = regexp.MustCompile(`([\S]+): digest: ([\S]+) size: ([\d]+)`)

type target struct {
	reference registry.Reference
	digest    digest.Digest
	size      int64
}

func (cli *DockerCli) trustDirectory() string {
	return filepath.Join(cliconfig.ConfigDir(), "trust")
}

// certificateDirectory returns the directory containing
// TLS certificates for the given server. An error is
// returned if there was an error parsing the server string.
func (cli *DockerCli) certificateDirectory(server string) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}

	return filepath.Join(cliconfig.ConfigDir(), "tls", u.Host), nil
}

func trustServer(index *registry.IndexInfo) (string, error) {
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
	auth cliconfig.AuthConfig
}

func (scs simpleCredentialStore) Basic(u *url.URL) (string, string) {
	return scs.auth.Username, scs.auth.Password
}

func (cli *DockerCli) getNotaryRepository(repoInfo *registry.RepositoryInfo, authConfig cliconfig.AuthConfig) (*client.NotaryRepository, error) {
	server, err := trustServer(repoInfo.Index)
	if err != nil {
		return nil, err
	}

	var cfg = tlsconfig.ClientDefault
	cfg.InsecureSkipVerify = !repoInfo.Index.Secure

	// Get certificate base directory
	certDir, err := cli.certificateDirectory(server)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("reading certificate directory: %s", certDir)

	if err := registry.ReadCertsDirectory(&cfg, certDir); err != nil {
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
	modifiers := registry.DockerHeaders(http.Header{})
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

	return client.NewNotaryRepository(cli.trustDirectory(), repoInfo.CanonicalName, server, tr, cli.getPassphraseRetriever())
}

func convertTarget(t client.Target) (target, error) {
	h, ok := t.Hashes["sha256"]
	if !ok {
		return target{}, errors.New("no valid hash, expecting sha256")
	}
	return target{
		reference: registry.ParseReference(t.Name),
		digest:    digest.NewDigestFromHex("sha256", hex.EncodeToString(h)),
		size:      t.Length,
	}, nil
}

func (cli *DockerCli) getPassphraseRetriever() passphrase.Retriever {
	aliasMap := map[string]string{
		"root":     "root",
		"snapshot": "repository",
		"targets":  "repository",
	}
	baseRetriever := passphrase.PromptRetrieverWithInOut(cli.in, cli.out, aliasMap)
	env := map[string]string{
		"root":     os.Getenv("DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE"),
		"snapshot": os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
		"targets":  os.Getenv("DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE"),
	}

	// Backwards compatibility with old env names. We should remove this in 1.10
	if env["root"] == "" {
		if passphrase := os.Getenv("DOCKER_CONTENT_TRUST_OFFLINE_PASSPHRASE"); passphrase != "" {
			env["root"] = passphrase
			fmt.Fprintf(cli.err, "[DEPRECATED] The environment variable DOCKER_CONTENT_TRUST_OFFLINE_PASSPHRASE has been deprecated and will be removed in v1.10. Please use DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE\n")
		}
	}
	if env["snapshot"] == "" || env["targets"] == "" {
		if passphrase := os.Getenv("DOCKER_CONTENT_TRUST_TAGGING_PASSPHRASE"); passphrase != "" {
			env["snapshot"] = passphrase
			env["targets"] = passphrase
			fmt.Fprintf(cli.err, "[DEPRECATED] The environment variable DOCKER_CONTENT_TRUST_TAGGING_PASSPHRASE has been deprecated and will be removed in v1.10. Please use DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE\n")
		}
	}

	return func(keyName string, alias string, createNew bool, numAttempts int) (string, bool, error) {
		if v := env[alias]; v != "" {
			return v, numAttempts > 1, nil
		}
		return baseRetriever(keyName, alias, createNew, numAttempts)
	}
}

func (cli *DockerCli) trustedReference(repo string, ref registry.Reference) (registry.Reference, error) {
	repoInfo, err := registry.ParseRepositoryInfo(repo)
	if err != nil {
		return nil, err
	}

	// Resolve the Auth config relevant for this server
	authConfig := registry.ResolveAuthConfig(cli.configFile, repoInfo.Index)

	notaryRepo, err := cli.getNotaryRepository(repoInfo, authConfig)
	if err != nil {
		fmt.Fprintf(cli.out, "Error establishing connection to trust repository: %s\n", err)
		return nil, err
	}

	t, err := notaryRepo.GetTargetByName(ref.String())
	if err != nil {
		return nil, err
	}
	r, err := convertTarget(*t)
	if err != nil {
		return nil, err

	}

	return registry.DigestReference(r.digest), nil
}

func (cli *DockerCli) tagTrusted(repoInfo *registry.RepositoryInfo, trustedRef, ref registry.Reference) error {
	fullName := trustedRef.ImageName(repoInfo.LocalName)
	fmt.Fprintf(cli.out, "Tagging %s as %s\n", fullName, ref.ImageName(repoInfo.LocalName))
	tv := url.Values{}
	tv.Set("repo", repoInfo.LocalName)
	tv.Set("tag", ref.String())
	tv.Set("force", "1")

	if _, _, err := readBody(cli.call("POST", "/images/"+fullName+"/tag?"+tv.Encode(), nil, nil)); err != nil {
		return err
	}

	return nil
}

func notaryError(err error) error {
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

func (cli *DockerCli) trustedPull(repoInfo *registry.RepositoryInfo, ref registry.Reference, authConfig cliconfig.AuthConfig) error {
	var (
		v    = url.Values{}
		refs = []target{}
	)

	notaryRepo, err := cli.getNotaryRepository(repoInfo, authConfig)
	if err != nil {
		fmt.Fprintf(cli.out, "Error establishing connection to trust repository: %s\n", err)
		return err
	}

	if ref.String() == "" {
		// List all targets
		targets, err := notaryRepo.ListTargets()
		if err != nil {
			return notaryError(err)
		}
		for _, tgt := range targets {
			t, err := convertTarget(*tgt)
			if err != nil {
				fmt.Fprintf(cli.out, "Skipping target for %q\n", repoInfo.LocalName)
				continue
			}
			refs = append(refs, t)
		}
	} else {
		t, err := notaryRepo.GetTargetByName(ref.String())
		if err != nil {
			return notaryError(err)
		}
		r, err := convertTarget(*t)
		if err != nil {
			return err

		}
		refs = append(refs, r)
	}

	v.Set("fromImage", repoInfo.LocalName)
	for i, r := range refs {
		displayTag := r.reference.String()
		if displayTag != "" {
			displayTag = ":" + displayTag
		}
		fmt.Fprintf(cli.out, "Pull (%d of %d): %s%s@%s\n", i+1, len(refs), repoInfo.LocalName, displayTag, r.digest)
		v.Set("tag", r.digest.String())

		_, _, err = cli.clientRequestAttemptLogin("POST", "/images/create?"+v.Encode(), nil, cli.out, repoInfo.Index, "pull")
		if err != nil {
			return err
		}

		// If reference is not trusted, tag by trusted reference
		if !r.reference.HasDigest() {
			if err := cli.tagTrusted(repoInfo, registry.DigestReference(r.digest), r.reference); err != nil {
				return err

			}
		}
	}
	return nil
}

func selectKey(keys map[string]string) string {
	if len(keys) == 0 {
		return ""
	}

	keyIDs := []string{}
	for k := range keys {
		keyIDs = append(keyIDs, k)
	}

	// TODO(dmcgowan): let user choose if multiple keys, now pick consistently
	sort.Strings(keyIDs)

	return keyIDs[0]
}

func targetStream(in io.Writer) (io.WriteCloser, <-chan []target) {
	r, w := io.Pipe()
	out := io.MultiWriter(in, w)
	targetChan := make(chan []target)

	go func() {
		targets := []target{}
		scanner := bufio.NewScanner(r)
		scanner.Split(ansiescape.ScanANSILines)
		for scanner.Scan() {
			line := scanner.Bytes()
			if matches := targetRegexp.FindSubmatch(line); len(matches) == 4 {
				dgst, err := digest.ParseDigest(string(matches[2]))
				if err != nil {
					// Line does match what is expected, continue looking for valid lines
					logrus.Debugf("Bad digest value %q in matched line, ignoring\n", string(matches[2]))
					continue
				}
				s, err := strconv.ParseInt(string(matches[3]), 10, 64)
				if err != nil {
					// Line does match what is expected, continue looking for valid lines
					logrus.Debugf("Bad size value %q in matched line, ignoring\n", string(matches[3]))
					continue
				}

				targets = append(targets, target{
					reference: registry.ParseReference(string(matches[1])),
					digest:    dgst,
					size:      s,
				})
			}
		}
		targetChan <- targets
	}()

	return ioutils.NewWriteCloserWrapper(out, w.Close), targetChan
}

func (cli *DockerCli) trustedPush(repoInfo *registry.RepositoryInfo, tag string, authConfig cliconfig.AuthConfig) error {
	streamOut, targetChan := targetStream(cli.out)

	v := url.Values{}
	v.Set("tag", tag)

	_, _, err := cli.clientRequestAttemptLogin("POST", "/images/"+repoInfo.LocalName+"/push?"+v.Encode(), nil, streamOut, repoInfo.Index, "push")
	// Close stream channel to finish target parsing
	if err := streamOut.Close(); err != nil {
		return err
	}
	// Check error from request
	if err != nil {
		return err
	}

	// Get target results
	targets := <-targetChan

	if tag == "" {
		fmt.Fprintf(cli.out, "No tag specified, skipping trust metadata push\n")
		return nil
	}
	if len(targets) == 0 {
		fmt.Fprintf(cli.out, "No targets found, skipping trust metadata push\n")
		return nil
	}

	fmt.Fprintf(cli.out, "Signing and pushing trust metadata\n")

	repo, err := cli.getNotaryRepository(repoInfo, authConfig)
	if err != nil {
		fmt.Fprintf(cli.out, "Error establishing connection to notary repository: %s\n", err)
		return err
	}

	for _, target := range targets {
		h, err := hex.DecodeString(target.digest.Hex())
		if err != nil {
			return err
		}
		t := &client.Target{
			Name: target.reference.String(),
			Hashes: data.Hashes{
				string(target.digest.Algorithm()): h,
			},
			Length: int64(target.size),
		}
		if err := repo.AddTarget(t); err != nil {
			return err
		}
	}

	err = repo.Publish()
	if _, ok := err.(*client.ErrRepoNotInitialized); !ok {
		return notaryError(err)
	}

	ks := repo.KeyStoreManager
	keys := ks.RootKeyStore().ListKeys()

	rootKey := selectKey(keys)
	if rootKey == "" {
		rootKey, err = ks.GenRootKey("ecdsa")
		if err != nil {
			return err
		}
	}

	cryptoService, err := ks.GetRootCryptoService(rootKey)
	if err != nil {
		return err
	}

	if err := repo.Initialize(cryptoService); err != nil {
		return notaryError(err)
	}
	fmt.Fprintf(cli.out, "Finished initializing %q\n", repoInfo.CanonicalName)

	return notaryError(repo.Publish())
}
