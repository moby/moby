// +build windows

package distribution

import (
	"encoding/json"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/docker/image"
)

func setupBaseLayer(history []schema1.History, rootFS image.RootFS) error {
	var v1Config map[string]*json.RawMessage
	if err := json.Unmarshal([]byte(history[len(history)-1].V1Compatibility), &v1Config); err != nil {
		return err
	}
	baseID, err := json.Marshal(rootFS.BaseLayerID())
	if err != nil {
		return err
	}
	v1Config["parent"] = (*json.RawMessage)(&baseID)
	configJSON, err := json.Marshal(v1Config)
	if err != nil {
		return err
	}
	history[len(history)-1].V1Compatibility = string(configJSON)
	return nil
}
