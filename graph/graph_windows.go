// +build windows

package graph

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/graphdriver/windows"
	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	"github.com/vbatts/tar-split/tar/asm"
	"github.com/vbatts/tar-split/tar/storage"
)

// setupInitLayer populates a directory with mountpoints suitable
// for bind-mounting dockerinit into the container. T
func SetupInitLayer(initLayer string) error {
	return nil
}

func createRootFilesystemInDriver(graph *Graph, img *image.Image, layerData archive.ArchiveReader) error {
	if wd, ok := graph.driver.(*windows.WindowsGraphDriver); ok {
		if img.Container != "" && layerData == nil {
			logrus.Debugf("Copying from container %s.", img.Container)

			var ids []string
			if img.Parent != "" {
				parentImg, err := graph.Get(img.Parent)
				if err != nil {
					return err
				}

				ids, err = graph.ParentLayerIds(parentImg)
				if err != nil {
					return err
				}
			}

			if err := wd.CopyDiff(img.Container, img.ID, wd.LayerIdsToPaths(ids)); err != nil {
				return fmt.Errorf("Driver %s failed to copy image rootfs %s: %s", graph.driver, img.Container, err)
			}
		} else if img.Parent == "" {
			if err := graph.driver.Create(img.ID, img.Parent); err != nil {
				return fmt.Errorf("Driver %s failed to create image rootfs %s: %s", graph.driver, img.ID, err)
			}
		}
	} else {
		// This fallback allows the use of VFS during daemon development.
		if err := graph.driver.Create(img.ID, img.Parent); err != nil {
			return fmt.Errorf("Driver %s failed to create image rootfs %s: %s", graph.driver, img.ID, err)
		}
	}
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
func (graph *Graph) storeImage(img *image.Image, layerData archive.ArchiveReader, root string) (err error) {

	if wd, ok := graph.driver.(*windows.WindowsGraphDriver); ok {
		// Store the layer. If layerData is not nil and this isn't a base image,
		// unpack it into the new layer
		if layerData != nil && img.Parent != "" {
			var ids []string
			if img.Parent != "" {
				parentImg, err := graph.Get(img.Parent)
				if err != nil {
					return err
				}

				ids, err = graph.ParentLayerIds(parentImg)
				if err != nil {
					return err
				}
			}

			if img.Size, err = wd.Import(img.ID, layerData, wd.LayerIdsToPaths(ids)); err != nil {
				return err
			}
		}

		if err := graph.saveSize(root, int(img.Size)); err != nil {
			return err
		}

		f, err := os.OpenFile(jsonPath(root), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(0600))
		if err != nil {
			return err
		}

		defer f.Close()

		return json.NewEncoder(f).Encode(img)
	} else {
		// We keep this functionality here so that we can still work with the
		// VFS driver during development. This will not be used for actual running
		// of Windows containers. Without this code, it would not be possible to
		// docker pull using the VFS driver.

		// Store the layer. If layerData is not nil, unpack it into the new layer
		if layerData != nil {
			// this is saving the tar-split metadata
			mf, err := os.OpenFile(filepath.Join(root, tarDataFileName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(0600))
			if err != nil {
				return err
			}
			defer mf.Close()
			mfz := gzip.NewWriter(mf)
			defer mfz.Close()
			metaPacker := storage.NewJSONPacker(mfz)

			inflatedLayerData, err := archive.DecompressStream(layerData)
			if err != nil {
				return err
			}

			// we're passing nil here for the file putter, because the ApplyDiff will
			// handle the extraction of the archive
			its, err := asm.NewInputTarStream(inflatedLayerData, metaPacker, nil)
			if err != nil {
				return err
			}

			if img.Size, err = graph.driver.ApplyDiff(img.ID, img.Parent, archive.ArchiveReader(its)); err != nil {
				return err
			}
		}

		if err := graph.saveSize(root, int(img.Size)); err != nil {
			return err
		}

		f, err := os.OpenFile(jsonPath(root), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(0600))
		if err != nil {
			return err
		}

		defer f.Close()

		return json.NewEncoder(f).Encode(img)
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
		root := graph.imageRoot(img.ID)
		mFileName := filepath.Join(root, tarDataFileName)
		mf, err := os.Open(mFileName)
		if err != nil {
			if !os.IsNotExist(err) {
				logrus.Errorf("failed to open %q: %s", mFileName, err)
			}
			logrus.Debugf("[graph] TarLayer with traditional differ: %s", img.ID)
			return graph.driver.Diff(img.ID, img.Parent)
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
}
