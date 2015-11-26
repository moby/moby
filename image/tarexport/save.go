package tarexport

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/image"
	"github.com/docker/docker/image/v1"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/tag"
)

type imageDescriptor struct {
	refs   []reference.NamedTagged
	layers []string
}

type saveSession struct {
	*tarexporter
	outDir      string
	images      map[image.ID]*imageDescriptor
	savedLayers map[string]struct{}
}

func (l *tarexporter) Save(names []string, outStream io.Writer) error {
	images, err := l.parseNames(names)
	if err != nil {
		return err
	}

	return (&saveSession{tarexporter: l, images: images}).save(outStream)
}

func (l *tarexporter) parseNames(names []string) (map[image.ID]*imageDescriptor, error) {
	imgDescr := make(map[image.ID]*imageDescriptor)

	addAssoc := func(id image.ID, ref reference.Named) {
		if _, ok := imgDescr[id]; !ok {
			imgDescr[id] = &imageDescriptor{}
		}

		if ref != nil {
			var tagged reference.NamedTagged
			if _, ok := ref.(reference.Digested); ok {
				return
			}
			var ok bool
			if tagged, ok = ref.(reference.NamedTagged); !ok {
				var err error
				if tagged, err = reference.WithTag(ref, tag.DefaultTag); err != nil {
					return
				}
			}

			for _, t := range imgDescr[id].refs {
				if tagged.String() == t.String() {
					return
				}
			}
			imgDescr[id].refs = append(imgDescr[id].refs, tagged)
		}
	}

	for _, name := range names {
		ref, err := reference.ParseNamed(name)
		if err != nil {
			return nil, err
		}
		ref = registry.NormalizeLocalReference(ref)
		if ref.Name() == string(digest.Canonical) {
			imgID, err := l.is.Search(name)
			if err != nil {
				return nil, err
			}
			addAssoc(imgID, nil)
			continue
		}
		if _, ok := ref.(reference.Digested); !ok {
			if _, ok := ref.(reference.NamedTagged); !ok {
				assocs := l.ts.ReferencesByName(ref)
				for _, assoc := range assocs {
					addAssoc(assoc.ImageID, assoc.Ref)
				}
				if len(assocs) == 0 {
					imgID, err := l.is.Search(name)
					if err != nil {
						return nil, err
					}
					addAssoc(imgID, nil)
				}
				continue
			}
		}
		var imgID image.ID
		if imgID, err = l.ts.Get(ref); err != nil {
			return nil, err
		}
		addAssoc(imgID, ref)

	}
	return imgDescr, nil
}

func (s *saveSession) save(outStream io.Writer) error {
	s.savedLayers = make(map[string]struct{})

	// get image json
	tempDir, err := ioutil.TempDir("", "docker-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	s.outDir = tempDir
	reposLegacy := make(map[string]map[string]string)

	var manifest []manifestItem

	for id, imageDescr := range s.images {
		if err = s.saveImage(id); err != nil {
			return err
		}

		var repoTags []string
		var layers []string

		for _, ref := range imageDescr.refs {
			if _, ok := reposLegacy[ref.Name()]; !ok {
				reposLegacy[ref.Name()] = make(map[string]string)
			}
			reposLegacy[ref.Name()][ref.Tag()] = imageDescr.layers[len(imageDescr.layers)-1]
			repoTags = append(repoTags, ref.String())
		}

		for _, l := range imageDescr.layers {
			layers = append(layers, filepath.Join(l, legacyLayerFileName))
		}

		manifest = append(manifest, manifestItem{
			Config:   digest.Digest(id).Hex() + ".json",
			RepoTags: repoTags,
			Layers:   layers,
		})
	}

	if len(reposLegacy) > 0 {
		reposFile := filepath.Join(tempDir, legacyRepositoriesFileName)
		f, err := os.OpenFile(reposFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			f.Close()
			return err
		}
		if err := json.NewEncoder(f).Encode(reposLegacy); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		if err := os.Chtimes(reposFile, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
			return err
		}
	}

	manifestFileName := filepath.Join(tempDir, manifestFileName)
	f, err := os.OpenFile(manifestFileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		f.Close()
		return err
	}
	if err := json.NewEncoder(f).Encode(manifest); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chtimes(manifestFileName, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
		return err
	}

	fs, err := archive.Tar(tempDir, archive.Uncompressed)
	if err != nil {
		return err
	}
	defer fs.Close()

	if _, err := io.Copy(outStream, fs); err != nil {
		return err
	}
	return nil
}

func (s *saveSession) saveImage(id image.ID) error {
	img, err := s.is.Get(id)
	if err != nil {
		return err
	}

	if len(img.RootFS.DiffIDs) == 0 {
		return fmt.Errorf("empty export - not implemented")
	}

	var parent digest.Digest
	var layers []string
	for i := range img.RootFS.DiffIDs {
		v1Img := image.V1Image{}
		if i == len(img.RootFS.DiffIDs)-1 {
			v1Img = img.V1Image
		}
		rootFS := *img.RootFS
		rootFS.DiffIDs = rootFS.DiffIDs[:i+1]
		v1ID, err := v1.CreateID(v1Img, rootFS.ChainID(), parent)
		if err != nil {
			return err
		}

		v1Img.ID = v1ID.Hex()
		if parent != "" {
			v1Img.Parent = parent.Hex()
		}

		if err := s.saveLayer(rootFS.ChainID(), v1Img, img.Created); err != nil {
			return err
		}
		layers = append(layers, v1Img.ID)
		parent = v1ID
	}

	configFile := filepath.Join(s.outDir, digest.Digest(id).Hex()+".json")
	if err := ioutil.WriteFile(configFile, img.RawJSON(), 0644); err != nil {
		return err
	}
	if err := os.Chtimes(configFile, img.Created, img.Created); err != nil {
		return err
	}

	s.images[id].layers = layers
	return nil
}

func (s *saveSession) saveLayer(id layer.ChainID, legacyImg image.V1Image, createdTime time.Time) error {
	if _, exists := s.savedLayers[legacyImg.ID]; exists {
		return nil
	}

	outDir := filepath.Join(s.outDir, legacyImg.ID)
	if err := os.Mkdir(outDir, 0755); err != nil {
		return err
	}

	// todo: why is this version file here?
	if err := ioutil.WriteFile(filepath.Join(outDir, legacyVersionFileName), []byte("1.0"), 0644); err != nil {
		return err
	}

	imageConfig, err := json.Marshal(legacyImg)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(outDir, legacyConfigFileName), imageConfig, 0644); err != nil {
		return err
	}

	// serialize filesystem
	tarFile, err := os.Create(filepath.Join(outDir, legacyLayerFileName))
	if err != nil {
		return err
	}
	defer tarFile.Close()

	l, err := s.ls.Get(id)
	if err != nil {
		return err
	}
	defer layer.ReleaseAndLog(s.ls, l)

	arch, err := l.TarStream()
	if err != nil {
		return err
	}
	defer arch.Close()

	if _, err := io.Copy(tarFile, arch); err != nil {
		return err
	}

	for _, fname := range []string{"", legacyVersionFileName, legacyConfigFileName, legacyLayerFileName} {
		// todo: maybe save layer created timestamp?
		if err := os.Chtimes(filepath.Join(outDir, fname), createdTime, createdTime); err != nil {
			return err
		}
	}

	s.savedLayers[legacyImg.ID] = struct{}{}
	return nil
}
