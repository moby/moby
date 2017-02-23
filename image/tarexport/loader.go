package tarexport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/pkg/progress"
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
	indexJSON, err := os.Open(filepath.Join(ol.tmpDir, "index.json"))
	if err != nil {
		return err
	}
	defer indexJSON.Close()
	index := ociv1.ImageIndex{}
	if err := json.NewDecoder(indexJSON).Decode(&index); err != nil {
		return err
	}
	for _, md := range index.Manifests {
		if md.MediaType != ociv1.MediaTypeImageManifest {
			continue
		}
		d := digest.Digest(md.Digest)
		manifestPath := filepath.Join(ol.tmpDir, "blobs", d.Algorithm().String(), d.Hex())
		f, err := os.Open(manifestPath)
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
		refName, ok := md.Annotations["org.opencontainers.ref.name"]
		if !ok {
			return fmt.Errorf("no ref name annotation")
		}
		if ol.tr.name != "" {
			named, err := reference.ParseNormalizedNamed(ol.tr.name)
			if err != nil {
				return err
			}
			withTag, err := reference.WithTag(named, refName)
			if err != nil {
				return err
			}
			tag = reference.FamiliarString(withTag)
		} else {
			_, rs, err := ol.tr.getRefs()
			if err != nil {
				return err
			}
			r, ok := rs[refName]
			if !ok {
				return fmt.Errorf("no naming provided for %q", refName)
			}
			tag = reference.FamiliarString(r)
		}
		configDigest := digest.Digest(man.Config.Digest)
		manifests = append(manifests, manifestItem{
			Config:   filepath.Join("blobs", configDigest.Algorithm().String(), configDigest.Hex()),
			RepoTags: []string{tag},
			Layers:   layers,
			// TODO(runcom): foreign srcs?
			// See https://github.com/docker/docker/pull/22866/files#r96125181
		})
	}

	return ol.tr.loadHelper(manifests, ol.outStream, ol.progressOutput, ol.tmpDir)
}
