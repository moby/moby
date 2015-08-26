package image

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/digest"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/pkg/version"
	"github.com/docker/docker/runconfig"
)

var validHex = regexp.MustCompile(`^([a-f0-9]{64})$`)

// noFallbackMinVersion is the minimum version for which v1compatibility
// information will not be marshaled through the Image struct to remove
// blank fields.
var noFallbackMinVersion = version.Version("1.8.3")

// Descriptor provides the information necessary to register an image in
// the graph.
type Descriptor interface {
	ID() string
	Parent() string
	MarshalConfig() ([]byte, error)
}

// Image stores the image configuration.
// All fields in this struct must be marked `omitempty` to keep getting
// predictable hashes from the old `v1Compatibility` configuration.
type Image struct {
	// ID a unique 64 character identifier of the image
	ID string `json:"id,omitempty"`
	// Parent id of the image
	Parent string `json:"parent,omitempty"`
	// Comment user added comment
	Comment string `json:"comment,omitempty"`
	// Created timestamp when image was created
	Created time.Time `json:"created"`
	// Container is the id of the container used to commit
	Container string `json:"container,omitempty"`
	// ContainerConfig  is the configuration of the container that is committed into the image
	ContainerConfig runconfig.Config `json:"container_config,omitempty"`
	// DockerVersion specifies version on which image is built
	DockerVersion string `json:"docker_version,omitempty"`
	// Author of the image
	Author string `json:"author,omitempty"`
	// Config is the configuration of the container received from the client
	Config *runconfig.Config `json:"config,omitempty"`
	// Architecture is the hardware that the image is build and runs on
	Architecture string `json:"architecture,omitempty"`
	// OS is the operating system used to build and run the image
	OS string `json:"os,omitempty"`
	// Size is the total size of the image including all layers it is composed of
	Size int64 `json:",omitempty"` // capitalized for backwards compatibility
	// ParentID specifies the strong, content address of the parent configuration.
	ParentID digest.Digest `json:"parent_id,omitempty"`
	// LayerID provides the content address of the associated layer.
	LayerID digest.Digest `json:"layer_id,omitempty"`
}

// NewImgJSON creates an Image configuration from json.
func NewImgJSON(src []byte) (*Image, error) {
	ret := &Image{}

	// FIXME: Is there a cleaner way to "purify" the input json?
	if err := json.Unmarshal(src, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// ValidateID checks whether an ID string is a valid image ID.
func ValidateID(id string) error {
	if ok := validHex.MatchString(id); !ok {
		return derr.ErrorCodeInvalidImageID.WithArgs(id)
	}
	return nil
}

// MakeImageConfig returns immutable configuration JSON for image based on the
// v1Compatibility object, layer digest and parent StrongID. SHA256() of this
// config is the new image ID (strongID).
func MakeImageConfig(v1Compatibility []byte, layerID, parentID digest.Digest) ([]byte, error) {

	// Detect images created after 1.8.3
	img, err := NewImgJSON(v1Compatibility)
	if err != nil {
		return nil, err
	}
	useFallback := version.Version(img.DockerVersion).LessThan(noFallbackMinVersion)

	if useFallback {
		// Fallback for pre-1.8.3. Calculate base config based on Image struct
		// so that fields with default values added by Docker will use same ID
		logrus.Debugf("Using fallback hash for %v", layerID)

		v1Compatibility, err = json.Marshal(img)
		if err != nil {
			return nil, err
		}
	}

	var c map[string]*json.RawMessage
	if err := json.Unmarshal(v1Compatibility, &c); err != nil {
		return nil, err
	}

	if err := layerID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid layerID: %v", err)
	}

	c["layer_id"] = rawJSON(layerID)

	if parentID != "" {
		if err := parentID.Validate(); err != nil {
			return nil, fmt.Errorf("invalid parentID %v", err)
		}
		c["parent_id"] = rawJSON(parentID)
	}

	delete(c, "id")
	delete(c, "parent")
	delete(c, "Size") // Size is calculated from data on disk and is inconsitent

	return json.Marshal(c)
}

// StrongID returns image ID for the config JSON.
func StrongID(configJSON []byte) (digest.Digest, error) {
	digester := digest.Canonical.New()
	if _, err := digester.Hash().Write(configJSON); err != nil {
		return "", err
	}
	dgst := digester.Digest()
	logrus.Debugf("H(%v) = %v", string(configJSON), dgst)
	return dgst, nil
}

func rawJSON(value interface{}) *json.RawMessage {
	jsonval, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return (*json.RawMessage)(&jsonval)
}
