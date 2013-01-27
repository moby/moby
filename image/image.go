package image

import (
	"io"
	"io/ioutil"
	"encoding/json"
	"time"
	"path"
	"path/filepath"
	"errors"
	"sort"
	"os"
	"github.com/dotcloud/docker/future"
	"strings"
)


type Store struct {
	*Index
	Root	string
	Layers	*LayerStore
}


func New(root string) (*Store, error) {
	abspath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	layers, err := NewLayerStore(path.Join(root, "layers"))
	if err != nil {
		return nil, err
	}
	if err := layers.Init(); err != nil {
		return nil, err
	}
	return &Store{
		Root: abspath,
		Index: NewIndex(path.Join(root, "index.json")),
		Layers: layers,
	}, nil
}


func (store *Store) Import(name string, archive io.Reader, stderr io.Writer, parent *Image) (*Image, error) {
	layer, err := store.Layers.AddLayer(archive, stderr)
	if err != nil {
		return nil, err
	}
	layers := []string{layer}
	if parent != nil {
		layers = append(parent.Layers, layers...)
	}
	var parentId string
	if parent != nil {
		parentId = parent.Id
	}
	return store.Create(name, parentId, layers...)
}

func (store *Store) Create(name string, source string, layers ...string) (*Image, error) {
	image, err := NewImage(name, layers, source)
	if err != nil {
		return nil, err
	}
	if err := store.Index.Add(name, image); err != nil {
		return nil, err
	}
	return image, nil
}


// Index

type Index struct {
	Path	string
	ByName	map[string]*History
	ById	map[string]*Image
}

func NewIndex(path string) *Index {
	return &Index{
		Path: path,
		ByName: make(map[string]*History),
		ById: make(map[string]*Image),
	}
}

func (index *Index) Exists(id string) bool {
	_, exists := index.ById[id]
	return exists
}

func (index *Index) Find(idOrName string) *Image {
	// Load
	if err := index.load(); err != nil {
		return nil
	}
	// Lookup by ID
	if image, exists := index.ById[idOrName]; exists {
		return image
	}
	// Lookup by name
	if history, exists := index.ByName[idOrName]; exists && history.Len() > 0 {
		return (*history)[0]
	}
	return nil
}

func (index *Index) Add(name string, image *Image) error {
	// Load
	if err := index.load(); err != nil {
		return err
	}
	if _, exists := index.ByName[name]; !exists {
		index.ByName[name] = new(History)
	} else {
		// If this image is already the latest version, don't add it.
		if (*index.ByName[name])[0].Id == image.Id {
			return nil
		}
	}
	index.ByName[name].Add(image)
	index.ById[image.Id] = image
	// Save
	if err := index.save(); err != nil {
		return err
	}
	return nil
}

func (index *Index) Copy(srcNameOrId, dstName string) (*Image, error) {
	if srcNameOrId == "" || dstName == "" {
		return nil, errors.New("Illegal image name")
	}
	// Load
	if err := index.load(); err != nil {
		return nil, err
	}
	src := index.Find(srcNameOrId)
	if src == nil {
		return nil, errors.New("No such image: " + srcNameOrId)
	}
	if index.Find(dstName) != nil {
		return nil, errors.New(dstName + ": image already exists.")
	}
	dst, err := NewImage(dstName, src.Layers, src.Id)
	if err != nil {
		return nil, err
	}
	if err := index.Add(dstName, dst); err != nil {
		return nil, err
	}
	// Save
	if err := index.save(); err != nil {
		return nil, err
	}
	return dst, nil
}

func (index *Index) Rename(oldName, newName string) error {
	// Load
	if err := index.load(); err != nil {
		return err
	}
	if _, exists := index.ByName[oldName]; !exists {
		return errors.New("Can't rename " + oldName + ": no such image.")
	}
	if _, exists := index.ByName[newName]; exists {
		return errors.New("Can't rename to " + newName + ": name is already in use.")
	}
	index.ByName[newName] = index.ByName[oldName]
	delete(index.ByName, oldName)
	// Change the ID of all images, since they include the name
	for _, image := range *index.ByName[newName] {
		if id, err := generateImageId(newName, image.Layers); err != nil {
			return err
		} else {
			oldId := image.Id
			image.Id = id
			index.ById[id] = image
			delete(index.ById, oldId)
		}
	}
	// Save
	if err := index.save(); err != nil {
		return err
	}
	return nil
}

func (index *Index) Names() []string {
	if err := index.load(); err != nil {
		return []string{}
	}
	var names[]string
	for name := range index.ByName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (index *Index) load() error {
	jsonData, err := ioutil.ReadFile(index.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	path := index.Path
	if err := json.Unmarshal(jsonData, index); err != nil {
		return err
	}
	index.Path = path
	return nil
}

func (index *Index) save() error {
	jsonData, err := json.Marshal(index)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(index.Path, jsonData, 0600); err != nil {
		return err
	}
	return nil
}

// History wraps an array of images so they can be sorted by date (most recent first)

type History []*Image

func (history *History) Len() int {
	return len(*history)
}

func (history *History) Less(i, j int) bool {
	images := *history
	return images[j].Created.Before(images[i].Created)
}

func (history *History) Swap(i, j int) {
	images := *history
	tmp := images[i]
	images[i] = images[j]
	images[j] = tmp
}

func (history *History) Add(image *Image) {
	*history = append(*history, image)
	sort.Sort(history)
}

func (history *History) Del(id string) {
	for idx, image := range *history {
		if image.Id == id {
			*history = append((*history)[:idx], (*history)[idx + 1:]...)
		}
	}
}

type Image struct {
	Id	string		// Globally unique identifier
	Layers	[]string	// Absolute paths
	Created	time.Time
	Parent	string
}

func (image *Image) IdParts() (string, string) {
	if len(image.Id) < 8 {
		return "", image.Id
	}
	hash := image.Id[len(image.Id)-8:len(image.Id)]
	name := image.Id[:len(image.Id)-9]
	return name, hash
}

func (image *Image) IdIsFinal() bool {
	return len(image.Layers) == 1
}

func generateImageId(name string, layers []string) (string, error) {
	if len(layers) == 0 {
		return "", errors.New("No layers provided.")
	}
	var hash string
	if len(layers) == 1 {
		hash = path.Base(layers[0])
	} else {
		var ids string
		for _, layer := range layers {
			ids += path.Base(layer)
		}
		if h, err := future.ComputeId(strings.NewReader(ids)); err != nil  {
			return "", err
		} else {
			hash = h
		}
	}
	return name + ":" + hash, nil
}

func NewImage(name string, layers []string, parent string) (*Image, error) {
	id, err := generateImageId(name, layers)
	if err != nil {
		return nil, err
	}
	return &Image{
		Id:		id,
		Layers:		layers,
		Created:	time.Now(),
		Parent:		parent,
	}, nil
}
