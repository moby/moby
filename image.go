package docker

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Image struct {
	ID              string    `json:"id"`
	Parent          string    `json:"parent,omitempty"`
	Comment         string    `json:"comment,omitempty"`
	Created         time.Time `json:"created"`
	Container       string    `json:"container,omitempty"`
	ContainerConfig Config    `json:"container_config,omitempty"`
	DockerVersion   string    `json:"docker_version,omitempty"`
	Author          string    `json:"author,omitempty"`
	Config          *Config   `json:"config,omitempty"`
	Architecture    string    `json:"architecture,omitempty"`
	graph           *Graph
	Size            int64
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
	if err := ValidateID(img.ID); err != nil {
		return nil, err
	}

	if buf, err := ioutil.ReadFile(path.Join(root, "layersize")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		if size, err := strconv.Atoi(string(buf)); err != nil {
			return nil, err
		} else {
			img.Size = int64(size)
		}
	}

	// Check that the filesystem layer exists
	if stat, err := os.Stat(layerPath(root)); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Couldn't load image %s: no filesystem layer", img.ID)
		}
		return nil, err
	} else if !stat.IsDir() {
		return nil, fmt.Errorf("Couldn't load image %s: %s is not a directory", img.ID, layerPath(root))
	}
	return img, nil
}

func StoreImage(img *Image, jsonData []byte, layerData Archive, root string) error {
	// Check that root doesn't already exist
	if _, err := os.Stat(root); err == nil {
		return fmt.Errorf("Image %s already exists", img.ID)
	} else if !os.IsNotExist(err) {
		return err
	}
	// Store the layer
	layer := layerPath(root)
	if err := os.MkdirAll(layer, 0755); err != nil {
		return err
	}

	// If layerData is not nil, unpack it into the new layer
	if layerData != nil {
		start := time.Now()
		utils.Debugf("Start untar layer")
		if err := Untar(layerData, layer); err != nil {
			return err
		}
		utils.Debugf("Untar time: %vs", time.Now().Sub(start).Seconds())
	}

	// If raw json is provided, then use it
	if jsonData != nil {
		return ioutil.WriteFile(jsonPath(root), jsonData, 0600)
	} else { // Otherwise, unmarshal the image
		jsonData, err := json.Marshal(img)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(jsonPath(root), jsonData, 0600); err != nil {
			return err
		}
	}

	return StoreSize(img, root)
}

func StoreSize(img *Image, root string) error {
	layer := layerPath(root)

	var totalSize int64 = 0
	filepath.Walk(layer, func(path string, fileInfo os.FileInfo, err error) error {
		totalSize += fileInfo.Size()
		return nil
	})
	img.Size = totalSize

	if err := ioutil.WriteFile(path.Join(root, "layersize"), []byte(strconv.Itoa(int(totalSize))), 0600); err != nil {
		return nil
	}

	return nil
}

func layerPath(root string) string {
	return path.Join(root, "layer")
}

func jsonPath(root string) string {
	return path.Join(root, "json")
}

func MountAUFS(ro []string, rw string, target string) error {
	// FIXME: Now mount the layers
	rwBranch := fmt.Sprintf("%v=rw", rw)
	roBranches := ""
	for _, layer := range ro {
		roBranches += fmt.Sprintf("%v=ro+wh:", layer)
	}
	branches := fmt.Sprintf("br:%v:%v", rwBranch, roBranches)

	branches += ",xino=/dev/shm/aufs.xino"

	//if error, try to load aufs kernel module
	if err := mount("none", target, "aufs", 0, branches); err != nil {
		log.Printf("Kernel does not support AUFS, trying to load the AUFS module with modprobe...")
		if err := exec.Command("modprobe", "aufs").Run(); err != nil {
			return fmt.Errorf("Unable to load the AUFS module")
		}
		log.Printf("...module loaded.")
		if err := mount("none", target, "aufs", 0, branches); err != nil {
			return fmt.Errorf("Unable to mount using aufs")
		}
	}
	return nil
}

// TarLayer returns a tar archive of the image's filesystem layer.
func (image *Image) TarLayer(compression Compression) (Archive, error) {
	layerPath, err := image.layer()
	if err != nil {
		return nil, err
	}
	return Tar(layerPath, compression)
}

func (image *Image) Mount(root, rw string) error {
	if mounted, err := Mounted(root); err != nil {
		return err
	} else if mounted {
		return fmt.Errorf("%s is already mounted", root)
	}
	layers, err := image.layers()
	if err != nil {
		return err
	}
	// Create the target directories if they don't exist
	if err := os.Mkdir(root, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := os.Mkdir(rw, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	if err := MountAUFS(layers, rw, root); err != nil {
		return err
	}
	return nil
}

func (image *Image) Changes(rw string) ([]Change, error) {
	layers, err := image.layers()
	if err != nil {
		return nil, err
	}
	return Changes(layers, rw)
}

func (image *Image) ShortID() string {
	return utils.TruncateID(image.ID)
}

func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("Image id can't be empty")
	}
	if strings.Contains(id, ":") {
		return fmt.Errorf("Invalid character in image id: ':'")
	}
	return nil
}

func GenerateID() string {
	id := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, id)
	if err != nil {
		panic(err) // This shouldn't happen
	}
	return hex.EncodeToString(id)
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

// layers returns all the filesystem layers needed to mount an image
// FIXME: @shykes refactor this function with the new error handling
//        (I'll do it if I have time tonight, I focus on the rest)
func (img *Image) layers() ([]string, error) {
	var list []string
	var e error
	if err := img.WalkHistory(
		func(img *Image) (err error) {
			if layer, err := img.layer(); err != nil {
				e = err
			} else if layer != "" {
				list = append(list, layer)
			}
			return err
		},
	); err != nil {
		return nil, err
	} else if e != nil { // Did an error occur inside the handler?
		return nil, e
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("No layer found for image %s\n", img.ID)
	}

	// Inject the dockerinit layer (empty place-holder for mount-binding dockerinit)
	if dockerinitLayer, err := img.getDockerInitLayer(); err != nil {
		return nil, err
	} else {
		list = append([]string{dockerinitLayer}, list...)
	}
	return list, nil
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

func (img *Image) getDockerInitLayer() (string, error) {
	if img.graph == nil {
		return "", fmt.Errorf("Can't lookup dockerinit layer of unregistered image")
	}
	return img.graph.getDockerInitLayer()
}

func (img *Image) root() (string, error) {
	if img.graph == nil {
		return "", fmt.Errorf("Can't lookup root of unregistered image")
	}
	return img.graph.imageRoot(img.ID), nil
}

// Return the path of an image's layer
func (img *Image) layer() (string, error) {
	root, err := img.root()
	if err != nil {
		return "", err
	}
	return layerPath(root), nil
}

func (img *Image) getParentsSize(size int64) int64 {
	parentImage, err := img.GetParent()
	if err != nil || parentImage == nil {
		return size
	}
	size += parentImage.Size
	return parentImage.getParentsSize(size)
}

// Build an Image object from raw json data
func NewImgJSON(src []byte) (*Image, error) {
	ret := &Image{}

	utils.Debugf("Json string: {%s}", src)
	// FIXME: Is there a cleaner way to "purify" the input json?
	if err := json.Unmarshal(src, ret); err != nil {
		return nil, err
	}
	return ret, nil
}
