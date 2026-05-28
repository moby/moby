package v1

import (
	"context"
	"encoding/json"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/moby/moby/v2/daemon/internal/layer"
	"github.com/opencontainers/go-digest"
)

// CreateID creates an ID from v1 image, layerID and parent ID.
// Used for backwards compatibility with old clients.
func CreateID(v1Image image.V1Image, layerID layer.ChainID, parent digest.Digest) (digest.Digest, error) {
	v1Image.ID = ""
	v1JSON, err := json.Marshal(v1Image)
	if err != nil {
		return "", err
	}

	var config map[string]*json.RawMessage
	if err := json.Unmarshal(v1JSON, &config); err != nil {
		return "", err
	}

	// FIXME: note that this is slightly incompatible with RootFS logic
	config["layer_id"] = rawJSON(layerID)
	if parent != "" {
		config["parent"] = rawJSON(parent)
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	log.G(context.TODO()).Debugf("CreateV1ID %s", configJSON)

	return digest.FromBytes(configJSON), nil
}

func rawJSON(value any) *json.RawMessage {
	jsonval, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return (*json.RawMessage)(&jsonval)
}
