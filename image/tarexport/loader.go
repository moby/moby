package tarexport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/progress"
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

type v0TarLoader struct {
	tr             *tarexporter
	tmpDir         string
	outStream      io.Writer
	progressOutput progress.Output
}

func (dll *v0TarLoader) Load() error {
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
		return errors.New("cannot load with both name and refs")
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
	index := ociv1.Index{}
	if err := json.NewDecoder(indexJSON).Decode(&index); err != nil {
		return err
	}
	for _, md := range index.Manifests {
		if md.MediaType != ociv1.MediaTypeImageManifest {
			continue
		}
		d := md.Digest
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
		foreignSrc := make(map[layer.DiffID]distribution.Descriptor)
		for i, l := range man.Layers {
			layerDigest := l.Digest
			layers[i] = filepath.Join("blobs", layerDigest.Algorithm().String(), layerDigest.Hex())
			if len(l.URLs) != 0 {
				foreignSrc[layer.DiffID(layerDigest)] = distribution.Descriptor{
					Digest: l.Digest,
					Size:   l.Size,
					URLs:   l.URLs,
				}
			}
		}
		var tag string
		refName, ok := md.Annotations[ociv1.AnnotationRefName]
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
		configDigest := man.Config.Digest
		manifests = append(manifests, manifestItem{
			Config:       filepath.Join("blobs", configDigest.Algorithm().String(), configDigest.Hex()),
			RepoTags:     []string{tag},
			Layers:       layers,
			LayerSources: foreignSrc,
		})
	}

	return ol.tr.loadHelper(manifests, ol.outStream, ol.progressOutput, ol.tmpDir)
}
