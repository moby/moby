package tarexport // import "github.com/docker/docker/image/tarexport"

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/containerd/containerd/images"
	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/image"
	v1 "github.com/docker/docker/image/v1"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/system"
	"github.com/moby/sys/sequential"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type imageDescriptor struct {
	refs     []reference.NamedTagged
	layers   []digest.Digest
	image    *image.Image
	layerRef layer.Layer
}

type saveSession struct {
	*tarexporter
	outDir      string
	images      map[image.ID]*imageDescriptor
	savedLayers map[string]struct{}
	diffIDPaths map[layer.DiffID]string // cache every diffID blob to avoid duplicates
}

func (l *tarexporter) Save(names []string, outStream io.Writer) error {
	images, err := l.parseNames(names)
	if err != nil {
		return err
	}

	// Release all the image top layer references
	defer l.releaseLayerReferences(images)
	return (&saveSession{tarexporter: l, images: images}).save(outStream)
}

// parseNames will parse the image names to a map which contains image.ID to *imageDescriptor.
// Each imageDescriptor holds an image top layer reference named 'layerRef'. It is taken here, should be released later.
func (l *tarexporter) parseNames(names []string) (desc map[image.ID]*imageDescriptor, rErr error) {
	imgDescr := make(map[image.ID]*imageDescriptor)
	defer func() {
		if rErr != nil {
			l.releaseLayerReferences(imgDescr)
		}
	}()

	addAssoc := func(id image.ID, ref reference.Named) error {
		if _, ok := imgDescr[id]; !ok {
			descr := &imageDescriptor{}
			if err := l.takeLayerReference(id, descr); err != nil {
				return err
			}
			imgDescr[id] = descr
		}

		if ref != nil {
			if _, ok := ref.(reference.Canonical); ok {
				return nil
			}
			tagged, ok := reference.TagNameOnly(ref).(reference.NamedTagged)
			if !ok {
				return nil
			}

			for _, t := range imgDescr[id].refs {
				if tagged.String() == t.String() {
					return nil
				}
			}
			imgDescr[id].refs = append(imgDescr[id].refs, tagged)
		}
		return nil
	}

	for _, name := range names {
		ref, err := reference.ParseAnyReference(name)
		if err != nil {
			return nil, err
		}
		namedRef, ok := ref.(reference.Named)
		if !ok {
			// Check if digest ID reference
			if digested, ok := ref.(reference.Digested); ok {
				if err := addAssoc(image.ID(digested.Digest()), nil); err != nil {
					return nil, err
				}
				continue
			}
			return nil, errors.Errorf("invalid reference: %v", name)
		}

		if reference.FamiliarName(namedRef) == string(digest.Canonical) {
			imgID, err := l.is.Search(name)
			if err != nil {
				return nil, err
			}
			if err := addAssoc(imgID, nil); err != nil {
				return nil, err
			}
			continue
		}
		if reference.IsNameOnly(namedRef) {
			assocs := l.rs.ReferencesByName(namedRef)
			for _, assoc := range assocs {
				if err := addAssoc(image.ID(assoc.ID), assoc.Ref); err != nil {
					return nil, err
				}
			}
			if len(assocs) == 0 {
				imgID, err := l.is.Search(name)
				if err != nil {
					return nil, err
				}
				if err := addAssoc(imgID, nil); err != nil {
					return nil, err
				}
			}
			continue
		}
		id, err := l.rs.Get(namedRef)
		if err != nil {
			return nil, err
		}
		if err := addAssoc(image.ID(id), namedRef); err != nil {
			return nil, err
		}
	}
	return imgDescr, nil
}

// takeLayerReference will take/Get the image top layer reference
func (l *tarexporter) takeLayerReference(id image.ID, imgDescr *imageDescriptor) error {
	img, err := l.is.Get(id)
	if err != nil {
		return err
	}
	if os := img.OperatingSystem(); !system.IsOSSupported(os) {
		return fmt.Errorf("os %q is not supported", os)
	}
	imgDescr.image = img
	topLayerID := img.RootFS.ChainID()
	if topLayerID == "" {
		return nil
	}
	layer, err := l.lss.Get(topLayerID)
	if err != nil {
		return err
	}
	imgDescr.layerRef = layer
	return nil
}

// releaseLayerReferences will release all the image top layer references
func (l *tarexporter) releaseLayerReferences(imgDescr map[image.ID]*imageDescriptor) error {
	for _, descr := range imgDescr {
		if descr.layerRef != nil {
			l.lss.Release(descr.layerRef)
		}
	}
	return nil
}

func (s *saveSession) save(outStream io.Writer) error {
	s.savedLayers = make(map[string]struct{})
	s.diffIDPaths = make(map[layer.DiffID]string)

	// get image json
	tempDir, err := os.MkdirTemp("", "docker-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	s.outDir = tempDir
	reposLegacy := make(map[string]map[string]string)

	var manifest []manifestItem
	var parentLinks []parentLink

	var manifestDescriptors []ocispec.Descriptor

	for id, imageDescr := range s.images {
		foreignSrcs, err := s.saveImage(id)
		if err != nil {
			return err
		}

		var (
			repoTags []string
			layers   []string
			foreign  = make([]ocispec.Descriptor, 0, len(foreignSrcs))
		)

		for _, desc := range foreignSrcs {
			foreign = append(foreign, ocispec.Descriptor{
				MediaType:   desc.MediaType,
				Digest:      desc.Digest,
				Size:        desc.Size,
				URLs:        desc.URLs,
				Annotations: desc.Annotations,
				Platform:    desc.Platform,
			})
		}

		imgPlat := imageDescr.image.Platform()

		m := ocispec.Manifest{
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			MediaType: ocispec.MediaTypeImageManifest,
			Config: ocispec.Descriptor{
				MediaType: ocispec.MediaTypeImageConfig,
				Digest:    digest.Digest(imageDescr.image.ID()),
				Size:      int64(len(imageDescr.image.RawJSON())),
				Platform:  &imgPlat,
			},
			Layers: foreign,
		}

		data, err := json.Marshal(m)
		if err != nil {
			return errors.Wrap(err, "error marshaling manifest")
		}
		dgst := digest.FromBytes(data)

		mFile := filepath.Join(s.outDir, "blobs", dgst.Algorithm().String(), dgst.Encoded())
		if err := os.MkdirAll(filepath.Dir(mFile), 0o755); err != nil {
			return errors.Wrap(err, "error creating blob directory")
		}
		if err := system.Chtimes(filepath.Dir(mFile), time.Unix(0, 0), time.Unix(0, 0)); err != nil {
			return errors.Wrap(err, "error setting blob directory timestamps")
		}
		if err := os.WriteFile(mFile, data, 0o644); err != nil {
			return errors.Wrap(err, "error writing oci manifest file")
		}
		if err := system.Chtimes(mFile, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
			return errors.Wrap(err, "error setting blob directory timestamps")
		}
		size := int64(len(data))

		for _, ref := range imageDescr.refs {
			familiarName := reference.FamiliarName(ref)
			if _, ok := reposLegacy[familiarName]; !ok {
				reposLegacy[familiarName] = make(map[string]string)
			}
			reposLegacy[familiarName][ref.Tag()] = imageDescr.layers[len(imageDescr.layers)-1].Encoded()
			repoTags = append(repoTags, reference.FamiliarString(ref))

			manifestDescriptors = append(manifestDescriptors, ocispec.Descriptor{
				MediaType: ocispec.MediaTypeImageManifest,
				Digest:    dgst,
				Size:      size,
				Platform:  m.Config.Platform,
				Annotations: map[string]string{
					images.AnnotationImageName: ref.String(),
					ocispec.AnnotationRefName:  ref.Tag(),
				},
			})
		}

		for _, l := range imageDescr.layers {
			// IMPORTANT: We use path, not filepath here to ensure the layers
			// in the manifest use Unix-style forward-slashes.
			layers = append(layers, path.Join("blobs", l.Algorithm().String(), l.Encoded()))
		}

		manifest = append(manifest, manifestItem{
			Config:       path.Join("blobs", id.Digest().Algorithm().String(), id.Digest().Encoded()),
			RepoTags:     repoTags,
			Layers:       layers,
			LayerSources: foreignSrcs,
		})

		parentID, _ := s.is.GetParent(id)
		parentLinks = append(parentLinks, parentLink{id, parentID})
		s.tarexporter.loggerImgEvent.LogImageEvent(id.String(), id.String(), "save")
	}

	for i, p := range validatedParentLinks(parentLinks) {
		if p.parentID != "" {
			manifest[i].Parent = p.parentID
		}
	}

	if len(reposLegacy) > 0 {
		reposFile := filepath.Join(tempDir, legacyRepositoriesFileName)
		rf, err := os.OpenFile(reposFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}

		if err := json.NewEncoder(rf).Encode(reposLegacy); err != nil {
			rf.Close()
			return err
		}

		rf.Close()

		if err := system.Chtimes(reposFile, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
			return err
		}
	}

	manifestPath := filepath.Join(tempDir, manifestFileName)
	f, err := os.OpenFile(manifestPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(f).Encode(manifest); err != nil {
		f.Close()
		return err
	}

	f.Close()

	if err := system.Chtimes(manifestPath, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
		return err
	}

	layoutPath := filepath.Join(tempDir, ociLayoutFilename)
	if err := os.WriteFile(layoutPath, []byte(ociLayoutContent), 0o644); err != nil {
		return errors.Wrap(err, "error writing oci layout file")
	}
	if err := system.Chtimes(layoutPath, time.Unix(0, 0), time.Unix(0, 0)); err != nil {
		return errors.Wrap(err, "error setting oci layout file timestamps")
	}

	data, err := json.Marshal(ocispec.Index{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ocispec.MediaTypeImageIndex,
		Manifests: manifestDescriptors,
	})
	if err != nil {
		return errors.Wrap(err, "error marshaling oci index")
	}

	idxFile := filepath.Join(s.outDir, ociIndexFileName)
	if err := os.WriteFile(idxFile, data, 0o644); err != nil {
		return errors.Wrap(err, "error writing oci index file")
	}

	fs, err := archive.Tar(tempDir, archive.Uncompressed)
	if err != nil {
		return err
	}
	defer fs.Close()

	_, err = io.Copy(outStream, fs)
	return err
}

func (s *saveSession) saveImage(id image.ID) (map[layer.DiffID]distribution.Descriptor, error) {
	img := s.images[id].image
	if len(img.RootFS.DiffIDs) == 0 {
		return nil, fmt.Errorf("empty export - not implemented")
	}

	var parent digest.Digest
	var layers []digest.Digest
	var foreignSrcs map[layer.DiffID]distribution.Descriptor
	for i, diffID := range img.RootFS.DiffIDs {
		v1ImgCreated := time.Unix(0, 0)
		v1Img := image.V1Image{
			// This is for backward compatibility used for
			// pre v1.9 docker.
			Created: &v1ImgCreated,
		}
		if i == len(img.RootFS.DiffIDs)-1 {
			v1Img = img.V1Image
		}
		rootFS := *img.RootFS
		rootFS.DiffIDs = rootFS.DiffIDs[:i+1]
		v1ID, err := v1.CreateID(v1Img, rootFS.ChainID(), parent)
		if err != nil {
			return nil, err
		}

		v1Img.ID = v1ID.Encoded()
		if parent != "" {
			v1Img.Parent = parent.Encoded()
		}

		v1Img.OS = img.OS
		src, err := s.saveLayer(rootFS.ChainID(), v1Img, img.Created)
		if err != nil {
			return nil, err
		}

		layers = append(layers, digest.Digest(diffID))
		parent = v1ID
		if src.Digest != "" {
			if foreignSrcs == nil {
				foreignSrcs = make(map[layer.DiffID]distribution.Descriptor)
			}
			foreignSrcs[img.RootFS.DiffIDs[i]] = src
		}
	}

	data := img.RawJSON()
	dgst := digest.FromBytes(data)

	blobDir := filepath.Join(s.outDir, "blobs", dgst.Algorithm().String())
	if err := os.MkdirAll(blobDir, 0o755); err != nil {
		return nil, err
	}
	if img.Created != nil {
		if err := system.Chtimes(blobDir, *img.Created, *img.Created); err != nil {
			return nil, err
		}
		if err := system.Chtimes(filepath.Dir(blobDir), *img.Created, *img.Created); err != nil {
			return nil, err
		}
	}

	configFile := filepath.Join(blobDir, dgst.Encoded())
	if err := os.WriteFile(configFile, img.RawJSON(), 0o644); err != nil {
		return nil, err
	}
	if img.Created != nil {
		if err := system.Chtimes(configFile, *img.Created, *img.Created); err != nil {
			return nil, err
		}
	}

	s.images[id].layers = layers
	return foreignSrcs, nil
}

func (s *saveSession) saveLayer(id layer.ChainID, legacyImg image.V1Image, createdTime *time.Time) (distribution.Descriptor, error) {
	if _, exists := s.savedLayers[legacyImg.ID]; exists {
		return distribution.Descriptor{}, nil
	}

	outDir := filepath.Join(s.outDir, "blobs")

	imageConfig, err := json.Marshal(legacyImg)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	cfgDgst := digest.FromBytes(imageConfig)
	configPath := filepath.Join(outDir, cfgDgst.Algorithm().String(), cfgDgst.Encoded())
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return distribution.Descriptor{}, errors.Wrap(err, "could not create layer dir parent")
	}

	if err := os.WriteFile(configPath, imageConfig, 0o644); err != nil {
		return distribution.Descriptor{}, err
	}

	// serialize filesystem
	l, err := s.lss.Get(id)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	lDgst := digest.Digest(l.DiffID())
	layerPath := filepath.Join(outDir, lDgst.Algorithm().String(), lDgst.Encoded())
	defer layer.ReleaseAndLog(s.lss, l)

	if _, err = os.Stat(layerPath); err != nil {
		if !os.IsNotExist(err) {
			return distribution.Descriptor{}, err
		}

		// We use sequential file access to avoid depleting the standby list on
		// Windows. On Linux, this equates to a regular os.Create.
		if err := os.MkdirAll(filepath.Dir(layerPath), 0o755); err != nil {
			return distribution.Descriptor{}, errors.Wrap(err, "could not create layer dir parent")
		}
		tarFile, err := sequential.Create(layerPath)
		if err != nil {
			return distribution.Descriptor{}, errors.Wrap(err, "error creating layer file")
		}
		defer tarFile.Close()

		arch, err := l.TarStream()
		if err != nil {
			return distribution.Descriptor{}, err
		}
		defer arch.Close()

		if _, err := io.Copy(tarFile, arch); err != nil {
			return distribution.Descriptor{}, err
		}

		if createdTime != nil {
			for _, fname := range []string{outDir, configPath, layerPath} {
				// todo: maybe save layer created timestamp?
				if err := system.Chtimes(fname, *createdTime, *createdTime); err != nil {
					return distribution.Descriptor{}, errors.Wrap(err, "could not set layer timestamp")
				}
			}
		}

		s.diffIDPaths[l.DiffID()] = layerPath
		s.savedLayers[legacyImg.ID] = struct{}{}
	}

	var src distribution.Descriptor
	if fs, ok := l.(distribution.Describable); ok {
		src = fs.Descriptor()
	}
	return src, nil
}
