package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"

	digest "github.com/opencontainers/go-digest"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/config/configfile"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/homedir"
	"github.com/docker/docker/registry"
)

type pushOpts struct {
	newRef string
	file   string
}

type existingTokenHandler struct {
	token string
}

func (th *existingTokenHandler) AuthorizeRequest(req *http.Request, params map[string]string) error {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", th.token))
	return nil
}

func (th *existingTokenHandler) Scheme() string {
	return "bearer"
}

type dumbCredentialStore struct {
	auth *types.AuthConfig
}

// YamlInput represents the YAML format input to the pushml
// command.
type YamlInput struct {
	Image     string
	Manifests []YamlManifestEntry
}

// YamlManifestEntry represents an entry in the list of manifests to
// be combined into a manifest list, provided via the YAML input
type YamlManifestEntry struct {
	Image    string
	Platform manifestlist.PlatformSpec
}

// we will store up a list of blobs we must ask the registry
// to cross-mount into our target namespace
type blobMount struct {
	FromRepo string
	Digest   string
}

// if we have mounted blobs referenced from manifests from
// outside the target repository namespace we will need to
// push them to our target's repo as they will be references
// from the final manifest list object we push
type manifestPush struct {
	Name      string
	Digest    string
	JSONBytes []byte
	MediaType string
}

func newPushListCommand(dockerCli *command.DockerCli) *cobra.Command {

	opts := pushOpts{}

	cmd := &cobra.Command{
		Use:   "push [newRef | --file pre-annotated-yaml]",
		Short: "Push a manifest list for an image to a repository",
		Args:  cli.RequiresMinArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return putManifestList(dockerCli, opts, args)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.file, "file", "f", "", "Path to a file containing a manifest list and its annotated constituent manifests")
	return cmd
}

func putManifestList(dockerCli *command.DockerCli, opts pushOpts, args []string) error {
	var (
		manifests         []string
		manifestList      manifestlist.ManifestList
		targetRef         reference.Named
		blobMountRequests []blobMount
		manifestRequests  []manifestPush
		err               error
		yamlInput         YamlInput
	)

	// First get all the info we'll need from either a yaml file, or a user's locally-creatd manifest transation.
	numArgs := len(args)
	if numArgs > 1 {
		return fmt.Errorf("More than one argument provided to 'manifest push'")
	}
	if (numArgs == 0) && (opts.file == "") {
		return fmt.Errorf("Please push using a yaml file or a list created using 'manifest create.'")
	}
	if opts.file != "" {
		yamlInput, err = getYamlInput(dockerCli, opts.file)
		if err != nil {
			return fmt.Errorf("Error retrieving manifests from YAML file: %s", err)
		}
		targetRef, err = reference.ParseNormalizedNamed(yamlInput.Image)
		if err != nil {
			return fmt.Errorf("Error parsing name for manifest list (%s): %v", yamlInput.Image, err)
		}
		if _, isDigested := targetRef.(reference.Canonical); !isDigested {
			targetRef = reference.TagNameOnly(targetRef)
		}
	} else {
		targetRef, err = reference.ParseNormalizedNamed(args[0])
		if err != nil {
			return fmt.Errorf("Error parsing name for manifest list (%s): %v", args[0], err)
		}
		if _, isDigested := targetRef.(reference.Canonical); !isDigested {
			targetRef = reference.TagNameOnly(targetRef)
		}
		manifests, err = getListFilenames(targetRef.String())
		if err != nil {
			return err
		}
	}
	targetRepo, err := registry.ParseRepositoryInfo(targetRef)
	if err != nil {
		return fmt.Errorf("Error parsing repository name for manifest list (%s): %v", opts.newRef, err)
	}
	targetEndpoint, targetRepoName, err := setupRepo(targetRepo)
	if err != nil {
		return fmt.Errorf("Error setting up repository endpoint and references for %q: %v", targetRef, err)
	}

	// Now that targetRepo is set, jump through a lot of hoops to get a Named reference without
	// the domain included (targetRef), and one without the tag (bareRef)
	tagIndex := strings.LastIndex(targetRef.String(), ":")
	if tagIndex < 0 {
		targetRef = reference.TagNameOnly(targetRef)
		tagIndex = strings.IndexRune(targetRef.String(), ':')
	}
	tag := targetRef.String()[tagIndex+1:]
	bareRef, err := reference.WithName(reference.Path(targetRef))
	if err != nil {
		return err
	}
	targetRef, _ = reference.WithTag(bareRef, tag)

	logrus.Debugf("Creating target ref: %s", targetRef.String())

	ctx := context.Background()

	// Now create the manifest list payload by looking up the manifest schemas
	// for the constituent images:
	logrus.Info("Retrieving digests of images...")
	if opts.file == "" {
		// manifests is a list of file paths
		for _, manifestFile := range manifests {
			fileParts := strings.Split(manifestFile, string(filepath.Separator))
			numParts := len(fileParts)
			mfstInspect, err := unmarshalIntoManifestInspect(fileParts[numParts-1], fileParts[numParts-2])
			if err != nil {
				return err
			}
			if mfstInspect.Architecture == "" || mfstInspect.OS == "" {
				return fmt.Errorf("Malformed manifest object. Cannot push to registry.")
			}
			manifest, repoInfo, err := buildManifestObj(targetRepo, mfstInspect)
			if err != nil {
				return err
			}
			manifestList.Manifests = append(manifestList.Manifests, manifest)

			// if this image is in a different repo, we need to add the layer/blob digests to the list of
			// requested blob mounts (cross-repository push) before pushing the manifest list
			manifestRepoName := reference.Path(repoInfo.Name)
			if targetRepoName != manifestRepoName {
				bmr, mr := buildBlobMountRequestLists(mfstInspect, targetRepoName, manifestRepoName)
				blobMountRequests = append(blobMountRequests, bmr...)
				manifestRequests = append(manifestRequests, mr...)
			}
		}
		// @TODO: Pull the dup parts out from these two if/else blocks. Make a list of Manifest objects and run through that
		// doing the dup parts.
	}

	// Set the schema version
	manifestList.Versioned = manifestlist.SchemaVersion

	urlBuilder, err := v2.NewURLBuilderFromString(targetEndpoint.URL.String(), false)
	logrus.Infof("manifest: put: target endpoint url: %s", targetEndpoint.URL.String())
	if err != nil {
		return fmt.Errorf("Can't create URL builder from endpoint (%s): %v", targetEndpoint.URL.String(), err)
	}
	pushURL, err := createManifestURLFromRef(targetRef, urlBuilder)
	if err != nil {
		return fmt.Errorf("Error setting up repository endpoint and references for %q: %v", targetRef, err)
	}
	logrus.Debugf("Manifest list push url: %s", pushURL)

	deserializedManifestList, err := manifestlist.FromDescriptors(manifestList.Manifests)
	if err != nil {
		return fmt.Errorf("Cannot deserialize manifest list: %v", err)
	}
	mediaType, p, err := deserializedManifestList.Payload()
	logrus.Debugf("mediaType of manifestList: %s", mediaType)
	if err != nil {
		return fmt.Errorf("Cannot retrieve payload for HTTP PUT of manifest list: %v", err)

	}
	putRequest, err := http.NewRequest("PUT", pushURL, bytes.NewReader(p))
	if err != nil {
		return fmt.Errorf("HTTP PUT request creation failed: %v", err)
	}
	putRequest.Header.Set("Content-Type", mediaType)

	httpClient, err := getHTTPClient(ctx, dockerCli, targetRepo, targetEndpoint, targetRepoName)
	if err != nil {
		return fmt.Errorf("Failed to setup HTTP client to repository: %v", err)
	}

	// before we push the manifest list, if we have any blob mount requests, we need
	// to ask the registry to mount those blobs in our target so they are available
	// as references
	if err := mountBlobs(httpClient, urlBuilder, targetRef, blobMountRequests); err != nil {
		return fmt.Errorf("Couldn't mount blobs for cross-repository push: %v", err)
	}

	// we also must push any manifests that are referenced in the manifest list into
	// the target namespace
	// Use the untagged target for this so the digest is used
	if err := pushReferences(httpClient, urlBuilder, bareRef, manifestRequests); err != nil {
		return fmt.Errorf("Couldn't push manifests referenced in our manifest list: %v", err)
	}

	resp, err := httpClient.Do(putRequest)
	if err != nil {
		return fmt.Errorf("V2 registry PUT of manifest list failed: %v", err)
	}
	defer resp.Body.Close()

	if statusSuccess(resp.StatusCode) {
		dgstHeader := resp.Header.Get("Docker-Content-Digest")
		dgst, err := digest.Parse(dgstHeader)
		if err != nil {
			return err
		}
		logrus.Infof("Succesfully pushed manifest list %s with digest %s", targetRef, dgst)
		return nil
	}
	return fmt.Errorf("Registry push unsuccessful: response %d: %s", resp.StatusCode, resp.Status)
}

func getYamlInput(dockerCli *command.DockerCli, yamlFile string) (YamlInput, error) {
	logrus.Debugf("YAML file: %s", yamlFile)

	if _, err := os.Stat(yamlFile); err != nil {
		logrus.Debugf("Unable to open file: %s", yamlFile)
	}

	var yamlInput YamlInput
	yamlBuf, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		logrus.Fatalf(fmt.Sprintf("Can't read YAML file %q: %v", yamlFile, err))
	}
	err = yaml.Unmarshal(yamlBuf, &yamlInput)
	if err != nil {
		logrus.Fatalf(fmt.Sprintf("Can't unmarshal YAML file %q: %v", yamlFile, err))
	}
	return yamlInput, nil
}

func buildManifestObj(targetRepo *registry.RepositoryInfo, mfInspect ImgManifestInspect) (manifestlist.ManifestDescriptor, *registry.RepositoryInfo, error) {

	manifestRef, err := reference.ParseNormalizedNamed(mfInspect.RefName)
	if err != nil {
		return manifestlist.ManifestDescriptor{}, nil, err
	}
	repoInfo, err := registry.ParseRepositoryInfo(manifestRef)
	if err != nil {
		return manifestlist.ManifestDescriptor{}, nil, err
	}

	manifestRepoHostname := reference.Domain(repoInfo.Name)
	targetRepoHostname := reference.Domain(targetRepo.Name)
	if manifestRepoHostname != targetRepoHostname {
		return manifestlist.ManifestDescriptor{}, nil, fmt.Errorf("Cannot use source images from a different registry than the target image: %s != %s", manifestRepoHostname, targetRepoHostname)
	}

	manifest := manifestlist.ManifestDescriptor{
		Platform: manifestlist.PlatformSpec{
			Architecture: mfInspect.Architecture,
			OS:           mfInspect.OS,
			OSVersion:    mfInspect.OSVersion,
			OSFeatures:   mfInspect.OSFeatures,
			Variant:      mfInspect.Variant,
			Features:     mfInspect.Features,
		},
	}
	manifest.Descriptor.Digest = mfInspect.Digest
	manifest.Size = mfInspect.Size
	manifest.MediaType = mfInspect.MediaType

	err = manifest.Descriptor.Digest.Validate()
	if err != nil {
		return manifestlist.ManifestDescriptor{}, nil, fmt.Errorf("Digest parse of image %q failed with error: %v", manifestRef, err)
	}

	return manifest, repoInfo, nil
}

func buildBlobMountRequestLists(mfstInspect ImgManifestInspect, targetRepoName, mfRepoName string) ([]blobMount, []manifestPush) {

	var (
		blobMountRequests []blobMount
		manifestRequests  []manifestPush
	)

	logrus.Debugf("Adding manifest references of %q to blob mount requests to %s", mfRepoName, targetRepoName)
	for _, layer := range mfstInspect.References {
		blobMountRequests = append(blobMountRequests, blobMount{FromRepo: mfRepoName, Digest: layer})
	}
	// also must add the manifest to be pushed in the target namespace
	logrus.Debugf("Adding manifest %q -> to be pushed to %q as a manifest reference", mfRepoName, targetRepoName)
	manifestRequests = append(manifestRequests, manifestPush{
		Name:      mfRepoName,
		Digest:    mfstInspect.Digest.String(),
		JSONBytes: mfstInspect.CanonicalJSON,
		MediaType: mfstInspect.MediaType,
	})

	return blobMountRequests, manifestRequests
}

func getHTTPClient(ctx context.Context, dockerCli *command.DockerCli, repoInfo *registry.RepositoryInfo, endpoint registry.APIEndpoint, repoName string) (*http.Client, error) {
	// get the http transport, this will be used in a client to upload manifest
	// TODO - add separate function get client
	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     endpoint.TLSConfig,
		DisableKeepAlives:   true,
	}

	authConfig := command.ResolveAuthConfig(ctx, dockerCli, repoInfo.Index)
	modifiers := registry.DockerHeaders(dockerversion.DockerUserAgent(nil), http.Header{})
	authTransport := transport.NewTransport(base, modifiers...)
	challengeManager, _, err := registry.PingV2Registry(endpoint.URL, authTransport)
	if err != nil {
		return nil, fmt.Errorf("Ping of V2 registry failed: %v", err)
	}
	if authConfig.RegistryToken != "" {
		passThruTokenHandler := &existingTokenHandler{token: authConfig.RegistryToken}
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, passThruTokenHandler))
	} else {
		creds := registry.NewStaticCredentialStore(&authConfig)
		tokenHandler := auth.NewTokenHandler(authTransport, creds, repoName, "*")
		basicHandler := auth.NewBasicHandler(creds)
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler))
	}
	tr := transport.NewTransport(base, modifiers...)

	httpClient := &http.Client{
		Transport: tr,
		// @TODO: Use the default (leave CheckRedirect nil), or write a generic one
		// and put it somewhere? (There's one in docker/distribution/registry/client/repository.go)
		// CheckRedirect: checkHTTPRedirect,
	}
	return httpClient, nil
}

func createManifestURLFromRef(targetRef reference.Named, urlBuilder *v2.URLBuilder) (string, error) {

	manifestURL, err := urlBuilder.BuildManifestURL(targetRef)
	if err != nil {
		return "", fmt.Errorf("Failed to build manifest URL from target reference: %v", err)
	}
	return manifestURL, nil
}

func setupRepo(repoInfo *registry.RepositoryInfo) (registry.APIEndpoint, string, error) {
	endpoint, err := selectPushEndpoint(repoInfo)
	if err != nil {
		return endpoint, "", err
	}
	logrus.Debugf("manifest: create: endpoint: %v", endpoint)
	repoName := repoInfo.Name.Name()
	// If endpoint does not support CanonicalName, use the RemoteName instead
	if endpoint.TrimHostname {
		repoName = reference.Path(repoInfo.Name)
		logrus.Debugf("repoName: %v", repoName)
	}
	return endpoint, repoName, nil
}

func selectPushEndpoint(repoInfo *registry.RepositoryInfo) (registry.APIEndpoint, error) {
	var err error

	options := registry.ServiceOptions{}
	// By default (unless deprecated), loopback (IPv4 at least...) is automatically added as an insecure registry.
	options.InsecureRegistries, err = loadLocalInsecureRegistries()
	if err != nil {
		return registry.APIEndpoint{}, err
	}
	registryService := registry.NewService(options)
	endpoints, err := registryService.LookupPushEndpoints(reference.Domain(repoInfo.Name))
	if err != nil {
		return registry.APIEndpoint{}, err
	}
	logrus.Debugf("manifest: potential push endpoints: %v\n", endpoints)
	// Default to the highest priority endpoint to return
	endpoint := endpoints[0]
	if !repoInfo.Index.Secure {
		for _, ep := range endpoints {
			if ep.URL.Scheme == "http" {
				endpoint = ep
			}
		}
	}
	return endpoint, nil
}

func loadLocalInsecureRegistries() ([]string, error) {
	insecureRegistries := []string{}
	// Check $HOME/.docker/config.json. There may be mismatches between what the user has in their
	// local config and what the daemon they're talking to allows, but we can be okay with that.
	userHome, err := homedir.GetStatic()
	if err != nil {
		return []string{}, fmt.Errorf("Manifest create: lookup local insecure registries: Unable to retreive $HOME")
	}

	jsonData, err := ioutil.ReadFile(fmt.Sprintf("%s/.docker/config.json", userHome))
	if err != nil {
		if !os.IsNotExist(err) {
			return []string{}, fmt.Errorf("Manifest create: Unable to read $HOME/.docker/config.json: %s", err)
		}
		// If the file just doesn't exist, no insecure registries were specified.
		logrus.Debug("Manifest: No insecure registries were specified via $HOME/.docker/config.json")
		return []string{}, nil
	}

	if jsonData != nil {
		cf := configfile.ConfigFile{}
		if err := json.Unmarshal(jsonData, &cf); err != nil {
			logrus.Debugf("Manifest create: Unable to unmarshal insecure registries from $HOME/.docker/config.json: %s", err)
			return []string{}, nil
		}
		if cf.InsecureRegistries == nil {
			return []string{}, nil
		}
		// @TODO: Add tests for a) specifying in config.json, b) invalid entries
		for _, reg := range cf.InsecureRegistries {
			if err := net.ParseIP(reg); err == nil {
				insecureRegistries = append(insecureRegistries, reg)
			} else if _, _, err := net.ParseCIDR(reg); err == nil {
				insecureRegistries = append(insecureRegistries, reg)
			} else if ips, err := net.LookupHost(reg); err == nil {
				insecureRegistries = append(insecureRegistries, ips...)
			} else {
				return []string{}, fmt.Errorf("Manifest create: Invalid registry (%s) specified in ~/.docker/config.json: %s", reg, err)
			}
		}
	}

	return insecureRegistries, nil
}

func pushReferences(httpClient *http.Client, urlBuilder *v2.URLBuilder, ref reference.Named, manifests []manifestPush) error {
	for _, manifest := range manifests {
		dgst, err := digest.Parse(manifest.Digest)
		logrus.Debugf("pushing ref digest %s", dgst)
		if err != nil {
			return fmt.Errorf("Error parsing manifest digest (%s) for referenced manifest %q: %v", manifest.Digest, manifest.Name, err)
		}
		targetRef, err := reference.WithDigest(ref, dgst)
		logrus.Debugf("pushing ref %v", targetRef)
		if err != nil {
			return fmt.Errorf("Error creating manifest digest target for referenced manifest %q: %v", manifest.Name, err)
		}
		pushURL, err := urlBuilder.BuildManifestURL(targetRef)
		if err != nil {
			return fmt.Errorf("Error setting up manifest push URL for manifest references for %q: %v", manifest.Name, err)
		}
		logrus.Debugf("manifest reference push URL: %s", pushURL)

		pushRequest, err := http.NewRequest("PUT", pushURL, bytes.NewReader(manifest.JSONBytes))
		if err != nil {
			return fmt.Errorf("HTTP PUT request creation for manifest reference push failed: %v", err)
		}
		pushRequest.Header.Set("Content-Type", manifest.MediaType)
		resp, err := httpClient.Do(pushRequest)
		if err != nil {
			return fmt.Errorf("PUT of manifest reference failed: %v", err)
		}

		resp.Body.Close()
		if !statusSuccess(resp.StatusCode) {
			return fmt.Errorf("Referenced manifest push unsuccessful: response %d: %s", resp.StatusCode, resp.Status)
		}
		dgstHeader := resp.Header.Get("Docker-Content-Digest")
		dgstResult, err := digest.Parse(dgstHeader)
		if err != nil {
			return fmt.Errorf("Couldn't parse pushed manifest digest response: %v", err)
		}
		if string(dgstResult) != manifest.Digest {
			return fmt.Errorf("Pushed referenced manifest received a different digest: expected %s, got %s", manifest.Digest, string(dgst))
		}
		logrus.Debugf("referenced manifest %q pushed; digest matches: %s", manifest.Name, string(dgst))
	}
	return nil
}

func mountBlobs(httpClient *http.Client, urlBuilder *v2.URLBuilder, ref reference.Named, blobsRequested []blobMount) error {

	for _, blob := range blobsRequested {
		// create URL request
		url, err := urlBuilder.BuildBlobUploadURL(ref, url.Values{"from": {blob.FromRepo}, "mount": {blob.Digest}})
		if err != nil {
			return fmt.Errorf("Failed to create blob mount URL: %v", err)
		}
		mountRequest, err := http.NewRequest("POST", url, nil)
		if err != nil {
			return fmt.Errorf("HTTP POST request creation for blob mount failed: %v", err)
		}
		mountRequest.Header.Set("Content-Length", "0")
		resp, err := httpClient.Do(mountRequest)
		if err != nil {
			return fmt.Errorf("V2 registry POST of blob mount failed: %v", err)
		}

		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("Blob mount failed to url %s: HTTP status %d", url, resp.StatusCode)
		}
		logrus.Debugf("Mount of blob %s succeeded, location: %q", blob.Digest, resp.Header.Get("Location"))
	}
	return nil
}

func statusSuccess(status int) bool {
	return status >= 200 && status <= 399
}
