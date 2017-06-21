package manifest

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/opencontainers/go-digest"
)

// Store provides methods which can operate on a manifest store.
type Store interface {
	Get(reference.Canonical) (*distribution.Manifest, error)
	GetImageID(reference.Canonical) (image.ID, error)
	Set(string, distribution.Manifest) (digest.Digest, error)
	Delete(reference.Canonical) error
	DeleteByImageID(image.ID) error
}

// store implements a Store on the filesystem.
type store struct {
	sync.RWMutex
	// root is the directory under which manifests are cached.
	root string
}

var (
	// ErrNotFound indicates an entity was not found in the store.
	ErrNotFound = errors.New("Not found")
)

const (
	contentDirName = "content"
)

// NewManifestStore returns a new filesystem backed Store.
func NewManifestStore(root string) (Store, error) {
	abspath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	s := &store{root: abspath}
	if err := os.MkdirAll(s.contentPath(), 0700); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *store) contentPath() string {
	return filepath.Join(s.root, contentDirName, string(digest.Canonical))
}

func (s *store) contentFile(ref reference.Canonical) string {
	base46Ref := base64.StdEncoding.EncodeToString([]byte(ref.String()))
	return filepath.Join(s.contentPath(), base46Ref)
}

func newReference(str string) (reference.Canonical, error) {
	ref, err := reference.Parse(str)
	if err != nil {
		return nil, err
	}

	canonical, ok := ref.(reference.Canonical)
	if !ok {
		return nil, errors.New("Expected reference to be canonical")
	}
	return canonical, nil
}

// Get returns the content stored under a given canonical reference.
// The content is verified to match the digest in the lookup reference.
func (s *store) Get(ref reference.Canonical) (*distribution.Manifest, error) {
	s.RLock()
	defer s.RUnlock()

	data, err := ioutil.ReadFile(s.contentFile(ref))
	if err != nil {
		return nil, err
	}

	dgst := ref.Digest()
	if digest.FromBytes(data) != dgst {
		return nil, fmt.Errorf("failed to verify cached manifest: %v", dgst)
	}

	var version manifest.Versioned
	if err := json.Unmarshal(data, &version); err != nil {
		return nil, err
	}

	m, _, err := distribution.UnmarshalManifest(version.MediaType, data)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// GetImageID retrieves the content stored under a given canonical reference and extracts the image ID.
// The content is verified to match the digest in the lookup reference.
func (s *store) GetImageID(ref reference.Canonical) (image.ID, error) {
	m, err := s.Get(ref)
	if err != nil {
		return "", ErrNotFound
	}
	return s.toImageID(ref, m)
}

// Set stores content by canonical reference.
func (s *store) Set(name string, manifest distribution.Manifest) (digest.Digest, error) {
	s.Lock()
	defer s.Unlock()

	_, data, err := manifest.Payload()
	if err != nil {
		return "", err
	}

	ref, err := newReference(fmt.Sprintf("%s@%s", name, digest.FromBytes(data)))
	if err != nil {
		return "", err
	}
	if err := ioutils.AtomicWriteFile(s.contentFile(ref), data, 0600); err != nil {
		return "", err
	}

	return ref.Digest(), nil
}

// Delete removes the content identified by the canonical reference.
func (s *store) Delete(ref reference.Canonical) error {
	s.Lock()
	defer s.Unlock()

	return os.Remove(s.contentFile(ref))
}

// DeleteByImageID removes any content containing the given image ID.
func (s *store) DeleteByImageID(imageID image.ID) error {
	files, err := ioutil.ReadDir(s.contentPath())
	if err != nil {
		return err
	}

	for _, f := range files {
		decoded, err := base64.StdEncoding.DecodeString(f.Name())
		if err != nil {
			return err
		}

		ref, err := newReference(string(decoded))
		if err != nil {
			return err
		}

		m, err := s.Get(ref)
		if err != nil {
			return err
		}

		id, err := s.toImageID(ref, m)
		if err != nil {
			return err
		}

		if imageID == id {
			err = s.Delete(ref)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *store) toImageID(ref reference.Canonical, m *distribution.Manifest) (image.ID, error) {
	switch v := (*m).(type) {
	case *schema2.DeserializedManifest:
		return image.IDFromDigest(v.Target().Digest), nil
	case *manifestlist.DeserializedManifestList:
		subManifestDigest := subManifestForRuntime(v.Manifests)
		if subManifestDigest == "" {
			return "", ErrNotFound
		}

		subRef, err := newReference(fmt.Sprintf("%s@%s", ref.Name(), subManifestDigest))
		if err != nil {
			return "", ErrNotFound
		}

		subManifest, err := s.Get(subRef)
		if err != nil {
			return "", ErrNotFound
		}

		if v2, ok := (*subManifest).(*schema2.DeserializedManifest); ok {
			return image.IDFromDigest(v2.Target().Digest), nil
		}
	}
	return "", ErrNotFound
}

// subManifestForRuntime is a helper to find the appropriate manifest for the current runtime.
func subManifestForRuntime(descriptors []manifestlist.ManifestDescriptor) digest.Digest {
	for _, descriptor := range descriptors {
		// Look for digest for this system.
		if descriptor.Platform.Architecture == runtime.GOARCH && descriptor.Platform.OS == runtime.GOOS {
			return descriptor.Digest
		}
	}
	return ""
}
