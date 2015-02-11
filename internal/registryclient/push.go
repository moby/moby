package client

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/distribution/manifest"
)

// simultaneousLayerPushWindow is the size of the parallel layer push window.
// A layer may not be pushed until the layer preceeding it by the length of the
// push window has been successfully pushed.
const simultaneousLayerPushWindow = 4

type pushFunction func(fsLayer manifest.FSLayer) error

// Push implements a client push workflow for the image defined by the given
// name and tag pair, using the given ObjectStore for local manifest and layer
// storage
func Push(c Client, objectStore ObjectStore, name, tag string) error {
	manifest, err := objectStore.Manifest(name, tag)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"name":  name,
			"tag":   tag,
		}).Info("No image found")
		return err
	}

	errChans := make([]chan error, len(manifest.FSLayers))
	for i := range manifest.FSLayers {
		errChans[i] = make(chan error)
	}

	cancelCh := make(chan struct{})

	// Iterate over each layer in the manifest, simultaneously pushing no more
	// than simultaneousLayerPushWindow layers at a time. If an error is
	// received from a layer push, we abort the push.
	for i := 0; i < len(manifest.FSLayers)+simultaneousLayerPushWindow; i++ {
		dependentLayer := i - simultaneousLayerPushWindow
		if dependentLayer >= 0 {
			err := <-errChans[dependentLayer]
			if err != nil {
				log.WithField("error", err).Warn("Push aborted")
				close(cancelCh)
				return err
			}
		}

		if i < len(manifest.FSLayers) {
			go func(i int) {
				select {
				case errChans[i] <- pushLayer(c, objectStore, name, manifest.FSLayers[i]):
				case <-cancelCh: // recv broadcast notification about cancelation
				}
			}(i)
		}
	}

	err = c.PutImageManifest(name, tag, manifest)
	if err != nil {
		log.WithFields(log.Fields{
			"error":    err,
			"manifest": manifest,
		}).Warn("Unable to upload manifest")
		return err
	}

	return nil
}

func pushLayer(c Client, objectStore ObjectStore, name string, fsLayer manifest.FSLayer) error {
	log.WithField("layer", fsLayer).Info("Pushing layer")

	layer, err := objectStore.Layer(fsLayer.BlobSum)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to read local layer")
		return err
	}

	layerReader, err := layer.Reader()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to read local layer")
		return err
	}
	defer layerReader.Close()

	if layerReader.CurrentSize() != layerReader.Size() {
		log.WithFields(log.Fields{
			"layer":       fsLayer,
			"currentSize": layerReader.CurrentSize(),
			"size":        layerReader.Size(),
		}).Warn("Local layer incomplete")
		return fmt.Errorf("Local layer incomplete")
	}

	length, err := c.BlobLength(name, fsLayer.BlobSum)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to check existence of remote layer")
		return err
	}
	if length >= 0 {
		log.WithField("layer", fsLayer).Info("Layer already exists")
		return nil
	}

	location, err := c.InitiateBlobUpload(name)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to upload layer")
		return err
	}

	err = c.UploadBlob(location, layerReader, int(layerReader.CurrentSize()), fsLayer.BlobSum)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"layer": fsLayer,
		}).Warn("Unable to upload layer")
		return err
	}

	return nil
}
