package image

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/tarsum"
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
	Checksum        string            `json:"checksum"`
	Size            int64

	graph Graph
}

func LoadImage(root string) (*Image, error) {
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

// StoreImage stores file system layer data for the given image to the
// image's registered storage driver. Image metadata is stored in a file
// at the specified root directory. This function also computes the TarSum
// of `layerData` (currently using tarsum.dev).
func StoreImage(img *Image, layerData archive.ArchiveReader, root string) error {
	// Store the layer
	var (
		size        int64
		err         error
		driver      = img.graph.Driver()
		layerTarSum tarsum.TarSum
	)

	// If layerData is not nil, unpack it into the new layer
	if layerData != nil {
		layerDataDecompressed, err := archive.DecompressStream(layerData)
		if err != nil {
			return err
		}

		defer layerDataDecompressed.Close()

		if layerTarSum, err = tarsum.NewTarSum(layerDataDecompressed, true, tarsum.VersionDev); err != nil {
			return err
		}

		if size, err = driver.ApplyDiff(img.ID, img.Parent, layerTarSum); err != nil {
			return err
		}

		checksum := layerTarSum.Sum(nil)

		if img.Checksum != "" && img.Checksum != checksum {
			log.Warnf("image layer checksum mismatch: computed %q, expected %q", checksum, img.Checksum)
		}

		img.Checksum = checksum
	}

	img.Size = size
	if err := img.SaveSize(root); err != nil {
		return err
	}

	f, err := os.OpenFile(jsonPath(root), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(0600))
	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewEncoder(f).Encode(img)
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

	return driver.Diff(img.ID, img.Parent)
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
