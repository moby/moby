package graph

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/pkg/truncindex"
	"github.com/docker/docker/runconfig"
)

// A Graph is a store for versioned filesystem images and the relationship between them.
type Graph struct {
	root       string
	idIndex    *truncindex.TruncIndex
	driver     graphdriver.Driver
	imageMutex imageMutex // protect images in driver.
}

type Image struct {
	ID              string            `json:"id"`
	Parent          string            `json:"parent,omitempty"`
	Comment         string            `json:"comment,omitempty"`
	Created         time.Time         `json:"created"`
	Container       string            `json:"container,omitempty"`
	ContainerConfig runconfig.Config  `json:"container_config,omitempty"`
	DockerVersion   string            `json:"docker_version,omitempty"`
	Author          string            `json:"author,omitempty"`
	Config          *runconfig.Config `json:"config,omitempty"`
	Architecture    string            `json:"architecture,omitempty"`
	OS              string            `json:"os,omitempty"`
	Size            int64
	graph           Graph
}

var (
	// ErrDigestNotSet is used when request the digest for a layer
	// but the layer has no digest value or content to compute the
	// the digest.
	ErrDigestNotSet = errors.New("digest is not set for layer")
)

// NewGraph instantiates a new graph at the given root path in the filesystem.
// `root` will be created if it doesn't exist.
func NewGraph(root string, driver graphdriver.Driver) (*Graph, error) {
	abspath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	// Create the root directory if it doesn't exists
	if err := system.MkdirAll(root, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	graph := &Graph{
		root:    abspath,
		idIndex: truncindex.NewTruncIndex([]string{}),
		driver:  driver,
	}
	if err := graph.restore(); err != nil {
		return nil, err
	}
	return graph, nil
}

func (graph *Graph) restore() error {
	dir, err := ioutil.ReadDir(graph.root)
	if err != nil {
		return err
	}
	var ids = []string{}
	for _, v := range dir {
		id := v.Name()
		if graph.driver.Exists(id) {
			ids = append(ids, id)
		}
	}

	baseIds, err := graph.restoreBaseImages()
	if err != nil {
		return err
	}
	ids = append(ids, baseIds...)

	graph.idIndex = truncindex.NewTruncIndex(ids)
	logrus.Debugf("Restored %d elements", len(ids))
	return nil
}

// FIXME: Implement error subclass instead of looking at the error text
// Note: This is the way golang implements os.IsNotExists on Plan9
func (graph *Graph) IsNotExist(err error, id string) bool {
	return err != nil && (strings.Contains(strings.ToLower(err.Error()), "does not exist") || strings.Contains(strings.ToLower(err.Error()), "no such")) && strings.Contains(err.Error(), id)
}

// Exists returns true if an image is registered at the given id.
// If the image doesn't exist or if an error is encountered, false is returned.
func (graph *Graph) Exists(id string) bool {
	if _, err := graph.Get(id); err != nil {
		return false
	}
	return true
}

// Get returns the image with the given id, or an error if the image doesn't exist.
func (graph *Graph) Get(name string) (*Image, error) {
	id, err := graph.idIndex.Get(name)
	if err != nil {
		return nil, fmt.Errorf("could not find image: %v", err)
	}
	img, err := graph.loadImage(id)
	if err != nil {
		return nil, err
	}
	if img.ID != id {
		return nil, fmt.Errorf("Image stored at '%s' has wrong id '%s'", id, img.ID)
	}

	if img.Size < 0 {
		size, err := graph.driver.DiffSize(img.ID, img.Parent)
		if err != nil {
			return nil, fmt.Errorf("unable to calculate size of image id %q: %s", img.ID, err)
		}

		img.Size = size
		if err := graph.saveSize(graph.imageRoot(id), int(img.Size)); err != nil {
			return nil, err
		}
	}
	return img, nil
}

// Create creates a new image and registers it in the graph.
func (graph *Graph) Create(layerData archive.ArchiveReader, containerID, containerImage, comment, author string, containerConfig, config *runconfig.Config) (*Image, error) {
	img := &Image{
		ID:            stringid.GenerateRandomID(),
		Comment:       comment,
		Created:       time.Now().UTC(),
		DockerVersion: dockerversion.VERSION,
		Author:        author,
		Config:        config,
		Architecture:  runtime.GOARCH,
		OS:            runtime.GOOS,
	}

	if containerID != "" {
		img.Parent = containerImage
		img.Container = containerID
		img.ContainerConfig = *containerConfig
	}

	if err := graph.Register(img, layerData); err != nil {
		return nil, err
	}
	return img, nil
}

// Register imports a pre-existing image into the graph.
func (graph *Graph) Register(img *Image, layerData archive.ArchiveReader) (err error) {
	if err := image.ValidateID(img.ID); err != nil {
		return err
	}

	// We need this entire operation to be atomic within the engine. Note that
	// this doesn't mean Register is fully safe yet.
	graph.imageMutex.Lock(img.ID)
	defer graph.imageMutex.Unlock(img.ID)

	defer func() {
		// If any error occurs, remove the new dir from the driver.
		// Don't check for errors since the dir might not have been created.
		// FIXME: this leaves a possible race condition.
		if err != nil {
			graph.driver.Remove(img.ID)
		}
	}()

	// (This is a convenience to save time. Race conditions are taken care of by os.Rename)
	if graph.Exists(img.ID) {
		return fmt.Errorf("Image %s already exists", img.ID)
	}

	// Ensure that the image root does not exist on the filesystem
	// when it is not registered in the graph.
	// This is common when you switch from one graph driver to another
	if err := os.RemoveAll(graph.imageRoot(img.ID)); err != nil && !os.IsNotExist(err) {
		return err
	}

	// If the driver has this ID but the graph doesn't, remove it from the driver to start fresh.
	// (the graph is the source of truth).
	// Ignore errors, since we don't know if the driver correctly returns ErrNotExist.
	// (FIXME: make that mandatory for drivers).
	graph.driver.Remove(img.ID)

	tmp, err := graph.mktemp("")
	defer os.RemoveAll(tmp)
	if err != nil {
		return fmt.Errorf("mktemp failed: %s", err)
	}

	// Create root filesystem in the driver
	if err := createRootFilesystemInDriver(graph, img, layerData); err != nil {
		return err
	}

	// Apply the diff/layer
	if err := graph.storeImage(img, layerData, tmp); err != nil {
		return err
	}
	// Commit
	if err := os.Rename(tmp, graph.imageRoot(img.ID)); err != nil {
		return err
	}
	graph.idIndex.Add(img.ID)
	return nil
}

// TempLayerArchive creates a temporary archive of the given image's filesystem layer.
//   The archive is stored on disk and will be automatically deleted as soon as has been read.
//   If output is not nil, a human-readable progress bar will be written to it.
func (graph *Graph) TempLayerArchive(id string, sf *streamformatter.StreamFormatter, output io.Writer) (*archive.TempArchive, error) {
	image, err := graph.Get(id)
	if err != nil {
		return nil, err
	}
	tmp, err := graph.mktemp("")
	if err != nil {
		return nil, err
	}
	a, err := graph.TarLayer(image)
	if err != nil {
		return nil, err
	}
	progressReader := progressreader.New(progressreader.Config{
		In:        a,
		Out:       output,
		Formatter: sf,
		Size:      0,
		NewLines:  false,
		ID:        stringid.TruncateID(id),
		Action:    "Buffering to disk",
	})
	defer progressReader.Close()
	return archive.NewTempArchive(progressReader, tmp)
}

// mktemp creates a temporary sub-directory inside the graph's filesystem.
func (graph *Graph) mktemp(id string) (string, error) {
	dir := filepath.Join(graph.root, "_tmp", stringid.GenerateRandomID())
	if err := system.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func (graph *Graph) newTempFile() (*os.File, error) {
	tmp, err := graph.mktemp("")
	if err != nil {
		return nil, err
	}
	return ioutil.TempFile(tmp, "")
}

func bufferToFile(f *os.File, src io.Reader) (int64, digest.Digest, error) {
	var (
		h = sha256.New()
		w = gzip.NewWriter(io.MultiWriter(f, h))
	)
	_, err := io.Copy(w, src)
	w.Close()
	if err != nil {
		return 0, "", err
	}
	n, err := f.Seek(0, os.SEEK_CUR)
	if err != nil {
		return 0, "", err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return 0, "", err
	}
	return n, digest.NewDigest("sha256", h), nil
}

// Delete atomically removes an image from the graph.
func (graph *Graph) Delete(name string) error {
	id, err := graph.idIndex.Get(name)
	if err != nil {
		return err
	}
	tmp, err := graph.mktemp("")
	graph.idIndex.Delete(id)
	if err == nil {
		if err := os.Rename(graph.imageRoot(id), tmp); err != nil {
			// On err make tmp point to old dir and cleanup unused tmp dir
			os.RemoveAll(tmp)
			tmp = graph.imageRoot(id)
		}
	} else {
		// On err make tmp point to old dir for cleanup
		tmp = graph.imageRoot(id)
	}
	// Remove rootfs data from the driver
	graph.driver.Remove(id)
	// Remove the trashed image directory
	return os.RemoveAll(tmp)
}

// Map returns a list of all images in the graph, addressable by ID.
func (graph *Graph) Map() (map[string]*Image, error) {
	images := make(map[string]*Image)
	err := graph.walkAll(func(image *Image) {
		images[image.ID] = image
	})
	if err != nil {
		return nil, err
	}
	return images, nil
}

// walkAll iterates over each image in the graph, and passes it to a handler.
// The walking order is undetermined.
func (graph *Graph) walkAll(handler func(*Image)) error {
	files, err := ioutil.ReadDir(graph.root)
	if err != nil {
		return err
	}
	for _, st := range files {
		if img, err := graph.Get(st.Name()); err != nil {
			// Skip image
			continue
		} else if handler != nil {
			handler(img)
		}
	}
	return nil
}

// ByParent returns a lookup table of images by their parent.
// If an image of id ID has 3 children images, then the value for key ID
// will be a list of 3 images.
// If an image has no children, it will not have an entry in the table.
func (graph *Graph) ByParent() (map[string][]*Image, error) {
	byParent := make(map[string][]*Image)
	err := graph.walkAll(func(img *Image) {
		parent, err := graph.Get(img.Parent)
		if err != nil {
			return
		}
		if children, exists := byParent[parent.ID]; exists {
			byParent[parent.ID] = append(children, img)
		} else {
			byParent[parent.ID] = []*Image{img}
		}
	})
	return byParent, err
}

// Heads returns all heads in the graph, keyed by id.
// A head is an image which is not the parent of another image in the graph.
func (graph *Graph) Heads() (map[string]*Image, error) {
	heads := make(map[string]*Image)
	byParent, err := graph.ByParent()
	if err != nil {
		return nil, err
	}
	err = graph.walkAll(func(image *Image) {
		// If it's not in the byParent lookup table, then
		// it's not a parent -> so it's a head!
		if _, exists := byParent[image.ID]; !exists {
			heads[image.ID] = image
		}
	})
	return heads, err
}

func (graph *Graph) imageRoot(id string) string {
	return filepath.Join(graph.root, id)
}

// loadImage fetches the image with the given id from the graph.
func (graph *Graph) loadImage(id string) (*Image, error) {
	root := graph.imageRoot(id)

	// Open the JSON file to decode by streaming
	jsonSource, err := os.Open(jsonPath(root))
	if err != nil {
		return nil, err
	}
	defer jsonSource.Close()

	img := &Image{}
	dec := json.NewDecoder(jsonSource)

	// Decode the JSON data
	if err := dec.Decode(img); err != nil {
		return nil, err
	}
	if err := image.ValidateID(img.ID); err != nil {
		return nil, err
	}

	if buf, err := ioutil.ReadFile(filepath.Join(root, "layersize")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		// If the layersize file does not exist then set the size to a negative number
		// because a layer size of 0 (zero) is valid
		img.Size = -1
	} else {
		// Using Atoi here instead would temporarily convert the size to a machine
		// dependent integer type, which causes images larger than 2^31 bytes to
		// display negative sizes on 32-bit machines:
		size, err := strconv.ParseInt(string(buf), 10, 64)
		if err != nil {
			return nil, err
		}
		img.Size = int64(size)
	}

	return img, nil
}

// saveSize stores the `size` in the provided graph `img` directory `root`.
func (graph *Graph) saveSize(root string, size int) error {
	if err := ioutil.WriteFile(filepath.Join(root, "layersize"), []byte(strconv.Itoa(size)), 0600); err != nil {
		return fmt.Errorf("Error storing image size in %s/layersize: %s", root, err)
	}
	return nil
}

// SetDigest sets the digest for the image layer to the provided value.
func (graph *Graph) SetDigest(id string, dgst digest.Digest) error {
	root := graph.imageRoot(id)
	if err := ioutil.WriteFile(filepath.Join(root, "checksum"), []byte(dgst.String()), 0600); err != nil {
		return fmt.Errorf("Error storing digest in %s/checksum: %s", root, err)
	}
	return nil
}

// GetDigest gets the digest for the provide image layer id.
func (graph *Graph) GetDigest(id string) (digest.Digest, error) {
	root := graph.imageRoot(id)
	cs, err := ioutil.ReadFile(filepath.Join(root, "checksum"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrDigestNotSet
		}
		return "", err
	}
	return digest.ParseDigest(string(cs))
}

// RawJSON returns the JSON representation for an image as a byte array.
func (graph *Graph) RawJSON(id string) ([]byte, error) {
	root := graph.imageRoot(id)

	buf, err := ioutil.ReadFile(jsonPath(root))
	if err != nil {
		return nil, fmt.Errorf("Failed to read json for image %s: %s", id, err)
	}

	return buf, nil
}

func jsonPath(root string) string {
	return filepath.Join(root, "json")
}

// Build an Image object from raw json data
func NewImgJSON(src []byte) (*Image, error) {
	ret := &Image{}

	// FIXME: Is there a cleaner way to "purify" the input json?
	if err := json.Unmarshal(src, ret); err != nil {
		return nil, err
	}
	return ret, nil
}
