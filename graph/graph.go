package graph

import (
	"compress/gzip"
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
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/truncindex"
	"github.com/docker/docker/runconfig"
	"github.com/vbatts/tar-split/tar/asm"
	"github.com/vbatts/tar-split/tar/storage"
)

// v1Descriptor is a non-content-addressable image descriptor
type v1Descriptor struct {
	img *image.Image
}

// ID returns the image ID specified in the image structure.
func (img v1Descriptor) ID() string {
	return img.img.ID
}

// Parent returns the parent ID specified in the image structure.
func (img v1Descriptor) Parent() string {
	return img.img.Parent
}

// MarshalConfig renders the image structure into JSON.
func (img v1Descriptor) MarshalConfig() ([]byte, error) {
	return json.Marshal(img.img)
}

// The type is used to protect pulling or building related image
// layers from deleteing when filtered by dangling=true
// The key of layers is the images ID which is pulling or building
// The value of layers is a slice which hold layer IDs referenced to
// pulling or building images
type retainedLayers struct {
	layerHolders map[string]map[string]struct{} // map[layerID]map[sessionID]
	sync.Mutex
}

func (r *retainedLayers) Add(sessionID string, layerIDs []string) {
	r.Lock()
	defer r.Unlock()
	for _, layerID := range layerIDs {
		if r.layerHolders[layerID] == nil {
			r.layerHolders[layerID] = map[string]struct{}{}
		}
		r.layerHolders[layerID][sessionID] = struct{}{}
	}
}

func (r *retainedLayers) Delete(sessionID string, layerIDs []string) {
	r.Lock()
	defer r.Unlock()
	for _, layerID := range layerIDs {
		holders, ok := r.layerHolders[layerID]
		if !ok {
			continue
		}
		delete(holders, sessionID)
		if len(holders) == 0 {
			delete(r.layerHolders, layerID) // Delete any empty reference set.
		}
	}
}

func (r *retainedLayers) Exists(layerID string) bool {
	r.Lock()
	_, exists := r.layerHolders[layerID]
	r.Unlock()
	return exists
}

// A Graph is a store for versioned filesystem images and the relationship between them.
type Graph struct {
	root             string
	idIndex          *truncindex.TruncIndex
	driver           graphdriver.Driver
	imagesMutex      sync.Mutex
	imageMutex       imageMutex // protect images in driver.
	retained         *retainedLayers
	tarSplitDisabled bool
	uidMaps          []idtools.IDMap
	gidMaps          []idtools.IDMap

	// access to parentRefs must be protected with imageMutex locking the image id
	// on the key of the map (e.g. imageMutex.Lock(img.ID), parentRefs[img.ID]...)
	parentRefs map[string]int
}

// file names for ./graph/<ID>/
const (
	jsonFileName            = "json"
	layersizeFileName       = "layersize"
	digestFileName          = "checksum"
	tarDataFileName         = "tar-data.json.gz"
	v1CompatibilityFileName = "v1Compatibility"
	parentFileName          = "parent"
)

var (
	// ErrDigestNotSet is used when request the digest for a layer
	// but the layer has no digest value or content to compute the
	// the digest.
	ErrDigestNotSet = errors.New("digest is not set for layer")
)

// NewGraph instantiates a new graph at the given root path in the filesystem.
// `root` will be created if it doesn't exist.
func NewGraph(root string, driver graphdriver.Driver, uidMaps, gidMaps []idtools.IDMap) (*Graph, error) {
	abspath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	rootUID, rootGID, err := idtools.GetRootUIDGID(uidMaps, gidMaps)
	if err != nil {
		return nil, err
	}
	// Create the root directory if it doesn't exists
	if err := idtools.MkdirAllAs(root, 0700, rootUID, rootGID); err != nil && !os.IsExist(err) {
		return nil, err
	}

	graph := &Graph{
		root:       abspath,
		idIndex:    truncindex.NewTruncIndex([]string{}),
		driver:     driver,
		retained:   &retainedLayers{layerHolders: make(map[string]map[string]struct{})},
		uidMaps:    uidMaps,
		gidMaps:    gidMaps,
		parentRefs: make(map[string]int),
	}

	// Windows does not currently support tarsplit functionality.
	if runtime.GOOS == "windows" {
		graph.tarSplitDisabled = true
	}

	if err := graph.restore(); err != nil {
		return nil, err
	}
	return graph, nil
}

// IsHeld returns whether the given layerID is being used by an ongoing pull or build.
func (graph *Graph) IsHeld(layerID string) bool {
	return graph.retained.Exists(layerID)
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
			img, err := graph.loadImage(id)
			if err != nil {
				if err != io.EOF {
					return fmt.Errorf("could not restore image %s: %v", id, err)
				}
				logrus.Warnf("could not restore image %s due to corrupted files", id)
				continue
			}
			graph.imageMutex.Lock(img.Parent)
			graph.parentRefs[img.Parent]++
			graph.imageMutex.Unlock(img.Parent)
			ids = append(ids, id)
		}
	}

	graph.idIndex = truncindex.NewTruncIndex(ids)
	logrus.Debugf("Restored %d elements", len(ids))
	return nil
}

// IsNotExist detects whether an image exists by parsing the incoming error
// message.
func (graph *Graph) IsNotExist(err error, id string) bool {
	// FIXME: Implement error subclass instead of looking at the error text
	// Note: This is the way golang implements os.IsNotExists on Plan9
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
func (graph *Graph) Get(name string) (*image.Image, error) {
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
		if err := graph.saveSize(graph.imageRoot(id), img.Size); err != nil {
			return nil, err
		}
	}
	return img, nil
}

// Create creates a new image and registers it in the graph.
func (graph *Graph) Create(layerData io.Reader, containerID, containerImage, comment, author string, containerConfig, config *runconfig.Config) (*image.Image, error) {
	img := &image.Image{
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

	if err := graph.Register(v1Descriptor{img}, layerData); err != nil {
		return nil, err
	}
	return img, nil
}

// Register imports a pre-existing image into the graph.
// Returns nil if the image is already registered.
func (graph *Graph) Register(im image.Descriptor, layerData io.Reader) (err error) {
	imgID := im.ID()

	if err := image.ValidateID(imgID); err != nil {
		return err
	}

	// this is needed cause pull_v2 attemptIDReuse could deadlock
	graph.imagesMutex.Lock()
	defer graph.imagesMutex.Unlock()

	// We need this entire operation to be atomic within the engine. Note that
	// this doesn't mean Register is fully safe yet.
	graph.imageMutex.Lock(imgID)
	defer graph.imageMutex.Unlock(imgID)

	return graph.register(im, layerData)
}

func (graph *Graph) register(im image.Descriptor, layerData io.Reader) (err error) {
	imgID := im.ID()

	// Skip register if image is already registered
	if graph.Exists(imgID) {
		return nil
	}

	// The returned `error` must be named in this function's signature so that
	// `err` is not shadowed in this deferred cleanup.
	defer func() {
		// If any error occurs, remove the new dir from the driver.
		// Don't check for errors since the dir might not have been created.
		if err != nil {
			graph.driver.Remove(imgID)
		}
	}()

	// Ensure that the image root does not exist on the filesystem
	// when it is not registered in the graph.
	// This is common when you switch from one graph driver to another
	if err := os.RemoveAll(graph.imageRoot(imgID)); err != nil && !os.IsNotExist(err) {
		return err
	}

	// If the driver has this ID but the graph doesn't, remove it from the driver to start fresh.
	// (the graph is the source of truth).
	// Ignore errors, since we don't know if the driver correctly returns ErrNotExist.
	// (FIXME: make that mandatory for drivers).
	graph.driver.Remove(imgID)

	tmp, err := graph.mktemp()
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	parent := im.Parent()

	// Create root filesystem in the driver
	if err := createRootFilesystemInDriver(graph, imgID, parent, layerData); err != nil {
		return err
	}

	// Apply the diff/layer
	config, err := im.MarshalConfig()
	if err != nil {
		return err
	}
	if err := graph.storeImage(imgID, parent, config, layerData, tmp); err != nil {
		return err
	}
	// Commit
	if err := os.Rename(tmp, graph.imageRoot(imgID)); err != nil {
		return err
	}

	graph.idIndex.Add(imgID)

	graph.imageMutex.Lock(parent)
	graph.parentRefs[parent]++
	graph.imageMutex.Unlock(parent)

	return nil
}

func createRootFilesystemInDriver(graph *Graph, id, parent string, layerData io.Reader) error {
	if err := graph.driver.Create(id, parent); err != nil {
		return fmt.Errorf("Driver %s failed to create image rootfs %s: %s", graph.driver, id, err)
	}
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
	tmp, err := graph.mktemp()
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
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
func (graph *Graph) mktemp() (string, error) {
	dir := filepath.Join(graph.root, "_tmp", stringid.GenerateNonCryptoID())
	rootUID, rootGID, err := idtools.GetRootUIDGID(graph.uidMaps, graph.gidMaps)
	if err != nil {
		return "", err
	}
	if err := idtools.MkdirAllAs(dir, 0700, rootUID, rootGID); err != nil {
		return "", err
	}
	return dir, nil
}

// Delete atomically removes an image from the graph.
func (graph *Graph) Delete(name string) error {
	id, err := graph.idIndex.Get(name)
	if err != nil {
		return err
	}
	img, err := graph.Get(id)
	if err != nil {
		return err
	}
	graph.idIndex.Delete(id)
	tmp, err := graph.mktemp()
	if err != nil {
		tmp = graph.imageRoot(id)
	} else {
		if err := os.Rename(graph.imageRoot(id), tmp); err != nil {
			// On err make tmp point to old dir and cleanup unused tmp dir
			os.RemoveAll(tmp)
			tmp = graph.imageRoot(id)
		}
	}
	// Remove rootfs data from the driver
	graph.driver.Remove(id)

	graph.imageMutex.Lock(img.Parent)
	graph.parentRefs[img.Parent]--
	if graph.parentRefs[img.Parent] == 0 {
		delete(graph.parentRefs, img.Parent)
	}
	graph.imageMutex.Unlock(img.Parent)

	// Remove the trashed image directory
	return os.RemoveAll(tmp)
}

// Map returns a list of all images in the graph, addressable by ID.
func (graph *Graph) Map() map[string]*image.Image {
	images := make(map[string]*image.Image)
	graph.walkAll(func(image *image.Image) {
		images[image.ID] = image
	})
	return images
}

// walkAll iterates over each image in the graph, and passes it to a handler.
// The walking order is undetermined.
func (graph *Graph) walkAll(handler func(*image.Image)) {
	graph.idIndex.Iterate(func(id string) {
		img, err := graph.Get(id)
		if err != nil {
			return
		}
		if handler != nil {
			handler(img)
		}
	})
}

// ByParent returns a lookup table of images by their parent.
// If an image of key ID has 3 children images, then the value for key ID
// will be a list of 3 images.
// If an image has no children, it will not have an entry in the table.
func (graph *Graph) ByParent() map[string][]*image.Image {
	byParent := make(map[string][]*image.Image)
	graph.walkAll(func(img *image.Image) {
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
	return byParent
}

// HasChildren returns whether the given image has any child images.
func (graph *Graph) HasChildren(imgID string) bool {
	graph.imageMutex.Lock(imgID)
	count := graph.parentRefs[imgID]
	graph.imageMutex.Unlock(imgID)
	return count > 0
}

// Retain keeps the images and layers that are in the pulling chain so that
// they are not deleted. If not retained, they may be deleted by rmi.
func (graph *Graph) Retain(sessionID string, layerIDs ...string) {
	graph.retained.Add(sessionID, layerIDs)
}

// Release removes the referenced image ID from the provided set of layers.
func (graph *Graph) Release(sessionID string, layerIDs ...string) {
	graph.retained.Delete(sessionID, layerIDs)
}

// Heads returns all heads in the graph, keyed by id.
// A head is an image which is not the parent of another image in the graph.
func (graph *Graph) Heads() map[string]*image.Image {
	heads := make(map[string]*image.Image)
	graph.walkAll(func(image *image.Image) {
		// if it has no children, then it's not a parent, so it's an head
		if !graph.HasChildren(image.ID) {
			heads[image.ID] = image
		}
	})
	return heads
}

// TarLayer returns a tar archive of the image's filesystem layer.
func (graph *Graph) TarLayer(img *image.Image) (arch io.ReadCloser, err error) {
	rdr, err := graph.assembleTarLayer(img)
	if err != nil {
		logrus.Debugf("[graph] TarLayer with traditional differ: %s", img.ID)
		return graph.driver.Diff(img.ID, img.Parent)
	}
	return rdr, nil
}

func (graph *Graph) imageRoot(id string) string {
	return filepath.Join(graph.root, id)
}

// loadImage fetches the image with the given id from the graph.
func (graph *Graph) loadImage(id string) (*image.Image, error) {
	root := graph.imageRoot(id)

	// Open the JSON file to decode by streaming
	jsonSource, err := os.Open(jsonPath(root))
	if err != nil {
		return nil, err
	}
	defer jsonSource.Close()

	img := &image.Image{}
	dec := json.NewDecoder(jsonSource)

	// Decode the JSON data
	if err := dec.Decode(img); err != nil {
		return nil, err
	}

	if img.ID == "" {
		img.ID = id
	}

	if img.Parent == "" && img.ParentID != "" && img.ParentID.Validate() == nil {
		img.Parent = img.ParentID.Hex()
	}

	// compatibilityID for parent
	parent, err := ioutil.ReadFile(filepath.Join(root, parentFileName))
	if err == nil && len(parent) > 0 {
		img.Parent = string(parent)
	}

	if err := image.ValidateID(img.ID); err != nil {
		return nil, err
	}

	if buf, err := ioutil.ReadFile(filepath.Join(root, layersizeFileName)); err != nil {
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
func (graph *Graph) saveSize(root string, size int64) error {
	if err := ioutil.WriteFile(filepath.Join(root, layersizeFileName), []byte(strconv.FormatInt(size, 10)), 0600); err != nil {
		return fmt.Errorf("Error storing image size in %s/%s: %s", root, layersizeFileName, err)
	}
	return nil
}

// SetLayerDigest sets the digest for the image layer to the provided value.
func (graph *Graph) SetLayerDigest(id string, dgst digest.Digest) error {
	graph.imageMutex.Lock(id)
	defer graph.imageMutex.Unlock(id)

	return graph.setLayerDigest(id, dgst)
}
func (graph *Graph) setLayerDigest(id string, dgst digest.Digest) error {
	root := graph.imageRoot(id)
	if err := ioutil.WriteFile(filepath.Join(root, digestFileName), []byte(dgst.String()), 0600); err != nil {
		return fmt.Errorf("Error storing digest in %s/%s: %s", root, digestFileName, err)
	}
	return nil
}

// GetLayerDigest gets the digest for the provide image layer id.
func (graph *Graph) GetLayerDigest(id string) (digest.Digest, error) {
	graph.imageMutex.Lock(id)
	defer graph.imageMutex.Unlock(id)

	return graph.getLayerDigest(id)
}

func (graph *Graph) getLayerDigest(id string) (digest.Digest, error) {
	root := graph.imageRoot(id)
	cs, err := ioutil.ReadFile(filepath.Join(root, digestFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrDigestNotSet
		}
		return "", err
	}
	return digest.ParseDigest(string(cs))
}

// SetV1CompatibilityConfig stores the v1Compatibility JSON data associated
// with the image in the manifest to the disk
func (graph *Graph) SetV1CompatibilityConfig(id string, data []byte) error {
	graph.imageMutex.Lock(id)
	defer graph.imageMutex.Unlock(id)

	return graph.setV1CompatibilityConfig(id, data)
}
func (graph *Graph) setV1CompatibilityConfig(id string, data []byte) error {
	root := graph.imageRoot(id)
	return ioutil.WriteFile(filepath.Join(root, v1CompatibilityFileName), data, 0600)
}

// GetV1CompatibilityConfig reads the v1Compatibility JSON data for the image
// from the disk
func (graph *Graph) GetV1CompatibilityConfig(id string) ([]byte, error) {
	graph.imageMutex.Lock(id)
	defer graph.imageMutex.Unlock(id)

	return graph.getV1CompatibilityConfig(id)
}

func (graph *Graph) getV1CompatibilityConfig(id string) ([]byte, error) {
	root := graph.imageRoot(id)
	return ioutil.ReadFile(filepath.Join(root, v1CompatibilityFileName))
}

// GenerateV1CompatibilityChain makes sure v1Compatibility JSON data exists
// for the image. If it doesn't it generates and stores it for the image and
// all of it's parents based on the image config JSON.
func (graph *Graph) GenerateV1CompatibilityChain(id string) ([]byte, error) {
	graph.imageMutex.Lock(id)
	defer graph.imageMutex.Unlock(id)

	if v1config, err := graph.getV1CompatibilityConfig(id); err == nil {
		return v1config, nil
	}

	// generate new, store it to disk
	img, err := graph.Get(id)
	if err != nil {
		return nil, err
	}

	digestPrefix := string(digest.Canonical) + ":"
	img.ID = strings.TrimPrefix(img.ID, digestPrefix)

	if img.Parent != "" {
		parentConfig, err := graph.GenerateV1CompatibilityChain(img.Parent)
		if err != nil {
			return nil, err
		}
		var parent struct{ ID string }
		err = json.Unmarshal(parentConfig, &parent)
		if err != nil {
			return nil, err
		}
		img.Parent = parent.ID
	}

	json, err := json.Marshal(img)
	if err != nil {
		return nil, err
	}
	if err := graph.setV1CompatibilityConfig(id, json); err != nil {
		return nil, err
	}
	return json, nil
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
	return filepath.Join(root, jsonFileName)
}

// storeImage stores file system layer data for the given image to the
// graph's storage driver. Image metadata is stored in a file
// at the specified root directory.
func (graph *Graph) storeImage(id, parent string, config []byte, layerData io.Reader, root string) (err error) {
	var size int64
	// Store the layer. If layerData is not nil, unpack it into the new layer
	if layerData != nil {
		if size, err = graph.disassembleAndApplyTarLayer(id, parent, layerData, root); err != nil {
			return err
		}
	}

	if err := graph.saveSize(root, size); err != nil {
		return err
	}

	if err := ioutil.WriteFile(jsonPath(root), config, 0600); err != nil {
		return err
	}

	// If image is pointing to a parent via CompatibilityID write the reference to disk
	img, err := image.NewImgJSON(config)
	if err != nil {
		return err
	}

	if img.ParentID.Validate() == nil && parent != img.ParentID.Hex() {
		if err := ioutil.WriteFile(filepath.Join(root, parentFileName), []byte(parent), 0600); err != nil {
			return err
		}
	}
	return nil
}

func (graph *Graph) disassembleAndApplyTarLayer(id, parent string, layerData io.Reader, root string) (size int64, err error) {
	var ar io.Reader

	if graph.tarSplitDisabled {
		ar = layerData
	} else {
		// this is saving the tar-split metadata
		mf, err := os.OpenFile(filepath.Join(root, tarDataFileName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(0600))
		if err != nil {
			return 0, err
		}

		mfz := gzip.NewWriter(mf)
		metaPacker := storage.NewJSONPacker(mfz)
		defer mf.Close()
		defer mfz.Close()

		inflatedLayerData, err := archive.DecompressStream(layerData)
		if err != nil {
			return 0, err
		}

		// we're passing nil here for the file putter, because the ApplyDiff will
		// handle the extraction of the archive
		rdr, err := asm.NewInputTarStream(inflatedLayerData, metaPacker, nil)
		if err != nil {
			return 0, err
		}

		ar = archive.Reader(rdr)
	}

	if size, err = graph.driver.ApplyDiff(id, parent, ar); err != nil {
		return 0, err
	}

	return
}

func (graph *Graph) assembleTarLayer(img *image.Image) (io.ReadCloser, error) {
	root := graph.imageRoot(img.ID)
	mFileName := filepath.Join(root, tarDataFileName)
	mf, err := os.Open(mFileName)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Errorf("failed to open %q: %s", mFileName, err)
		}
		return nil, err
	}
	pR, pW := io.Pipe()
	// this will need to be in a goroutine, as we are returning the stream of a
	// tar archive, but can not close the metadata reader early (when this
	// function returns)...
	go func() {
		defer mf.Close()
		// let's reassemble!
		logrus.Debugf("[graph] TarLayer with reassembly: %s", img.ID)
		mfz, err := gzip.NewReader(mf)
		if err != nil {
			pW.CloseWithError(fmt.Errorf("[graph] error with %s:  %s", mFileName, err))
			return
		}
		defer mfz.Close()

		// get our relative path to the container
		fsLayer, err := graph.driver.Get(img.ID, "")
		if err != nil {
			pW.CloseWithError(err)
			return
		}
		defer graph.driver.Put(img.ID)

		metaUnpacker := storage.NewJSONUnpacker(mfz)
		fileGetter := storage.NewPathFileGetter(fsLayer)
		logrus.Debugf("[graph] %s is at %q", img.ID, fsLayer)
		ots := asm.NewOutputTarStream(fileGetter, metaUnpacker)
		defer ots.Close()
		if _, err := io.Copy(pW, ots); err != nil {
			pW.CloseWithError(err)
			return
		}
		pW.Close()
	}()
	return pR, nil
}
