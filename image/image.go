package image

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/docker/docker/archive"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/log"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

// Set the max depth to the aufs default that most
// kernels are compiled with
// For more information see: http://sourceforge.net/p/aufs/aufs3-standalone/ci/aufs3.12/tree/config.mk
const MaxImageDepth = 127

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

	graph Graph
}

func LoadImage(root string) (*Image, error) {
	// Load the json data
	jsonData, err := ioutil.ReadFile(jsonPath(root))
	if err != nil {
		return nil, err
	}
	img := &Image{}

	if err := json.Unmarshal(jsonData, img); err != nil {
		return nil, err
	}
	if err := utils.ValidateID(img.ID); err != nil {
		return nil, err
	}

	if buf, err := ioutil.ReadFile(path.Join(root, "layersize")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		// If the layersize file does not exist then set the size to a negative number
		// because a layer size of 0 (zero) is valid
		img.Size = -1
	} else {
		size, err := strconv.Atoi(string(buf))
		if err != nil {
			return nil, err
		}
		img.Size = int64(size)
	}

	return img, nil
}

func StoreImage(img *Image, jsonData []byte, layerData archive.ArchiveReader, root, layer string) error {
	// Store the layer
	var (
		size   int64
		err    error
		driver = img.graph.Driver()
	)
	if err := os.MkdirAll(layer, 0755); err != nil {
		return err
	}

	// If layerData is not nil, unpack it into the new layer
	if layerData != nil {
		if differ, ok := driver.(graphdriver.Differ); ok {
			if err := differ.ApplyDiff(img.ID, layerData); err != nil {
				return err
			}

			if size, err = differ.DiffSize(img.ID); err != nil {
				return err
			}
		} else {
			start := time.Now().UTC()
			log.Debugf("Start untar layer")
			if err := archive.ApplyLayer(layer, layerData); err != nil {
				return err
			}
			log.Debugf("Untar time: %vs", time.Now().UTC().Sub(start).Seconds())

			if img.Parent == "" {
				if size, err = utils.TreeSize(layer); err != nil {
					return err
				}
			} else {
				parent, err := driver.Get(img.Parent, "")
				if err != nil {
					return err
				}
				defer driver.Put(img.Parent)
				changes, err := archive.ChangesDirs(layer, parent)
				if err != nil {
					return err
				}
				size = archive.ChangesSize(layer, changes)
			}
		}
	}

	img.Size = size
	if err := img.SaveSize(root); err != nil {
		return err
	}

	// If raw json is provided, then use it
	if jsonData != nil {
		if err := ioutil.WriteFile(jsonPath(root), jsonData, 0600); err != nil {
			return err
		}
	} else {
		if jsonData, err = json.Marshal(img); err != nil {
			return err
		}
		if err := ioutil.WriteFile(jsonPath(root), jsonData, 0600); err != nil {
			return err
		}
	}
	return nil
}

func (img *Image) SetGraph(graph Graph) {
	img.graph = graph
}

// SaveSize stores the current `size` value of `img` in the directory `root`.
func (img *Image) SaveSize(root string) error {
	if err := ioutil.WriteFile(path.Join(root, "layersize"), []byte(strconv.Itoa(int(img.Size))), 0600); err != nil {
		return fmt.Errorf("Error storing image size in %s/layersize: %s", root, err)
	}
	return nil
}

func jsonPath(root string) string {
	return path.Join(root, "json")
}

func (img *Image) RawJson() ([]byte, error) {
	root, err := img.root()
	if err != nil {
		return nil, fmt.Errorf("Failed to get root for image %s: %s", img.ID, err)
	}
	fh, err := os.Open(jsonPath(root))
	if err != nil {
		return nil, fmt.Errorf("Failed to open json for image %s: %s", img.ID, err)
	}
	buf, err := ioutil.ReadAll(fh)
	if err != nil {
		return nil, fmt.Errorf("Failed to read json for image %s: %s", img.ID, err)
	}
	return buf, nil
}

// TarLayer returns a tar archive of the image's filesystem layer.
func (img *Image) TarLayer() (arch archive.Archive, err error) {
	if img.graph == nil {
		return nil, fmt.Errorf("Can't load storage driver for unregistered image %s", img.ID)
	}
	driver := img.graph.Driver()
	if differ, ok := driver.(graphdriver.Differ); ok {
		return differ.Diff(img.ID)
	}

	imgFs, err := driver.Get(img.ID, "")
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			driver.Put(img.ID)
		}
	}()

	if img.Parent == "" {
		archive, err := archive.Tar(imgFs, archive.Uncompressed)
		if err != nil {
			return nil, err
		}
		return utils.NewReadCloserWrapper(archive, func() error {
			err := archive.Close()
			driver.Put(img.ID)
			return err
		}), nil
	}

	parentFs, err := driver.Get(img.Parent, "")
	if err != nil {
		return nil, err
	}
	defer driver.Put(img.Parent)
	changes, err := archive.ChangesDirs(imgFs, parentFs)
	if err != nil {
		return nil, err
	}
	archive, err := archive.ExportChanges(imgFs, changes)
	if err != nil {
		return nil, err
	}
	return utils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		driver.Put(img.ID)
		return err
	}), nil
}

// Image includes convenience proxy functions to its graph
// These functions will return an error if the image is not registered
// (ie. if image.graph == nil)
func (img *Image) History() ([]*Image, error) {
	var parents []*Image
	if err := img.WalkHistory(
		func(img *Image) error {
			parents = append(parents, img)
			return nil
		},
	); err != nil {
		return nil, err
	}
	return parents, nil
}

func (img *Image) WalkHistory(handler func(*Image) error) (err error) {
	currentImg := img
	for currentImg != nil {
		if handler != nil {
			if err := handler(currentImg); err != nil {
				return err
			}
		}
		currentImg, err = currentImg.GetParent()
		if err != nil {
			return fmt.Errorf("Error while getting parent image: %v", err)
		}
	}
	return nil
}

func (img *Image) GetParent() (*Image, error) {
	if img.Parent == "" {
		return nil, nil
	}
	if img.graph == nil {
		return nil, fmt.Errorf("Can't lookup parent of unregistered image")
	}
	return img.graph.Get(img.Parent)
}

func (img *Image) root() (string, error) {
	if img.graph == nil {
		return "", fmt.Errorf("Can't lookup root of unregistered image")
	}
	return img.graph.ImageRoot(img.ID), nil
}

func (img *Image) GetParentsSize(size int64) int64 {
	parentImage, err := img.GetParent()
	if err != nil || parentImage == nil {
		return size
	}
	size += parentImage.Size
	return parentImage.GetParentsSize(size)
}

// Depth returns the number of parents for a
// current image
func (img *Image) Depth() (int, error) {
	var (
		count  = 0
		parent = img
		err    error
	)

	for parent != nil {
		count++
		parent, err = parent.GetParent()
		if err != nil {
			return -1, err
		}
	}
	return count, nil
}

// CheckDepth returns an error if the depth of an image, as returned
// by ImageDepth, is too large to support creating a container from it
// on this daemon.
func (img *Image) CheckDepth() error {
	// We add 2 layers to the depth because the container's rw and
	// init layer add to the restriction
	depth, err := img.Depth()
	if err != nil {
		return err
	}
	if depth+2 >= MaxImageDepth {
		return fmt.Errorf("Cannot create container with more than %d parents", MaxImageDepth)
	}
	return nil
}

// Build an Image object from raw json data
func NewImgJSON(src []byte) (*Image, error) {
	ret := &Image{}

	log.Debugf("Json string: {%s}", src)
	// FIXME: Is there a cleaner way to "purify" the input json?
	if err := json.Unmarshal(src, ret); err != nil {
		return nil, err
	}
	return ret, nil
}
