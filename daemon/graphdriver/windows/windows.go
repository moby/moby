//+build windows

package windows

import (
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/chrootarchive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/random"
	"github.com/microsoft/hcsshim"
)

// init registers the windows graph drivers to the register.
func init() {
	graphdriver.Register("windowsfilter", InitFilter)
	graphdriver.Register("windowsdiff", InitDiff)
}

const (
	// diffDriver is an hcsshim driver type
	diffDriver = iota
	// filterDriver is an hcsshim driver type
	filterDriver
)

// Driver represents a windows graph driver.
type Driver struct {
	// info stores the shim driver information
	info hcsshim.DriverInfo
	// Mutex protects concurrent modification to active
	sync.Mutex
	// active stores references to the activated layers
	active map[string]int
}

// InitFilter returns a new Windows storage filter driver.
func InitFilter(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	logrus.Debugf("WindowsGraphDriver InitFilter at %s", home)
	d := &Driver{
		info: hcsshim.DriverInfo{
			HomeDir: home,
			Flavour: filterDriver,
		},
		active: make(map[string]int),
	}
	return d, nil
}

// InitDiff returns a new Windows differencing disk driver.
func InitDiff(home string, options []string, uidMaps, gidMaps []idtools.IDMap) (graphdriver.Driver, error) {
	logrus.Debugf("WindowsGraphDriver InitDiff at %s", home)
	d := &Driver{
		info: hcsshim.DriverInfo{
			HomeDir: home,
			Flavour: diffDriver,
		},
		active: make(map[string]int),
	}
	return d, nil
}

// String returns the string representation of a driver.
func (d *Driver) String() string {
	switch d.info.Flavour {
	case diffDriver:
		return "windowsdiff"
	case filterDriver:
		return "windowsfilter"
	default:
		return "Unknown driver flavour"
	}
}

// Status returns the status of the driver.
func (d *Driver) Status() [][2]string {
	return [][2]string{
		{"Windows", ""},
	}
}

// Exists returns true if the given id is registered with this driver.
func (d *Driver) Exists(id string) bool {
	rID, err := d.resolveID(id)
	if err != nil {
		return false
	}
	result, err := hcsshim.LayerExists(d.info, rID)
	if err != nil {
		return false
	}
	return result
}

// Create creates a new layer with the given id.
func (d *Driver) Create(id, parent, mountLabel string) error {
	rPId, err := d.resolveID(parent)
	if err != nil {
		return err
	}

	parentChain, err := d.getLayerChain(rPId)
	if err != nil {
		return err
	}

	var layerChain []string

	parentIsInit := strings.HasSuffix(rPId, "-init")

	if !parentIsInit && rPId != "" {
		parentPath, err := hcsshim.GetLayerMountPath(d.info, rPId)
		if err != nil {
			return err
		}
		layerChain = []string{parentPath}
	}

	layerChain = append(layerChain, parentChain...)

	if parentIsInit {
		if len(layerChain) == 0 {
			return fmt.Errorf("Cannot create a read/write layer without a parent layer.")
		}
		if err := hcsshim.CreateSandboxLayer(d.info, id, layerChain[0], layerChain); err != nil {
			return err
		}
	} else {
		if err := hcsshim.CreateLayer(d.info, id, rPId); err != nil {
			return err
		}
	}

	if _, err := os.Lstat(d.dir(parent)); err != nil {
		if err2 := hcsshim.DestroyLayer(d.info, id); err2 != nil {
			logrus.Warnf("Failed to DestroyLayer %s: %s", id, err2)
		}
		return fmt.Errorf("Cannot create layer with missing parent %s: %s", parent, err)
	}

	if err := d.setLayerChain(id, layerChain); err != nil {
		if err2 := hcsshim.DestroyLayer(d.info, id); err2 != nil {
			logrus.Warnf("Failed to DestroyLayer %s: %s", id, err2)
		}
		return err
	}

	return nil
}

// dir returns the absolute path to the layer.
func (d *Driver) dir(id string) string {
	return filepath.Join(d.info.HomeDir, filepath.Base(id))
}

// Remove unmounts and removes the dir information.
func (d *Driver) Remove(id string) error {
	rID, err := d.resolveID(id)
	if err != nil {
		return err
	}
	os.RemoveAll(filepath.Join(d.info.HomeDir, "sysfile-backups", rID)) // ok to fail
	return hcsshim.DestroyLayer(d.info, rID)
}

// Get returns the rootfs path for the id. This will mount the dir at it's given path.
func (d *Driver) Get(id, mountLabel string) (string, error) {
	logrus.Debugf("WindowsGraphDriver Get() id %s mountLabel %s", id, mountLabel)
	var dir string

	d.Lock()
	defer d.Unlock()

	rID, err := d.resolveID(id)
	if err != nil {
		return "", err
	}

	// Getting the layer paths must be done outside of the lock.
	layerChain, err := d.getLayerChain(rID)
	if err != nil {
		return "", err
	}

	if d.active[rID] == 0 {
		if err := hcsshim.ActivateLayer(d.info, rID); err != nil {
			return "", err
		}
		if err := hcsshim.PrepareLayer(d.info, rID, layerChain); err != nil {
			if err2 := hcsshim.DeactivateLayer(d.info, rID); err2 != nil {
				logrus.Warnf("Failed to Deactivate %s: %s", id, err)
			}
			return "", err
		}
	}

	mountPath, err := hcsshim.GetLayerMountPath(d.info, rID)
	if err != nil {
		if err2 := hcsshim.DeactivateLayer(d.info, rID); err2 != nil {
			logrus.Warnf("Failed to Deactivate %s: %s", id, err)
		}
		return "", err
	}

	d.active[rID]++

	// If the layer has a mount path, use that. Otherwise, use the
	// folder path.
	if mountPath != "" {
		dir = mountPath
	} else {
		dir = d.dir(id)
	}

	return dir, nil
}

// Put adds a new layer to the driver.
func (d *Driver) Put(id string) error {
	logrus.Debugf("WindowsGraphDriver Put() id %s", id)

	rID, err := d.resolveID(id)
	if err != nil {
		return err
	}

	d.Lock()
	defer d.Unlock()

	if d.active[rID] > 1 {
		d.active[rID]--
	} else if d.active[rID] == 1 {
		if err := hcsshim.UnprepareLayer(d.info, rID); err != nil {
			return err
		}
		if err := hcsshim.DeactivateLayer(d.info, rID); err != nil {
			return err
		}
		delete(d.active, rID)
	}

	return nil
}

// Cleanup ensures the information the driver stores is properly removed.
func (d *Driver) Cleanup() error {
	return nil
}

// Diff produces an archive of the changes between the specified
// layer and its parent layer which may be "".
func (d *Driver) Diff(id, parent string) (arch archive.Archive, err error) {
	rID, err := d.resolveID(id)
	if err != nil {
		return
	}

	// Getting the layer paths must be done outside of the lock.
	layerChain, err := d.getLayerChain(rID)
	if err != nil {
		return
	}

	d.Lock()

	// To support export, a layer must be activated but not prepared.
	if d.info.Flavour == filterDriver {
		if d.active[rID] == 0 {
			if err = hcsshim.ActivateLayer(d.info, rID); err != nil {
				d.Unlock()
				return
			}
			defer func() {
				if err := hcsshim.DeactivateLayer(d.info, rID); err != nil {
					logrus.Warnf("Failed to Deactivate %s: %s", rID, err)
				}
			}()
		} else {
			if err = hcsshim.UnprepareLayer(d.info, rID); err != nil {
				d.Unlock()
				return
			}
			defer func() {
				if err := hcsshim.PrepareLayer(d.info, rID, layerChain); err != nil {
					logrus.Warnf("Failed to re-PrepareLayer %s: %s", rID, err)
				}
			}()
		}
	}

	d.Unlock()

	return d.exportLayer(rID, layerChain)
}

// Changes produces a list of changes between the specified layer
// and its parent layer. If parent is "", then all changes will be ADD changes.
func (d *Driver) Changes(id, parent string) ([]archive.Change, error) {
	return nil, fmt.Errorf("The Windows graphdriver does not support Changes()")
}

// ApplyDiff extracts the changeset from the given diff into the
// layer with the specified id and parent, returning the size of the
// new layer in bytes.
func (d *Driver) ApplyDiff(id, parent string, diff archive.Reader) (size int64, err error) {
	rPId, err := d.resolveID(parent)
	if err != nil {
		return
	}

	if d.info.Flavour == diffDriver {
		start := time.Now().UTC()
		logrus.Debugf("WindowsGraphDriver ApplyDiff: Start untar layer")
		destination := d.dir(id)
		destination = filepath.Dir(destination)
		if size, err = chrootarchive.ApplyUncompressedLayer(destination, diff, nil); err != nil {
			return
		}
		logrus.Debugf("WindowsGraphDriver ApplyDiff: Untar time: %vs", time.Now().UTC().Sub(start).Seconds())

		return
	}

	parentChain, err := d.getLayerChain(rPId)
	if err != nil {
		return
	}
	parentPath, err := hcsshim.GetLayerMountPath(d.info, rPId)
	if err != nil {
		return
	}
	layerChain := []string{parentPath}
	layerChain = append(layerChain, parentChain...)

	if size, err = d.importLayer(id, diff, layerChain); err != nil {
		return
	}

	if err = d.setLayerChain(id, layerChain); err != nil {
		return
	}

	return
}

// DiffSize calculates the changes between the specified layer
// and its parent and returns the size in bytes of the changes
// relative to its base filesystem directory.
func (d *Driver) DiffSize(id, parent string) (size int64, err error) {
	rPId, err := d.resolveID(parent)
	if err != nil {
		return
	}

	changes, err := d.Changes(id, rPId)
	if err != nil {
		return
	}

	layerFs, err := d.Get(id, "")
	if err != nil {
		return
	}
	defer d.Put(id)

	return archive.ChangesSize(layerFs, changes), nil
}

// CustomImageInfo is the object returned by the driver describing the base
// image.
type CustomImageInfo struct {
	ID          string
	Name        string
	Version     string
	Path        string
	Size        int64
	CreatedTime time.Time
}

// GetCustomImageInfos returns the image infos for window specific
// base images which should always be present.
func (d *Driver) GetCustomImageInfos() ([]CustomImageInfo, error) {
	strData, err := hcsshim.GetSharedBaseImages()
	if err != nil {
		return nil, fmt.Errorf("Failed to restore base images: %s", err)
	}

	type customImageInfoList struct {
		Images []CustomImageInfo
	}

	var infoData customImageInfoList

	if err = json.Unmarshal([]byte(strData), &infoData); err != nil {
		err = fmt.Errorf("JSON unmarshal returned error=%s", err)
		logrus.Error(err)
		return nil, err
	}

	var images []CustomImageInfo

	for _, imageData := range infoData.Images {
		folderName := filepath.Base(imageData.Path)

		// Use crypto hash of the foldername to generate a docker style id.
		h := sha512.Sum384([]byte(folderName))
		id := fmt.Sprintf("%x", h[:32])

		if err := d.Create(id, "", ""); err != nil {
			return nil, err
		}
		// Create the alternate ID file.
		if err := d.setID(id, folderName); err != nil {
			return nil, err
		}

		imageData.ID = id
		images = append(images, imageData)
	}

	return images, nil
}

// GetMetadata returns custom driver information.
func (d *Driver) GetMetadata(id string) (map[string]string, error) {
	m := make(map[string]string)
	m["dir"] = d.dir(id)
	return m, nil
}

// exportLayer generates an archive from a layer based on the given ID.
func (d *Driver) exportLayer(id string, parentLayerPaths []string) (arch archive.Archive, err error) {
	layerFolder := d.dir(id)

	tempFolder := layerFolder + "-" + strconv.FormatUint(uint64(random.Rand.Uint32()), 10)
	if err = os.MkdirAll(tempFolder, 0755); err != nil {
		logrus.Errorf("Could not create %s %s", tempFolder, err)
		return
	}
	defer func() {
		if err != nil {
			_, folderName := filepath.Split(tempFolder)
			if err2 := hcsshim.DestroyLayer(d.info, folderName); err2 != nil {
				logrus.Warnf("Couldn't clean-up tempFolder: %s %s", tempFolder, err2)
			}
		}
	}()

	if err = hcsshim.ExportLayer(d.info, id, tempFolder, parentLayerPaths); err != nil {
		return
	}

	archive, err := archive.Tar(tempFolder, archive.Uncompressed)
	if err != nil {
		return
	}
	return ioutils.NewReadCloserWrapper(archive, func() error {
		err := archive.Close()
		d.Put(id)
		_, folderName := filepath.Split(tempFolder)
		if err2 := hcsshim.DestroyLayer(d.info, folderName); err2 != nil {
			logrus.Warnf("Couldn't clean-up tempFolder: %s %s", tempFolder, err2)
		}
		return err
	}), nil

}

// importLayer adds a new layer to the tag and graph store based on the given data.
func (d *Driver) importLayer(id string, layerData archive.Reader, parentLayerPaths []string) (size int64, err error) {
	layerFolder := d.dir(id)

	tempFolder := layerFolder + "-" + strconv.FormatUint(uint64(random.Rand.Uint32()), 10)
	if err = os.MkdirAll(tempFolder, 0755); err != nil {
		logrus.Errorf("Could not create %s %s", tempFolder, err)
		return
	}
	defer func() {
		_, folderName := filepath.Split(tempFolder)
		if err2 := hcsshim.DestroyLayer(d.info, folderName); err2 != nil {
			logrus.Warnf("Couldn't clean-up tempFolder: %s %s", tempFolder, err2)
		}
	}()

	start := time.Now().UTC()
	logrus.Debugf("Start untar layer")
	if size, err = chrootarchive.ApplyLayer(tempFolder, layerData); err != nil {
		return
	}
	err = copySysFiles(tempFolder, filepath.Join(d.info.HomeDir, "sysfile-backups", id))
	if err != nil {
		return
	}
	logrus.Debugf("Untar time: %vs", time.Now().UTC().Sub(start).Seconds())

	if err = hcsshim.ImportLayer(d.info, id, tempFolder, parentLayerPaths); err != nil {
		return
	}

	return
}

// resolveID computes the layerID information based on the given id.
func (d *Driver) resolveID(id string) (string, error) {
	content, err := ioutil.ReadFile(filepath.Join(d.dir(id), "layerID"))
	if os.IsNotExist(err) {
		return id, nil
	} else if err != nil {
		return "", err
	}
	return string(content), nil
}

// setID stores the layerId in disk.
func (d *Driver) setID(id, altID string) error {
	err := ioutil.WriteFile(filepath.Join(d.dir(id), "layerId"), []byte(altID), 0600)
	if err != nil {
		return err
	}
	return nil
}

// getLayerChain returns the layer chain information.
func (d *Driver) getLayerChain(id string) ([]string, error) {
	jPath := filepath.Join(d.dir(id), "layerchain.json")
	content, err := ioutil.ReadFile(jPath)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("Unable to read layerchain file - %s", err)
	}

	var layerChain []string
	err = json.Unmarshal(content, &layerChain)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshall layerchain json - %s", err)
	}

	return layerChain, nil
}

// setLayerChain stores the layer chain information in disk.
func (d *Driver) setLayerChain(id string, chain []string) error {
	content, err := json.Marshal(&chain)
	if err != nil {
		return fmt.Errorf("Failed to marshall layerchain json - %s", err)
	}

	jPath := filepath.Join(d.dir(id), "layerchain.json")
	err = ioutil.WriteFile(jPath, content, 0600)
	if err != nil {
		return fmt.Errorf("Unable to write layerchain file - %s", err)
	}

	return nil
}

// DiffPath returns a directory that contains files needed to construct layer diff.
func (d *Driver) DiffPath(id string) (path string, release func() error, err error) {
	id, err = d.resolveID(id)
	if err != nil {
		return
	}

	// Getting the layer paths must be done outside of the lock.
	layerChain, err := d.getLayerChain(id)
	if err != nil {
		return
	}

	layerFolder := d.dir(id)
	tempFolder := layerFolder + "-" + strconv.FormatUint(uint64(random.Rand.Uint32()), 10)
	if err = os.MkdirAll(tempFolder, 0755); err != nil {
		logrus.Errorf("Could not create %s %s", tempFolder, err)
		return
	}

	defer func() {
		if err != nil {
			_, folderName := filepath.Split(tempFolder)
			if err2 := hcsshim.DestroyLayer(d.info, folderName); err2 != nil {
				logrus.Warnf("Couldn't clean-up tempFolder: %s %s", tempFolder, err2)
			}
		}
	}()

	if err = hcsshim.ExportLayer(d.info, id, tempFolder, layerChain); err != nil {
		return
	}

	err = copySysFiles(filepath.Join(d.info.HomeDir, "sysfile-backups", id), tempFolder)
	if err != nil {
		return
	}

	return tempFolder, func() error {
		// TODO: activate layers and release here?
		_, folderName := filepath.Split(tempFolder)
		return hcsshim.DestroyLayer(d.info, folderName)
	}, nil
}

var sysFileWhiteList = []string{
	"Hives\\*",
	"Files\\BOOTNXT",
	"tombstones.txt",
}

// note this only handles files
func copySysFiles(src string, dest string) error {
	if err := os.MkdirAll(dest, 0700); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		for _, sysfile := range sysFileWhiteList {
			if matches, err := filepath.Match(sysfile, rel); err != nil || !matches {
				continue
			}

			fi, err := os.Lstat(path)
			if err != nil {
				return err
			}

			if !fi.Mode().IsRegular() {
				continue
			}

			targetPath := filepath.Join(dest, rel)
			if err = os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
				return err
			}

			in, err := os.Open(path)
			if err != nil {
				return err
			}
			out, err := os.Create(targetPath)
			if err != nil {
				in.Close()
				return err
			}
			_, err = io.Copy(out, in)
			in.Close()
			out.Close()
			if err != nil {
				return err
			}
		}
		return nil
	})
}
