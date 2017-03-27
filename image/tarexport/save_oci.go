package tarexport

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/system"
	"github.com/opencontainers/go-digest"
	imgspec "github.com/opencontainers/image-spec/specs-go"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type layerInfo struct {
	digest digest.Digest
	size   int64
}

type ociSaveSession struct {
	*tarexporter
	images       map[image.ID]*imageDescriptor
	name         string
	savedImages  map[image.ID]ociv1.Manifest // cache image.ID -> manifest bytes
	diffIDsCache map[layer.DiffID]*layerInfo
	outDir       string
	index        ociv1.ImageIndex
}

func (l *tarexporter) getRefs() (map[string]string, map[string]reference.NamedTagged, error) {
	refs := make(map[string]string)
	reversed := make(map[string]reference.NamedTagged)
	for image, ref := range l.refs {
		r, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			return nil, nil, err
		}
		if _, ok := r.(reference.Canonical); ok {
			continue // a digest reference it's unique, no need for a --ref
		}
		tagged, ok := reference.TagNameOnly(r).(reference.NamedTagged)
		if !ok {
			return nil, nil, fmt.Errorf("error adding default tag to ref %q", ref)
		}
		if !ociRefRegexp.MatchString(ref) {
			return nil, nil, fmt.Errorf(`invalid reference "%s=%s", reference must not include characters outside of the set of "A" to "Z", "a" to "z", "0" to "9", the hyphen "-", the dot ".", and the underscore "_"`, image, ref)
		}
		refs[reference.FamiliarString(tagged)] = ref
		reversed[ref] = tagged
	}
	return refs, reversed, nil
}

var ociRefRegexp = regexp.MustCompile(`^([A-Za-z0-9._-]+)+$`)

func (l *tarexporter) parseOCINames(names []string) (map[image.ID]*imageDescriptor, error) {
	refs, _, err := l.getRefs()
	if err != nil {
		return nil, err
	}
	imgDescr := make(map[image.ID]*imageDescriptor)
	tags := make(map[string]bool)

	addAssoc := func(id image.ID, ref reference.Named) error {
		if _, ok := imgDescr[id]; !ok {
			imgDescr[id] = &imageDescriptor{}
		}

		if ref != nil {
			if _, ok := ref.(reference.Canonical); ok {
				return nil
			}
			tagged, ok := reference.TagNameOnly(ref).(reference.NamedTagged)
			if !ok {
				return nil
			}

			r, ok := refs[reference.FamiliarString(tagged)]
			if ok {
				var err error
				if tagged, err = reference.WithTag(tagged, r); err != nil {
					return err
				}
			}

			for _, t := range imgDescr[id].refs {
				if reference.FamiliarString(tagged) == reference.FamiliarString(t) {
					return nil
				}
			}

			if tags[tagged.Tag()] {
				return fmt.Errorf("unable to include unique references %q in OCI image", tagged.Tag())
			}

			tags[tagged.Tag()] = true

			imgDescr[id].refs = append(imgDescr[id].refs, tagged)
		}

		return nil
	}

	// TODO(runcom): same as docker-save except the error return in addAssoc
	// and the tags map above.
	for _, name := range names {
		ref, err := reference.ParseAnyReference(name)
		if err != nil {
			return nil, err
		}
		namedRef, ok := ref.(reference.Named)
		if !ok {
			// Check if digest ID reference
			if digested, ok := ref.(reference.Digested); ok {
				id := image.IDFromDigest(digested.Digest())
				_, err := l.is.Get(id)
				if err != nil {
					return nil, err
				}
				if err := addAssoc(id, nil); err != nil {
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
				if err := addAssoc(image.IDFromDigest(assoc.ID), assoc.Ref); err != nil {
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
		if err := addAssoc(image.IDFromDigest(id), namedRef); err != nil {
			return nil, err
		}
	}
	return imgDescr, nil
}

func (s *ociSaveSession) save(outStream io.Writer) error {
	s.diffIDsCache = make(map[layer.DiffID]*layerInfo)
	s.savedImages = make(map[image.ID]ociv1.Manifest)
	s.index = ociv1.ImageIndex{
		Versioned: imgspec.Versioned{
			SchemaVersion: 2,
		},
	}
	tempDir, err := ioutil.TempDir("", "oci-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	s.outDir = tempDir

	if err := ioutil.WriteFile(filepath.Join(tempDir, "oci-layout"), []byte(`{"imageLayoutVersion": "1.0.0"}`), 0644); err != nil {
		return err
	}

	for id, info := range s.images {
		for _, i := range info.refs {
			// TODO(runcom): handle foreign srcs like save.go
			if err := s.saveImage(id, i.Tag()); err != nil {
				return err
			}
		}
		if len(info.refs) == 0 {
			// TODO(runcom): handle foreign srcs like save.go
			if err := s.saveImage(id, id.Digest().Hex()); err != nil {
				return err
			}
		}
	}

	indexJSON, err := json.Marshal(s.index)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(tempDir, "index.json"), indexJSON, 0644); err != nil {
		return err
	}

	fs, err := archive.Tar(tempDir, archive.Uncompressed)
	if err != nil {
		return err
	}
	defer fs.Close()

	_, err = io.Copy(outStream, fs)
	return err
}

func (s *ociSaveSession) saveImage(id image.ID, ref string) error {
	if m, ok := s.savedImages[id]; ok {
		if err := s.saveManifest(ref, m); err != nil {
			return err
		}
		return nil
	}

	img, err := s.is.Get(id)
	if err != nil {
		return err
	}

	if len(img.RootFS.DiffIDs) == 0 {
		return fmt.Errorf("empty export - not implemented")
	}

	configFile, err := blobPath(s.outDir, id.Digest())
	if err != nil {
		return err
	}
	if err := ensureParentDirectoryExists(configFile); err != nil {
		return err
	}
	if err := ioutil.WriteFile(configFile, img.RawJSON(), 0644); err != nil {
		return err
	}
	if err := system.Chtimes(configFile, img.Created, img.Created); err != nil {
		return err
	}

	// TODO(runcom): there should likely be a manifest builder (like docker/distribution)
	m := ociv1.Manifest{
		Versioned: imgspec.Versioned{
			SchemaVersion: 2,
		},
		Config: ociv1.Descriptor{
			MediaType: ociv1.MediaTypeImageConfig,
			Digest:    img.ID().Digest(),
			Size:      int64(len(img.RawJSON())),
		},
	}

	for i := range img.RootFS.DiffIDs {
		rootFS := *img.RootFS
		rootFS.DiffIDs = rootFS.DiffIDs[:i+1]

		l, err := s.ls.Get(rootFS.ChainID())
		if err != nil {
			return err
		}
		defer layer.ReleaseAndLog(s.ls, l)

		var (
			digest digest.Digest
			size   int64
		)
		if i, ok := s.diffIDsCache[l.DiffID()]; ok {
			digest = i.digest
			size = i.size
		} else {
			lInfo, err := s.saveLayer(l)
			if err != nil {
				return err
			}
			digest = lInfo.digest
			size = lInfo.size
		}

		descriptor := ociv1.Descriptor{
			MediaType: ociv1.MediaTypeImageLayer,
			Digest:    digest,
			Size:      size,
		}
		m.Layers = append(m.Layers, descriptor)
	}

	if err := s.saveManifest(ref, m); err != nil {
		return err
	}

	s.savedImages[id] = m

	return nil
}

func (s *ociSaveSession) saveManifest(ref string, ociMan ociv1.Manifest) error {
	b, err := json.Marshal(ociMan)
	if err != nil {
		return err
	}
	d := digest.FromBytes(b)
	desc := ociv1.Descriptor{}
	desc.Digest = d
	desc.Size = int64(len(b))
	desc.MediaType = ociv1.MediaTypeImageManifest

	blobPath, err := blobPath(s.outDir, d)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(blobPath, b, 0644); err != nil {
		return err
	}
	annotations := make(map[string]string)
	annotations["org.opencontainers.ref.name"] = ref
	desc.Annotations = annotations
	s.index.Manifests = append(s.index.Manifests, ociv1.ManifestDescriptor{
		Descriptor: desc,
		Platform: ociv1.Platform{
			Architecture: runtime.GOARCH,
			OS:           runtime.GOOS,
		},
	})
	return nil
}

func (s *ociSaveSession) saveLayer(l layer.Layer) (*layerInfo, error) {
	arch, err := l.TarStream()
	if err != nil {
		return nil, err
	}
	defer arch.Close()

	blobFile, err := ioutil.TempFile(s.outDir, "oci-blob")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(blobFile.Name())

	digester := digest.Canonical.Digester()
	tee := io.TeeReader(arch, digester.Hash())

	size, err := io.Copy(blobFile, tee)
	if err != nil {
		return nil, err
	}
	computedDigest := digester.Digest()
	if err := blobFile.Sync(); err != nil {
		return nil, err
	}
	if err := blobFile.Chmod(0644); err != nil {
		return nil, err
	}
	blobPath, err := blobPath(s.outDir, computedDigest)
	if err != nil {
		return nil, err
	}
	if err := ensureParentDirectoryExists(blobPath); err != nil {
		return nil, err
	}
	if err := os.Rename(blobFile.Name(), blobPath); err != nil {
		return nil, err
	}
	li := &layerInfo{digest: computedDigest, size: size}
	s.diffIDsCache[l.DiffID()] = li
	return li, nil
}

func ensureDirectoryExists(path string) error {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

// ensureParentDirectoryExists ensures the parent of the supplied path exists.
func ensureParentDirectoryExists(path string) error {
	return ensureDirectoryExists(filepath.Dir(path))
}

func blobPath(tmp string, digest digest.Digest) (string, error) {
	if err := digest.Validate(); err != nil {
		return "", fmt.Errorf("unexpected digest reference %s: %v", digest.String(), err)
	}
	return filepath.Join(tmp, "blobs", digest.Algorithm().String(), digest.Hex()), nil
}
