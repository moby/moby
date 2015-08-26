// +build windows

package graph

import (
	"io/ioutil"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver/windows"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
)

// setupInitLayer populates a directory with mountpoints suitable
// for bind-mounting dockerinit into the container. T
func SetupInitLayer(initLayer string) error {
	return nil
}

func (graph *Graph) restoreBaseImages() ([]string, error) {
	// TODO Windows. This needs implementing (@swernli)
	return nil, nil
}

// ParentLayerIds returns a list of all parent image IDs for the given image.
func (graph *Graph) ParentLayerIds(img *image.Image) (ids []string, err error) {
	for i := img; i != nil && err == nil; i, err = graph.GetParent(i) {
		ids = append(ids, i.ID)
	}

	return
}

// storeImage stores file system layer data for the given image to the
// graph's storage driver. Image metadata is stored in a file
// at the specified root directory.
func (graph *Graph) storeImage(id, parent string, config []byte, layerData archive.ArchiveReader, root string) (err error) {
	var size int64
	if wd, ok := graph.driver.(*windows.WindowsGraphDriver); ok {
		// Store the layer. If layerData is not nil and this isn't a base image,
		// unpack it into the new layer
		if layerData != nil && parent != "" {
			var ids []string
			parentImg, err := graph.Get(parent)
			if err != nil {
				return err
			}

			ids, err = graph.ParentLayerIds(parentImg)
			if err != nil {
				return err
			}

			if size, err = wd.Import(id, layerData, wd.LayerIdsToPaths(ids)); err != nil {
				return err
			}
		}

	} else {
		// We keep this functionality here so that we can still work with the
		// VFS driver during development. This will not be used for actual running
		// of Windows containers. Without this code, it would not be possible to
		// docker pull using the VFS driver.

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
}

// TarLayer returns a tar archive of the image's filesystem layer.
func (graph *Graph) TarLayer(img *image.Image) (arch archive.Archive, err error) {
	if wd, ok := graph.driver.(*windows.WindowsGraphDriver); ok {
		var ids []string
		if img.Parent != "" {
			parentImg, err := graph.Get(img.Parent)
			if err != nil {
				return nil, err
			}

			ids, err = graph.ParentLayerIds(parentImg)
			if err != nil {
				return nil, err
			}
		}

		return wd.Export(img.ID, wd.LayerIdsToPaths(ids))
	} else {
		// We keep this functionality here so that we can still work with the VFS
		// driver during development. VFS is not supported (and just will not work)
		// for Windows containers.
		rdr, err := graph.assembleTarLayer(img)
		if err != nil {
			logrus.Debugf("[graph] TarLayer with traditional differ: %s", img.ID)
			return graph.driver.Diff(img.ID, img.Parent)
		}
		return rdr, nil
	}
}
