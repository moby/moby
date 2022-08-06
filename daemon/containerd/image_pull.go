package containerd

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"github.com/containerd/imgcrypt"
	"github.com/containerd/imgcrypt/images/encryption"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/platforms"
	encconfig "github.com/containers/ocicrypt/config"
	cryptUtils "github.com/containers/ocicrypt/utils"
	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// PullImage initiates a pull operation. image is the repository name to pull, and
// tagOrDigest may be either empty, or indicate a specific tag or digest to pull.
func (i *ImageService) PullImage(ctx context.Context, image, tagOrDigest string, platform *specs.Platform, metaHeaders map[string][]string, authConfig *registry.AuthConfig, outStream io.Writer) error {
	var opts []containerd.RemoteOpt
	if platform != nil {
		opts = append(opts, containerd.WithPlatform(platforms.Format(*platform)))
	}
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return errdefs.InvalidParameter(err)
	}

	// TODO(thaJeztah) this could use a WithTagOrDigest() utility
	if tagOrDigest != "" {
		// The "tag" could actually be a digest.
		var dgst digest.Digest
		dgst, err = digest.Parse(tagOrDigest)
		if err == nil {
			ref, err = reference.WithDigest(reference.TrimNamed(ref), dgst)
		} else {
			ref, err = reference.WithTag(ref, tagOrDigest)
		}
		if err != nil {
			return errdefs.InvalidParameter(err)
		}
	}
	decryptKeyPath := "/etc/docker/ocicrypt/keys"
	_, err = os.Stat(decryptKeyPath)
	if err == nil {
		imgcryptPayload := imgcrypt.Payload{}
		keyPathCc, err := getDecryptionKeys(decryptKeyPath)
		if err != nil {
			return err
		}
		imgcryptPayload.DecryptConfig = *keyPathCc.DecryptConfig
		imgcryptUnpackOpt := encryption.WithUnpackConfigApplyOpts(encryption.WithDecryptedUnpack(&imgcryptPayload))
		opts = append(opts, containerd.WithPullUnpack, containerd.WithUnpackOpts([]containerd.UnpackOpt{imgcryptUnpackOpt}))
	}

	_, err = i.client.Pull(ctx, ref.String(), opts...)
	return err
}

// GetRepository returns a repository from the registry.
func (i *ImageService) GetRepository(ctx context.Context, ref reference.Named, authConfig *registry.AuthConfig) (distribution.Repository, error) {
	panic("not implemented")
}

func getDecryptionKeys(keysPath string) (encconfig.CryptoConfig, error) {
	var cc encconfig.CryptoConfig
	base64Keys := make([]string, 0)
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Handle symlinks
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			return fmt.Errorf("Symbolic links not supported in decryption keys paths")
		}
		privateKey, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		// TODO - Remove the need to covert to base64. The ocicrypt library
		// should provide a method to directly process the private keys
		sEnc := b64.StdEncoding.EncodeToString(privateKey)
		base64Keys = append(base64Keys, sEnc)
		return nil
	}
	err := filepath.Walk(keysPath, walkFn)
	if err != nil {
		return cc, err
	}
	sortedDc, err := cryptUtils.SortDecryptionKeys(strings.Join(base64Keys, ","))
	if err != nil {
		return cc, err
	}
	cc = encconfig.InitDecryption(sortedDc)
	return cc, nil
}