// +build !windows

package graph

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/system"
)

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
			prev = filepath.Join(prev, p)
			syscall.Unlink(filepath.Join(initLayer, prev))
		}

		if _, err := os.Stat(filepath.Join(initLayer, pth)); err != nil {
			if os.IsNotExist(err) {
				if err := system.MkdirAll(filepath.Join(initLayer, filepath.Dir(pth)), 0755); err != nil {
					return err
				}
				switch typ {
				case "dir":
					if err := system.MkdirAll(filepath.Join(initLayer, pth), 0755); err != nil {
						return err
					}
				case "file":
					f, err := os.OpenFile(filepath.Join(initLayer, pth), os.O_CREATE, 0755)
					if err != nil {
						return err
					}
					f.Close()
				default:
					if err := os.Symlink(typ, filepath.Join(initLayer, pth)); err != nil {
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

func (graph *Graph) restoreBaseImages() ([]string, error) {
	return nil, nil
}

// storeImage stores file system layer data for the given image to the
// graph's storage driver. Image metadata is stored in a file
// at the specified root directory.
func (graph *Graph) storeImage(id, parent string, config []byte, layerData archive.ArchiveReader, root string) (err error) {
	var size int64
	// Store the layer. If layerData is not nil, unpack it into the new layer
	if layerData != nil {
		if size, err = graph.disassembleAndApplyTarLayer(id, parent, layerData, root); err != nil {
			return err
		}
	}

	if err := graph.saveSize(root, int(size)); err != nil {
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

// TarLayer returns a tar archive of the image's filesystem layer.
func (graph *Graph) TarLayer(img *image.Image) (arch archive.Archive, err error) {
	rdr, err := graph.assembleTarLayer(img)
	if err != nil {
		logrus.Debugf("[graph] TarLayer with traditional differ: %s", img.ID)
		return graph.driver.Diff(img.ID, img.Parent)
	}
	return rdr, nil
}
