package daemonbuilder

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/daemon"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/runconfig"
)

// Docker implements builder.Backend for the docker Daemon object.
type Docker struct {
	*daemon.Daemon
	OutOld      io.Writer
	AuthConfigs map[string]types.AuthConfig
	Archiver    *archive.Archiver
}

// ensure Docker implements builder.Backend
var _ builder.Backend = Docker{}

// Pull tells Docker to pull image referenced by `name`.
func (d Docker) Pull(name string) (*image.Image, error) {
	ref, err := reference.ParseNamed(name)
	if err != nil {
		return nil, err
	}
	ref = reference.WithDefaultTag(ref)

	pullRegistryAuth := &types.AuthConfig{}
	if len(d.AuthConfigs) > 0 {
		// The request came with a full auth config file, we prefer to use that
		repoInfo, err := d.Daemon.RegistryService.ResolveRepository(ref)
		if err != nil {
			return nil, err
		}

		resolvedConfig := registry.ResolveAuthConfig(
			d.AuthConfigs,
			repoInfo.Index,
		)
		pullRegistryAuth = &resolvedConfig
	}

	if err := d.Daemon.PullImage(ref, nil, pullRegistryAuth, ioutils.NopWriteCloser(d.OutOld)); err != nil {
		return nil, err
	}

	return d.Daemon.GetImage(name)
}

// ContainerUpdateCmd updates Path and Args for the container with ID cID.
func (d Docker) ContainerUpdateCmd(cID string, cmd []string) error {
	c, err := d.Daemon.GetContainer(cID)
	if err != nil {
		return err
	}
	c.Path = cmd[0]
	c.Args = cmd[1:]
	return nil
}

// Retain retains an image avoiding it to be removed or overwritten until a corresponding Release() call.
func (d Docker) Retain(sessionID, imgID string) {
	// FIXME: This will be solved with tags in client-side builder
	//d.Daemon.Graph().Retain(sessionID, imgID)
}

// Release releases a list of images that were retained for the time of a build.
func (d Docker) Release(sessionID string, activeImages []string) {
	// FIXME: This will be solved with tags in client-side builder
	//d.Daemon.Graph().Release(sessionID, activeImages...)
}

// BuilderCopy copies/extracts a source FileInfo to a destination path inside a container
// specified by a container object.
// TODO: make sure callers don't unnecessarily convert destPath with filepath.FromSlash (Copy does it already).
// BuilderCopy should take in abstract paths (with slashes) and the implementation should convert it to OS-specific paths.
func (d Docker) BuilderCopy(cID string, destPath string, src builder.FileInfo, decompress bool) error {
	srcPath := src.Path()
	destExists := true
	rootUID, rootGID := d.Daemon.GetRemappedUIDGID()

	// Work in daemon-local OS specific file paths
	destPath = filepath.FromSlash(destPath)

	c, err := d.Daemon.GetContainer(cID)
	if err != nil {
		return err
	}
	dest, err := c.GetResourcePath(destPath)
	if err != nil {
		return err
	}

	// Preserve the trailing slash
	// TODO: why are we appending another path separator if there was already one?
	if strings.HasSuffix(destPath, string(os.PathSeparator)) || destPath == "." {
		dest += string(os.PathSeparator)
	}

	destPath = dest

	destStat, err := os.Stat(destPath)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Errorf("Error performing os.Stat on %s. %s", destPath, err)
			return err
		}
		destExists = false
	}

	if src.IsDir() {
		// copy as directory
		if err := d.Archiver.CopyWithTar(srcPath, destPath); err != nil {
			return err
		}
		return fixPermissions(srcPath, destPath, rootUID, rootGID, destExists)
	}
	if decompress && archive.IsArchivePath(srcPath) {
		// Only try to untar if it is a file and that we've been told to decompress (when ADD-ing a remote file)

		// First try to unpack the source as an archive
		// to support the untar feature we need to clean up the path a little bit
		// because tar is very forgiving.  First we need to strip off the archive's
		// filename from the path but this is only added if it does not end in slash
		tarDest := destPath
		if strings.HasSuffix(tarDest, string(os.PathSeparator)) {
			tarDest = filepath.Dir(destPath)
		}

		// try to successfully untar the orig
		err := d.Archiver.UntarPath(srcPath, tarDest)
		if err != nil {
			logrus.Errorf("Couldn't untar to %s: %v", tarDest, err)
		}
		return err
	}

	// only needed for fixPermissions, but might as well put it before CopyFileWithTar
	if destExists && destStat.IsDir() {
		destPath = filepath.Join(destPath, src.Name())
	}

	if err := idtools.MkdirAllNewAs(filepath.Dir(destPath), 0755, rootUID, rootGID); err != nil {
		return err
	}
	if err := d.Archiver.CopyFileWithTar(srcPath, destPath); err != nil {
		return err
	}

	return fixPermissions(srcPath, destPath, rootUID, rootGID, destExists)
}

// Mount mounts the root filesystem for the container.
func (d Docker) Mount(cID string) error {
	c, err := d.Daemon.GetContainer(cID)
	if err != nil {
		return err
	}
	return d.Daemon.Mount(c)
}

// Unmount unmounts the root filesystem for the container.
func (d Docker) Unmount(cID string) error {
	c, err := d.Daemon.GetContainer(cID)
	if err != nil {
		return err
	}
	d.Daemon.Unmount(c)
	return nil
}

// GetCachedImage returns a reference to a cached image whose parent equals `parent`
// and runconfig equals `cfg`. A cache miss is expected to return an empty ID and a nil error.
func (d Docker) GetCachedImage(imgID string, cfg *runconfig.Config) (string, error) {
	cache, err := d.Daemon.ImageGetCached(image.ID(imgID), cfg)
	if cache == nil || err != nil {
		return "", err
	}
	return cache.ID().String(), nil
}

// Following is specific to builder contexts

// DetectContextFromRemoteURL returns a context and in certain cases the name of the dockerfile to be used
// irrespective of user input.
// progressReader is only used if remoteURL is actually a URL (not empty, and not a Git endpoint).
func DetectContextFromRemoteURL(r io.ReadCloser, remoteURL string, createProgressReader func(in io.ReadCloser) io.ReadCloser) (context builder.ModifiableContext, dockerfileName string, err error) {
	switch {
	case remoteURL == "":
		context, err = builder.MakeTarSumContext(r)
	case urlutil.IsGitURL(remoteURL):
		context, err = builder.MakeGitContext(remoteURL)
	case urlutil.IsURL(remoteURL):
		context, err = builder.MakeRemoteContext(remoteURL, map[string]func(io.ReadCloser) (io.ReadCloser, error){
			httputils.MimeTypes.TextPlain: func(rc io.ReadCloser) (io.ReadCloser, error) {
				dockerfile, err := ioutil.ReadAll(rc)
				if err != nil {
					return nil, err
				}

				// dockerfileName is set to signal that the remote was interpreted as a single Dockerfile, in which case the caller
				// should use dockerfileName as the new name for the Dockerfile, irrespective of any other user input.
				dockerfileName = api.DefaultDockerfileName

				// TODO: return a context without tarsum
				return archive.Generate(dockerfileName, string(dockerfile))
			},
			// fallback handler (tar context)
			"": func(rc io.ReadCloser) (io.ReadCloser, error) {
				return createProgressReader(rc), nil
			},
		})
	default:
		err = fmt.Errorf("remoteURL (%s) could not be recognized as URL", remoteURL)
	}
	return
}
