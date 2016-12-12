// +build experimental

package distribution

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	dockerdist "github.com/docker/docker/distribution"
	archive "github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

// PullData is the plugin manifest and the rootfs
type PullData interface {
	Config() ([]byte, error)
	Layer() (io.ReadCloser, error)
}

type pullData struct {
	repository distribution.Repository
	manifest   schema2.Manifest
	index      int
}

func (pd *pullData) Config() ([]byte, error) {
	blobs := pd.repository.Blobs(context.Background())
	config, err := blobs.Get(context.Background(), pd.manifest.Config.Digest)
	if err != nil {
		return nil, err
	}
	// validate
	var p types.Plugin
	if err := json.Unmarshal(config, &p); err != nil {
		return nil, err
	}
	return config, nil
}

func (pd *pullData) Layer() (io.ReadCloser, error) {
	if pd.index >= len(pd.manifest.Layers) {
		return nil, io.EOF
	}

	blobs := pd.repository.Blobs(context.Background())
	rsc, err := blobs.Open(context.Background(), pd.manifest.Layers[pd.index].Digest)
	if err != nil {
		return nil, err
	}
	pd.index++
	return rsc, nil
}

// Pull downloads the plugin from Store
func Pull(name string, rs registry.Service, metaheader http.Header, authConfig *types.AuthConfig) (PullData, error) {
	ref, err := reference.ParseNamed(name)
	if err != nil {
		logrus.Debugf("pull.go: error in ParseNamed: %v", err)
		return nil, err
	}

	repoInfo, err := rs.ResolveRepository(ref)
	if err != nil {
		logrus.Debugf("pull.go: error in ResolveRepository: %v", err)
		return nil, err
	}

	if err := dockerdist.ValidateRepoName(repoInfo.Name()); err != nil {
		logrus.Debugf("pull.go: error in ValidateRepoName: %v", err)
		return nil, err
	}

	endpoints, err := rs.LookupPullEndpoints(repoInfo.Hostname())
	if err != nil {
		logrus.Debugf("pull.go: error in LookupPullEndpoints: %v", err)
		return nil, err
	}

	var confirmedV2 bool
	var repository distribution.Repository

	for _, endpoint := range endpoints {
		if confirmedV2 && endpoint.Version == registry.APIVersion1 {
			logrus.Debugf("Skipping v1 endpoint %s because v2 registry was detected", endpoint.URL)
			continue
		}

		// TODO: reuse contexts
		repository, confirmedV2, err = dockerdist.NewV2Repository(context.Background(), repoInfo, endpoint, metaheader, authConfig, "pull")
		if err != nil {
			logrus.Debugf("pull.go: error in NewV2Repository: %v", err)
			return nil, err
		}
		if !confirmedV2 {
			logrus.Debugf("pull.go: !confirmedV2")
			return nil, ErrUnsupportedRegistry
		}
		logrus.Debugf("Trying to pull %s from %s %s", repoInfo.Name(), endpoint.URL, endpoint.Version)
		break
	}

	tag := DefaultTag
	if ref, ok := ref.(reference.NamedTagged); ok {
		tag = ref.Tag()
	}

	// tags := repository.Tags(context.Background())
	// desc, err := tags.Get(context.Background(), tag)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	//
	msv, err := repository.Manifests(context.Background())
	if err != nil {
		logrus.Debugf("pull.go: error in repository.Manifests: %v", err)
		return nil, err
	}
	manifest, err := msv.Get(context.Background(), "", distribution.WithTag(tag))
	if err != nil {
		// TODO: change 401 to 404
		logrus.Debugf("pull.go: error in msv.Get(): %v", err)
		return nil, err
	}

	_, pl, err := manifest.Payload()
	if err != nil {
		logrus.Debugf("pull.go: error in manifest.Payload(): %v", err)
		return nil, err
	}
	var m schema2.Manifest
	if err := json.Unmarshal(pl, &m); err != nil {
		logrus.Debugf("pull.go: error in json.Unmarshal(): %v", err)
		return nil, err
	}
	if m.Config.MediaType != MediaTypeConfig {
		return nil, ErrUnsupportedMediaType
	}

	pd := &pullData{
		repository: repository,
		manifest:   m,
	}

	logrus.Debugf("manifest: %s", pl)
	return pd, nil
}

// WritePullData extracts manifest and rootfs to the disk.
func WritePullData(pd PullData, dest string, extract bool) error {
	config, err := pd.Config()
	if err != nil {
		return err
	}
	var p types.Plugin
	if err := json.Unmarshal(config, &p); err != nil {
		return err
	}
	logrus.Debugf("%#v", p)

	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}

	if extract {
		if err := ioutil.WriteFile(filepath.Join(dest, "manifest.json"), config, 0600); err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Join(dest, "rootfs"), 0700); err != nil {
			return err
		}
	}

	for i := 0; ; i++ {
		l, err := pd.Layer()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if !extract {
			f, err := os.Create(filepath.Join(dest, fmt.Sprintf("layer%d.tar", i)))
			if err != nil {
				l.Close()
				return err
			}
			io.Copy(f, l)
			l.Close()
			f.Close()
			continue
		}

		if _, err := archive.ApplyLayer(filepath.Join(dest, "rootfs"), l); err != nil {
			return err
		}

	}
	return nil
}
