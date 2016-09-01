package tarexport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/reference"
	digest "github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type loader interface {
	Load() error
}

type dockerLoader struct {
	manifests      []manifestItem
	tr             *tarexporter
	tmpDir         string
	outStream      io.Writer
	progressOutput progress.Output
}

func (dl *dockerLoader) Load() error {
	return dl.tr.loadHelper(dl.manifests, dl.outStream, dl.progressOutput, dl.tmpDir)
}

type dockerLegacyLoader struct {
	tr             *tarexporter
	tmpDir         string
	outStream      io.Writer
	progressOutput progress.Output
}

func (dll *dockerLegacyLoader) Load() error {
	return dll.tr.legacyLoad(dll.tmpDir, dll.outStream, dll.progressOutput)
}

type ociLoader struct {
	tr             *tarexporter
	tmpDir         string
	outStream      io.Writer
	progressOutput progress.Output
}

func (ol *ociLoader) Load() error {
	if ol.tr.name != "" && len(ol.tr.refs) != 0 {
		return errors.New("cannot load with either name and refs")
	}
	if ol.tr.name == "" && len(ol.tr.refs) == 0 {
		return errors.New("no OCI name mapping provided")
	}

	// FIXME(runcom): validate and check version of "oci-layout" file

	manifests := []manifestItem{}
	refsPath := filepath.Join(ol.tmpDir, "refs")
	if err := filepath.Walk(refsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		descriptor := ociv1.Descriptor{}
		if err := json.NewDecoder(f).Decode(&descriptor); err != nil {
			return err
		}
		// TODO(runcom): validate mediatype and size
		d := digest.Digest(descriptor.Digest)
		manifestPath := filepath.Join(ol.tmpDir, "blobs", d.Algorithm().String(), d.Hex())
		f, err = os.Open(manifestPath)
		if err != nil {
			return err
		}
		defer f.Close()
		man := ociv1.Manifest{}
		if err := json.NewDecoder(f).Decode(&man); err != nil {
			return err
		}
		layers := make([]string, len(man.Layers))
		for i, l := range man.Layers {
			layerDigest := digest.Digest(l.Digest)
			layers[i] = filepath.Join("blobs", layerDigest.Algorithm().String(), layerDigest.Hex())
		}
		tag := ""
		if ol.tr.name != "" {
			named, err := reference.ParseNamed(ol.tr.name)
			if err != nil {
				return err
			}
			withTag, err := reference.WithTag(named, info.Name())
			if err != nil {
				return err
			}
			tag = withTag.String()
		} else {
			_, rs, err := ol.tr.getRefs()
			if err != nil {
				return err
			}
			r, ok := rs[info.Name()]
			if !ok {
				return fmt.Errorf("no naming provided for %q", info.Name())
			}
			tag = r.String()
		}
		configDigest := digest.Digest(man.Config.Digest)
		manifests = append(manifests, manifestItem{
			Config:   filepath.Join("blobs", configDigest.Algorithm().String(), configDigest.Hex()),
			RepoTags: []string{tag},
			Layers:   layers,
			// TODO(runcom): foreign srcs?
			// See https://github.com/docker/docker/pull/22866/files#r96125181
		})
		return nil
	}); err != nil {
		return err
	}

	return ol.tr.loadHelper(manifests, ol.outStream, ol.progressOutput, ol.tmpDir)
}
