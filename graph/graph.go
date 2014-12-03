package graph

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/truncindex"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

// A Graph is a store for versioned filesystem images and the relationship between them.
type Graph struct {
	Root    string
	idIndex *truncindex.TruncIndex
	driver  graphdriver.Driver
}

// NewGraph instantiates a new graph at the given root path in the filesystem.
// `root` will be created if it doesn't exist.
func NewGraph(root string, driver graphdriver.Driver) (*Graph, error) {
	abspath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	// Create the root directory if it doesn't exists
	if err := os.MkdirAll(root, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	graph := &Graph{
		Root:    abspath,
		idIndex: truncindex.NewTruncIndex([]string{}),
		driver:  driver,
	}
	if err := graph.restore(); err != nil {
		return nil, err
	}
	return graph, nil
}

func (graph *Graph) restore() error {
	dir, err := ioutil.ReadDir(graph.Root)
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
	graph.idIndex = truncindex.NewTruncIndex(ids)
	log.Debugf("Restored %d elements", len(dir))
	return nil
}

// FIXME: Implement error subclass instead of looking at the error text
// Note: This is the way golang implements os.IsNotExists on Plan9
func (graph *Graph) IsNotExist(err error) bool {
	return err != nil && (strings.Contains(strings.ToLower(err.Error()), "does not exist") || strings.Contains(strings.ToLower(err.Error()), "no such"))
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
func (graph *Graph) Get(name string) (*image.Image, error) {
	id, err := graph.idIndex.Get(name)
	if err != nil {
		return nil, err
	}
	img, err := image.LoadImage(graph.ImageRoot(id))
	if err != nil {
		return nil, err
	}
	if img.ID != id {
		return nil, fmt.Errorf("Image stored at '%s' has wrong id '%s'", id, img.ID)
	}
	img.SetGraph(graph)

	if img.Size < 0 {
		size, err := graph.driver.DiffSize(img.ID, img.Parent)
		if err != nil {
			return nil, fmt.Errorf("unable to calculate size of image id %q: %s", img.ID, err)
		}

		img.Size = size
		if err := img.SaveSize(graph.ImageRoot(id)); err != nil {
			return nil, err
		}
	}
	return img, nil
}

// Create creates a new image and registers it in the graph.
func (graph *Graph) Create(layerData archive.ArchiveReader, containerID, containerImage, comment, author string, containerConfig, config *runconfig.Config) (*image.Image, error) {
	img := &image.Image{
		ID:            utils.GenerateRandomID(),
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
func (graph *Graph) Register(img *image.Image, layerData archive.ArchiveReader) (err error) {
	defer func() {
		// If any error occurs, remove the new dir from the driver.
		// Don't check for errors since the dir might not have been created.
		// FIXME: this leaves a possible race condition.
		if err != nil {
			graph.driver.Remove(img.ID)
		}
	}()
	if err := utils.ValidateID(img.ID); err != nil {
		return err
	}
	// (This is a convenience to save time. Race conditions are taken care of by os.Rename)
	if graph.Exists(img.ID) {
		return fmt.Errorf("Image %s already exists", img.ID)
	}

	// Ensure that the image root does not exist on the filesystem
	// when it is not registered in the graph.
	// This is common when you switch from one graph driver to another
	if err := os.RemoveAll(graph.ImageRoot(img.ID)); err != nil && !os.IsNotExist(err) {
		return err
	}

	// If the driver has this ID but the graph doesn't, remove it from the driver to start fresh.
	// (the graph is the source of truth).
	// Ignore errors, since we don't know if the driver correctly returns ErrNotExist.
	// (FIXME: make that mandatory for drivers).
	graph.driver.Remove(img.ID)

	tmp, err := graph.Mktemp("")
	defer os.RemoveAll(tmp)
	if err != nil {
		return fmt.Errorf("Mktemp failed: %s", err)
	}

	// Create root filesystem in the driver
	if err := graph.driver.Create(img.ID, img.Parent); err != nil {
		return fmt.Errorf("Driver %s failed to create image rootfs %s: %s", graph.driver, img.ID, err)
	}
	// Apply the diff/layer
	img.SetGraph(graph)
	if err := image.StoreImage(img, layerData, tmp); err != nil {
		return err
	}
	// Commit
	if err := os.Rename(tmp, graph.ImageRoot(img.ID)); err != nil {
		return err
	}
	graph.idIndex.Add(img.ID)
	return nil
}

// TempLayerArchive creates a temporary archive of the given image's filesystem layer.
//   The archive is stored on disk and will be automatically deleted as soon as has been read.
//   If output is not nil, a human-readable progress bar will be written to it.
//   FIXME: does this belong in Graph? How about MktempFile, let the caller use it for archives?
func (graph *Graph) TempLayerArchive(id string, compression archive.Compression, sf *utils.StreamFormatter, output io.Writer) (*archive.TempArchive, error) {
	image, err := graph.Get(id)
	if err != nil {
		return nil, err
	}
	tmp, err := graph.Mktemp("")
	if err != nil {
		return nil, err
	}
	a, err := image.TarLayer()
	if err != nil {
		return nil, err
	}
	progress := utils.ProgressReader(a, 0, output, sf, false, utils.TruncateID(id), "Buffering to disk")
	defer progress.Close()
	return archive.NewTempArchive(progress, tmp)
}

// Mktemp creates a temporary sub-directory inside the graph's filesystem.
func (graph *Graph) Mktemp(id string) (string, error) {
	dir := path.Join(graph.Root, "_tmp", utils.GenerateRandomID())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// setupInitLayer populates a directory with mountpoints suitable
// for bind-mounting dockerinit into the container. The mountpoint is simply an
// empty file at /.dockerinit
//
// This extra layer is used by all containers as the top-most ro layer. It protects
// the container from unwanted side-effects on the rw layer.
func SetupInitLayer(initLayer string) error {
	for pth, typ := range map[string]string{
		"/dev/pts":         "dir",
		"/dev/shm":         "dir",
		"/proc":            "dir",
		"/sys":             "dir",
		"/.dockerinit":     "file",
		"/.dockerenv":      "file",
		"/etc/resolv.conf": "file",
		"/etc/hosts":       "file",
		"/etc/hostname":    "file",
		"/dev/console":     "file",
		"/etc/mtab":        "/proc/mounts",
	} {
		parts := strings.Split(pth, "/")
		prev := "/"
		for _, p := range parts[1:] {
			prev = path.Join(prev, p)
			syscall.Unlink(path.Join(initLayer, prev))
		}

		if _, err := os.Stat(path.Join(initLayer, pth)); err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(path.Join(initLayer, path.Dir(pth)), 0755); err != nil {
					return err
				}
				switch typ {
				case "dir":
					if err := os.MkdirAll(path.Join(initLayer, pth), 0755); err != nil {
						return err
					}
				case "file":
					f, err := os.OpenFile(path.Join(initLayer, pth), os.O_CREATE, 0755)
					if err != nil {
						return err
					}
					f.Close()
				default:
					if err := os.Symlink(typ, path.Join(initLayer, pth)); err != nil {
						return err
					}
				}
			} else {
				return err
			}
		}
	}

	// Layer is ready to use, if it wasn't before.
	return nil
}

// Check if given error is "not empty".
// Note: this is the way golang does it internally with os.IsNotExists.
func isNotEmpty(err error) bool {
	switch pe := err.(type) {
	case nil:
		return false
	case *os.PathError:
		err = pe.Err
	case *os.LinkError:
		err = pe.Err
	}
	return strings.Contains(err.Error(), " not empty")
}

// Delete atomically removes an image from the graph.
func (graph *Graph) Delete(name string) error {
	id, err := graph.idIndex.Get(name)
	if err != nil {
		return err
	}
	tmp, err := graph.Mktemp("")
	graph.idIndex.Delete(id)
	if err == nil {
		err = os.Rename(graph.ImageRoot(id), tmp)
		// On err make tmp point to old dir and cleanup unused tmp dir
		if err != nil {
			os.RemoveAll(tmp)
			tmp = graph.ImageRoot(id)
		}
	} else {
		// On err make tmp point to old dir for cleanup
		tmp = graph.ImageRoot(id)
	}
	// Remove rootfs data from the driver
	graph.driver.Remove(id)
	// Remove the trashed image directory
	return os.RemoveAll(tmp)
}

// Map returns a list of all images in the graph, addressable by ID.
func (graph *Graph) Map() (map[string]*image.Image, error) {
	images := make(map[string]*image.Image)
	err := graph.walkAll(func(image *image.Image) {
		images[image.ID] = image
	})
	if err != nil {
		return nil, err
	}
	return images, nil
}

// walkAll iterates over each image in the graph, and passes it to a handler.
// The walking order is undetermined.
func (graph *Graph) walkAll(handler func(*image.Image)) error {
	files, err := ioutil.ReadDir(graph.Root)
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
func (graph *Graph) ByParent() (map[string][]*image.Image, error) {
	byParent := make(map[string][]*image.Image)
	err := graph.walkAll(func(img *image.Image) {
		parent, err := graph.Get(img.Parent)
		if err != nil {
			return
		}
		if children, exists := byParent[parent.ID]; exists {
			byParent[parent.ID] = append(children, img)
		} else {
			byParent[parent.ID] = []*image.Image{img}
		}
	})
	return byParent, err
}

// Heads returns all heads in the graph, keyed by id.
// A head is an image which is not the parent of another image in the graph.
func (graph *Graph) Heads() (map[string]*image.Image, error) {
	heads := make(map[string]*image.Image)
	byParent, err := graph.ByParent()
	if err != nil {
		return nil, err
	}
	err = graph.walkAll(func(image *image.Image) {
		// If it's not in the byParent lookup table, then
		// it's not a parent -> so it's a head!
		if _, exists := byParent[image.ID]; !exists {
			heads[image.ID] = image
		}
	})
	return heads, err
}

func (graph *Graph) ImageRoot(id string) string {
	return path.Join(graph.Root, id)
}

func (graph *Graph) Driver() graphdriver.Driver {
	return graph.driver
}
