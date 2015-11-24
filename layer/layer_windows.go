package layer

import (
	"errors"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
)

// GetLayerPath returns the path to a layer
func GetLayerPath(s Store, layer ChainID) (string, error) {
	ls, ok := s.(*layerStore)
	if !ok {
		return "", errors.New("unsupported layer store")
	}
	ls.layerL.Lock()
	defer ls.layerL.Unlock()

	rl, ok := ls.layerMap[layer]
	if !ok {
		return "", ErrLayerDoesNotExist
	}

	path, err := ls.driver.Get(rl.cacheID, "")
	if err != nil {
		return "", err
	}

	if err := ls.driver.Put(rl.cacheID); err != nil {
		return "", err
	}

	return path, nil
}

// RWLayerMetadata returns the graph metadata for the provided
// mount name.
func RWLayerMetadata(s Store, name string) (map[string]string, error) {
	ls, ok := s.(*layerStore)
	if !ok {
		return nil, errors.New("unsupported layer store")
	}
	ls.mountL.Lock()
	defer ls.mountL.Unlock()

	ml, ok := ls.mounts[name]
	if !ok {
		return nil, errors.New("mount does not exist")
	}

	return ls.driver.GetMetadata(ml.mountID)
}

func (ls *layerStore) RegisterDiffID(graphID string, size int64) (Layer, error) {
	var err error // this is used for cleanup in existingLayer case
	diffID, err := digest.FromBytes([]byte(graphID))
	if err != nil {
		return nil, err
	}

	// Create new roLayer
	layer := &roLayer{
		cacheID:        graphID,
		diffID:         DiffID(diffID),
		referenceCount: 1,
		layerStore:     ls,
		references:     map[Layer]struct{}{},
		size:           size,
	}

	tx, err := ls.store.StartTransaction()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if err := tx.Cancel(); err != nil {
				logrus.Errorf("Error canceling metadata transaction %q: %s", tx.String(), err)
			}
		}
	}()

	layer.chainID = createChainIDFromParent("", layer.diffID)

	if !ls.driver.Exists(layer.cacheID) {
		return nil, fmt.Errorf("layer %q is unknown to driver", layer.cacheID)
	}
	if err = storeLayer(tx, layer); err != nil {
		return nil, err
	}

	ls.layerL.Lock()
	defer ls.layerL.Unlock()

	if existingLayer := ls.getWithoutLock(layer.chainID); existingLayer != nil {
		// Set error for cleanup, but do not return
		err = errors.New("layer already exists")
		return existingLayer.getReference(), nil
	}

	if err = tx.Commit(layer.chainID); err != nil {
		return nil, err
	}

	ls.layerMap[layer.chainID] = layer

	return layer.getReference(), nil
}
