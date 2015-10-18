package client

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/pkg/ansiescape"
	"github.com/docker/docker/pkg/ioutils"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/registry"
	"github.com/docker/notary/client"
	"github.com/docker/notary/pkg/passphrase"
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

func (cli *DockerCli) getNotaryRepository(repoInfo *registry.RepositoryInfo, authConfig cliconfig.AuthConfig) (*client.NotaryRepository, error) {
	trustDirectory := filepath.Join(cliconfig.ConfigDir(), "trust")
	certificateRootDirectory := filepath.Join(cliconfig.ConfigDir(), "tls")
	return registry.GetNotaryRepository(repoInfo, trustDirectory, certificateRootDirectory, &authConfig, cli.getPassphraseRetriever())
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
		env["root"] = os.Getenv("DOCKER_CONTENT_TRUST_OFFLINE_PASSPHRASE")
		fmt.Fprintf(cli.err, "[DEPRECATED] The environment variable DOCKER_CONTENT_TRUST_OFFLINE_PASSPHRASE has been deprecated and will be removed in v1.10. Please use DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE\n")

	}
	if env["snapshot"] == "" || env["targets"] == "" {
		env["snapshot"] = os.Getenv("DOCKER_CONTENT_TRUST_TAGGING_PASSPHRASE")
		env["targets"] = os.Getenv("DOCKER_CONTENT_TRUST_TAGGING_PASSPHRASE")
		fmt.Fprintf(cli.err, "[DEPRECATED] The environment variable DOCKER_CONTENT_TRUST_TAGGING_PASSPHRASE has been deprecated and will be removed in v1.10. Please use DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE\n")

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

	r, err := registry.ResolveTagByNotary(notaryRepo, ref.String())
	if err != nil {
		return nil, err
	}

	return registry.DigestReference(r.Digest), nil
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

func (cli *DockerCli) trustedPull(repoInfo *registry.RepositoryInfo, ref registry.Reference, authConfig cliconfig.AuthConfig) error {
	var v = url.Values{}

	notaryRepo, err := cli.getNotaryRepository(repoInfo, authConfig)
	if err != nil {
		fmt.Fprintf(cli.out, "Error establishing connection to trust repository: %s\n", err)
		return err
	}

	refs, err := registry.ResolveTagSetByNotary(notaryRepo, ref.String())
	if err != nil {
		return err
	}

	v.Set("fromImage", repoInfo.LocalName)
	for i, r := range refs {
		displayTag := r.Reference.String()
		if displayTag != "" {
			displayTag = ":" + displayTag
		}
		fmt.Fprintf(cli.out, "Pull (%d of %d): %s%s@%s\n", i+1, len(refs), repoInfo.LocalName, displayTag, r.Digest)
		v.Set("tag", r.Digest.String())

		_, _, err = cli.clientRequestAttemptLogin("POST", "/images/create?"+v.Encode(), nil, cli.out, repoInfo.Index, "pull")
		if err != nil {
			return err
		}

		// If reference is not trusted, tag by trusted reference
		if !r.Reference.HasDigest() {
			if err := cli.tagTrusted(repoInfo, registry.DigestReference(r.Digest), r.Reference); err != nil {
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

func targetStream(in io.Writer) (io.WriteCloser, <-chan []registry.ResolvedTag) {
	r, w := io.Pipe()
	out := io.MultiWriter(in, w)
	targetChan := make(chan []registry.ResolvedTag)

	go func() {
		targets := []registry.ResolvedTag{}
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

				targets = append(targets, registry.ResolvedTag{
					Reference: registry.ParseReference(string(matches[1])),
					Digest:    dgst,
					Size:      s,
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
		h, err := hex.DecodeString(target.Digest.Hex())
		if err != nil {
			return err
		}
		t := &client.Target{
			Name: target.Reference.String(),
			Hashes: data.Hashes{
				string(target.Digest.Algorithm()): h,
			},
			Length: int64(target.Size),
		}
		if err := repo.AddTarget(t); err != nil {
			return err
		}
	}

	err = repo.Publish()
	if _, ok := err.(*client.ErrRepoNotInitialized); !ok {
		return registry.WrapNotaryError(err)
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
		return registry.WrapNotaryError(err)
	}
	fmt.Fprintf(cli.out, "Finished initializing %q\n", repoInfo.CanonicalName)

	return registry.WrapNotaryError(repo.Publish())
}
