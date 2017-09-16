package layer

import (
	"errors"
)

// Getter is an interface to get the path to a layer on the host.
type Getter interface {
	// GetLayerPath gets the path for the layer. This is different from Get()
	// since that returns an interface to account for umountable layers.
	GetLayerPath(id string) (string, error)
}

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

	if layerGetter, ok := ls.driver.(Getter); ok {
		return layerGetter.GetLayerPath(rl.cacheID)
	}

	path, err := ls.driver.Get(rl.cacheID, "")
	if err != nil {
		return "", err
	}

	if err := ls.driver.Put(rl.cacheID); err != nil {
		return "", err
	}

	return path.Path(), nil
}

func (ls *layerStore) mountID(name string) string {
	// windows has issues if container ID doesn't match mount ID
	return name
}
