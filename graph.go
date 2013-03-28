package docker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type Graph struct {
	Root string
}

func NewGraph(root string) (*Graph, error) {
	abspath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	// Create the root directory if it doesn't exists
	if err := os.Mkdir(root, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	return &Graph{
		Root: abspath,
	}, nil
}

// FIXME: Implement error subclass instead of looking at the error text
// Note: This is the way golang implements os.IsNotExists on Plan9
func (graph *Graph) IsNotExist(err error) bool {
	return err != nil && strings.Contains(err.Error(), "does not exist")
}

func (graph *Graph) Exists(id string) bool {
	if _, err := graph.Get(id); err != nil {
		return false
	}
	return true
}

func (graph *Graph) Get(id string) (*Image, error) {
	// FIXME: return nil when the image doesn't exist, instead of an error
	img, err := LoadImage(graph.imageRoot(id))
	if err != nil {
		return nil, err
	}
	if img.Id != id {
		return nil, fmt.Errorf("Image stored at '%s' has wrong id '%s'", id, img.Id)
	}
	img.graph = graph
	return img, nil
}

func (graph *Graph) Create(layerData Archive, container *Container, comment string) (*Image, error) {
	img := &Image{
		Id:      GenerateId(),
		Comment: comment,
		Created: time.Now(),
	}
	if container != nil {
		img.Parent = container.Image
		img.Container = container.Id
		img.ContainerConfig = *container.Config
	}
	if err := graph.Register(layerData, img); err != nil {
		return nil, err
	}
	return img, nil
}

func (graph *Graph) Register(layerData Archive, img *Image) error {
	if err := ValidateId(img.Id); err != nil {
		return err
	}
	// (This is a convenience to save time. Race conditions are taken care of by os.Rename)
	if graph.Exists(img.Id) {
		return fmt.Errorf("Image %s already exists", img.Id)
	}
	tmp, err := graph.Mktemp(img.Id)
	defer os.RemoveAll(tmp)
	if err != nil {
		return fmt.Errorf("Mktemp failed: %s", err)
	}
	if err := StoreImage(img, layerData, tmp); err != nil {
		return err
	}
	// Commit
	if err := os.Rename(tmp, graph.imageRoot(img.Id)); err != nil {
		return err
	}
	img.graph = graph
	return nil
}

func (graph *Graph) Mktemp(id string) (string, error) {
	tmp, err := NewGraph(path.Join(graph.Root, ":tmp:"))
	if err != nil {
		return "", fmt.Errorf("Couldn't create temp: %s", err)
	}
	if tmp.Exists(id) {
		return "", fmt.Errorf("Image %d already exists", id)
	}
	return tmp.imageRoot(id), nil
}

func (graph *Graph) Garbage() (*Graph, error) {
	return NewGraph(path.Join(graph.Root, ":garbage:"))
}

// Check if given error is "not empty"
// Note: this is the way golang do it internally with os.IsNotExists
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

func (graph *Graph) Delete(id string) error {
	garbage, err := graph.Garbage()
	if err != nil {
		return err
	}
	err = os.Rename(graph.imageRoot(id), garbage.imageRoot(id))
	if err != nil {
		if isNotEmpty(err) {
			Debugf("The image %s is already present in garbage. Removing it.", id)
			if err = os.RemoveAll(garbage.imageRoot(id)); err != nil {
				Debugf("Error while removing the image %s from garbage: %s\n", id, err)
				return err
			}
			Debugf("Image %s removed from garbage", id)
			if err = os.Rename(graph.imageRoot(id), garbage.imageRoot(id)); err != nil {
				return err
			}
			Debugf("Image %s put in the garbage", id)
		} else {
			Debugf("Error putting the image %s to garbage: %s\n", id, err)
		}
		return err
	}
	return nil
}

func (graph *Graph) Undelete(id string) error {
	garbage, err := graph.Garbage()
	if err != nil {
		return err
	}
	return os.Rename(garbage.imageRoot(id), graph.imageRoot(id))
}

func (graph *Graph) GarbageCollect() error {
	garbage, err := graph.Garbage()
	if err != nil {
		return err
	}
	return os.RemoveAll(garbage.Root)
}

func (graph *Graph) Map() (map[string]*Image, error) {
	// FIXME: this should replace All()
	all, err := graph.All()
	if err != nil {
		return nil, err
	}
	images := make(map[string]*Image, len(all))
	for _, image := range all {
		images[image.Id] = image
	}
	return images, nil
}

func (graph *Graph) All() ([]*Image, error) {
	var images []*Image
	err := graph.WalkAll(func(image *Image) {
		images = append(images, image)
	})
	return images, err
}

func (graph *Graph) WalkAll(handler func(*Image)) error {
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

func (graph *Graph) ByParent() (map[string][]*Image, error) {
	byParent := make(map[string][]*Image)
	err := graph.WalkAll(func(image *Image) {
		image, err := graph.Get(image.Parent)
		if err != nil {
			return
		}
		if children, exists := byParent[image.Parent]; exists {
			byParent[image.Parent] = []*Image{image}
		} else {
			byParent[image.Parent] = append(children, image)
		}
	})
	return byParent, err
}

func (graph *Graph) Heads() (map[string]*Image, error) {
	heads := make(map[string]*Image)
	byParent, err := graph.ByParent()
	if err != nil {
		return nil, err
	}
	err = graph.WalkAll(func(image *Image) {
		// If it's not in the byParent lookup table, then
		// it's not a parent -> so it's a head!
		if _, exists := byParent[image.Id]; !exists {
			heads[image.Id] = image
		}
	})
	return heads, err
}

func (graph *Graph) imageRoot(id string) string {
	return path.Join(graph.Root, id)
}
