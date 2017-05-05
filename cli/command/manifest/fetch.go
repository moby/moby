package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/go-digest"
	"golang.org/x/net/context"

	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/docker/api/types"
	//"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/distribution"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/registry"
	//"github.com/spf13/cobra"
	//	"github.com/spf13/pflag"
)

type fetchOptions struct {
	remote string
}

type manifestFetcher interface {
	Fetch(ctx context.Context, ref reference.Named) ([]ImgManifestInspect, error)
}

func loadManifest(manifest string, transaction string) ([]ImgManifestInspect, error) {

	// Load either a single manifest (if transaction is "", that's fine), or a
	// manifest list
	var foundImages []ImgManifestInspect
	fd, err := getManifestFd(manifest, transaction)
	if err != nil {
		return nil, err
	}
	if fd != nil {
		defer fd.Close()
		fileInfo, err := fd.Stat()
		if err != nil {
			return nil, err
		}
		if fileInfo.IsDir() { // manifest list transaction
			logrus.Errorf("Not supported: loading manifest list directory")
			return nil, fmt.Errorf("Not supported: loading manifest list directory")
		}
		// An individual manifest
		mfInspect, err := unmarshalIntoManifestInspect(manifest, transaction)
		if err != nil {
			return nil, err
		}
		return append(foundImages, mfInspect), nil
	}
	return nil, nil
}

func storeManifest(imgInspect ImgManifestInspect, name, transaction string) error {
	// Store this image manifest so that it can be annotated.
	// Store the manifests in a user's home to prevent conflict.
	manifestBase, err := buildBaseFilename()
	transaction = makeFilesafeName(transaction)
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Join(manifestBase, transaction), 0755)
	logrus.Debugf("Storing  %s", name)
	if err = updateMfFile(imgInspect, name, transaction); err != nil {
		fmt.Printf("Error writing local manifest copy: %s\n", err)
		return err
	}

	return nil
}

func getImageData(dockerCli *command.DockerCli, name string, transactionID string, fetchOnly bool) ([]ImgManifestInspect, *registry.RepositoryInfo, error) {

	var (
		lastErr                    error
		discardNoSupportErrors     bool
		foundImages                []ImgManifestInspect
		confirmedV2                bool
		confirmedTLSRegistries     = make(map[string]struct{})
		namedRef, transactionNamed reference.Named
		err                        error
		normalName                 string
	)

	if namedRef, err = reference.ParseNormalizedNamed(name); err != nil {
		return nil, nil, fmt.Errorf("Error parsing reference for %s: %s\n", name, err)
	}
	if transactionID != "" {
		if transactionNamed, err = reference.ParseNormalizedNamed(transactionID); err != nil {
			return nil, nil, fmt.Errorf("Error parsing reference for %s: %s\n", transactionID, err)
		}
		if _, isDigested := transactionNamed.(reference.Canonical); !isDigested {
			transactionNamed = reference.TagNameOnly(transactionNamed)
		}
		transactionID = makeFilesafeName(transactionNamed.String())
	}

	// Make sure these have a tag, as long as it's not a digest
	if _, isDigested := namedRef.(reference.Canonical); !isDigested {
		namedRef = reference.TagNameOnly(namedRef)
	}
	normalName = namedRef.String()
	logrus.Debugf("getting image data for ref: %s", normalName)

	// Resolve the Repository name from fqn to RepositoryInfo
	// This calls TrimNamed, which removes the tag, so always use namedRef for the image.
	repoInfo, err := registry.ParseRepositoryInfo(namedRef)
	if err != nil {
		return nil, nil, err
	}

	// First check to see if stored locally, either a single manfiest or list:
	if !fetchOnly {
		logrus.Debugf("Checking locally for %s", normalName)
		foundImages, err = loadManifest(makeFilesafeName(normalName), transactionID)
		if err != nil {
			return nil, nil, err
		}
	}
	// Great, no reason to pull from the registry.
	if len(foundImages) > 0 {
		return foundImages, repoInfo, nil
	}

	ctx := context.Background()

	authConfig := command.ResolveAuthConfig(ctx, dockerCli, repoInfo.Index)

	options := registry.ServiceOptions{}
	registryService := registry.NewService(options)

	// a list of registry.APIEndpoint, which could be mirrors, etc., of locally-configured
	// repo endpoints. The list will be ordered by priority (v2, https, v1).
	endpoints, err := registryService.LookupPullEndpoints(reference.Domain(repoInfo.Name))
	if err != nil {
		return nil, nil, err
	}
	logrus.Debugf("manifest pull: endpoints: %v", endpoints)

	// Try to find the first endpoint that is *both* v2 and using TLS.
	for _, endpoint := range endpoints {
		// make sure I can reach the registry, same as docker pull does
		v1endpoint, err := endpoint.ToV1Endpoint(dockerversion.DockerUserAgent(nil), nil)
		if err != nil {
			return nil, nil, err
		}
		if _, err := v1endpoint.Ping(); err != nil {
			if strings.Contains(err.Error(), "timeout") {
				return nil, nil, err
			}
			continue
		}

		if confirmedV2 && endpoint.Version == registry.APIVersion1 {
			logrus.Debugf("Skipping v1 endpoint %s because v2 registry was detected", endpoint.URL)
			continue
		}

		if endpoint.URL.Scheme != "https" {
			if _, confirmedTLS := confirmedTLSRegistries[endpoint.URL.Host]; confirmedTLS {
				logrus.Debugf("Skipping non-TLS endpoint %s for host/port that appears to use TLS", endpoint.URL)
				continue
			}
		}

		logrus.Debugf("Trying to fetch image manifest of %s repository from %s %s", normalName, endpoint.URL, endpoint.Version)

		fetcher, err := newManifestFetcher(endpoint, repoInfo, authConfig, registryService)
		if err != nil {
			lastErr = err
			continue
		}

		if foundImages, err = fetcher.Fetch(ctx, namedRef); err != nil {
			// Was this fetch cancelled? If so, don't try to fall back.
			fallback := false
			select {
			case <-ctx.Done():
			default:
				if fallbackErr, ok := err.(fallbackError); ok {
					fallback = true
					confirmedV2 = confirmedV2 || fallbackErr.confirmedV2
					if fallbackErr.transportOK && endpoint.URL.Scheme == "https" {
						confirmedTLSRegistries[endpoint.URL.Host] = struct{}{}
					}
					err = fallbackErr.err
				}
			}
			if fallback {
				if _, ok := err.(distribution.ErrNoSupport); !ok {
					// Because we found an error that's not ErrNoSupport, discard all subsequent ErrNoSupport errors.
					discardNoSupportErrors = true
					// save the current error
					lastErr = err
				} else if !discardNoSupportErrors {
					// Save the ErrNoSupport error, because it's either the first error or all encountered errors
					// were also ErrNoSupport errors.
					lastErr = err
				}
				continue
			}
			logrus.Errorf("Not continuing with pull after error: %v", err)
			return nil, nil, err
		}

		if transactionID == "" && len(foundImages) > 1 {
			transactionID = normalName
		}
		// Additionally, we're never storing on inspect, so if we're asked to save images it's for a create,
		// and this function will have been called for each image in the create. In that case we'll have an
		// image name *and* a transaction ID. IOW, foundImages will be only one image.
		if !fetchOnly {
			if err := storeManifest(foundImages[0], makeFilesafeName(normalName), transactionID); err != nil {
				logrus.Errorf("Error storing manifests: %s\n", err)
			}
		}
		return foundImages, repoInfo, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s\n", normalName)
	}

	return nil, nil, lastErr

}

func newManifestFetcher(endpoint registry.APIEndpoint, repoInfo *registry.RepositoryInfo, authConfig types.AuthConfig, registryService registry.Service) (manifestFetcher, error) {
	switch endpoint.Version {
	case registry.APIVersion2:
		return &v2ManifestFetcher{
			endpoint:   endpoint,
			authConfig: authConfig,
			service:    registryService,
			repoInfo:   repoInfo,
		}, nil
	case registry.APIVersion1:
		return &v1ManifestFetcher{
			endpoint:   endpoint,
			authConfig: authConfig,
			service:    registryService,
			repoInfo:   repoInfo,
		}, nil
	}
	return nil, fmt.Errorf("unknown version %d for registry %s", endpoint.Version, endpoint.URL)
}

func makeImgManifestInspect(name string, img *image.Image, tag string, mfInfo manifestInfo, mediaType string, tagList []string) *ImgManifestInspect {
	var digest digest.Digest
	if err := mfInfo.digest.Validate(); err == nil {
		digest = mfInfo.digest
	}

	if mediaType == manifestlist.MediaTypeManifestList {
		return &ImgManifestInspect{
			MediaType: mediaType,
			Digest:    digest,
		}
	}

	var digests []string
	for _, blobDigest := range mfInfo.blobDigests {
		digests = append(digests, blobDigest.String())
	}
	return &ImgManifestInspect{
		RefName:         name,
		Size:            mfInfo.length,
		MediaType:       mediaType,
		Tag:             tag,
		Digest:          digest,
		RepoTags:        tagList,
		Comment:         img.Comment,
		Created:         img.Created.Format(time.RFC3339Nano),
		ContainerConfig: &img.ContainerConfig,
		DockerVersion:   img.DockerVersion,
		Author:          img.Author,
		Config:          img.Config,
		Architecture:    mfInfo.platform.Architecture,
		OS:              mfInfo.platform.OS,
		OSVersion:       mfInfo.platform.OSVersion,
		OSFeatures:      mfInfo.platform.OSFeatures,
		Variant:         mfInfo.platform.Variant,
		Features:        mfInfo.platform.Features,
		References:      digests,
		LayerDigests:    mfInfo.layers,
		CanonicalJSON:   mfInfo.jsonBytes,
	}
}

func continueOnError(err error) bool {
	switch v := err.(type) {
	case errcode.Errors:
		if len(v) == 0 {
			return true
		}
		return continueOnError(v[0])
	case distribution.ErrNoSupport:
		return continueOnError(v.Err)
	case errcode.Error:
		return shouldV2Fallback(v)
	case *client.UnexpectedHTTPResponseError:
		return true
	case ImageConfigPullError:
		return false
	case error:
		return !strings.Contains(err.Error(), strings.ToLower(syscall.ENOSPC.Error()))
	}
	// let's be nice and fallback if the error is a completely
	// unexpected one.
	// If new errors have to be handled in some way, please
	// add them to the switch above.
	return true
}
