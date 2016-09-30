package image

import (
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/docker/distribution/digest"
	"github.com/docker/docker/api/types/container"
)

// ID is the content-addressable ID of an image.
type ID digest.Digest

func (id ID) String() string {
	return id.Digest().String()
}

// Digest converts ID into a digest
func (id ID) Digest() digest.Digest {
	return digest.Digest(id)
}

// IDFromDigest creates an ID from a digest
func IDFromDigest(digest digest.Digest) ID {
	return ID(digest)
}

// V1Image stores the V1 image configuration.
type V1Image struct {
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
	// ContainerConfig is the configuration of the container that is committed into the image
	ContainerConfig container.Config `json:"container_config,omitempty"`
	// DockerVersion specifies version on which image is built
	DockerVersion string `json:"docker_version,omitempty"`
	// Author of the image
	Author string `json:"author,omitempty"`
	// Config is the configuration of the container received from the client
	Config *container.Config `json:"config,omitempty"`
	// Architecture is the hardware that the image is build and runs on
	Architecture string `json:"architecture,omitempty"`
	// OS is the operating system used to build and run the image
	OS string `json:"os,omitempty"`
	// Size is the total size of the image including all layers it is composed of
	Size int64 `json:",omitempty"`
}

// Image stores the image configuration
type Image struct {
	V1Image
	Parent     ID        `json:"parent,omitempty"`
	RootFS     *RootFS   `json:"rootfs,omitempty"`
	History    []History `json:"history,omitempty"`
	OSVersion  string    `json:"os.version,omitempty"`
	OSFeatures []string  `json:"os.features,omitempty"`

	// rawJSON caches the immutable JSON associated with this image.
	rawJSON []byte

	// computedID is the ID computed from the hash of the image config.
	// Not to be confused with the legacy V1 ID in V1Image.
	computedID ID
}

// RawJSON returns the immutable JSON associated with the image.
func (img *Image) RawJSON() []byte {
	return img.rawJSON
}

// ID returns the image's content-addressable ID.
func (img *Image) ID() ID {
	return img.computedID
}

// ImageID stringifies ID.
func (img *Image) ImageID() string {
	return img.ID().String()
}

// RunConfig returns the image's container config.
func (img *Image) RunConfig() *container.Config {
	return img.Config
}

// MarshalJSON serializes the image to JSON. It sorts the top-level keys so
// that JSON that's been manipulated by a push/pull cycle with a legacy
// registry won't end up with a different key order.
func (img *Image) MarshalJSON() ([]byte, error) {
	type MarshalImage Image

	pass1, err := json.Marshal(MarshalImage(*img))
	if err != nil {
		return nil, err
	}

	var c map[string]*json.RawMessage
	if err := json.Unmarshal(pass1, &c); err != nil {
		return nil, err
	}
	return json.Marshal(c)
}

// History stores build commands that were used to create an image
type History struct {
	// Created timestamp for build point
	Created time.Time `json:"created"`
	// Author of the build point
	Author string `json:"author,omitempty"`
	// CreatedBy keeps the Dockerfile command used while building image.
	CreatedBy string `json:"created_by,omitempty"`
	// Comment is custom message set by the user when creating the image.
	Comment string `json:"comment,omitempty"`
	// EmptyLayer is set to true if this history item did not generate a
	// layer. Otherwise, the history item is associated with the next
	// layer in the RootFS section.
	EmptyLayer bool `json:"empty_layer,omitempty"`
}

// Exporter provides interface for exporting and importing images
type Exporter interface {
	Load(io.ReadCloser, io.Writer, bool) error
	// TODO: Load(net.Context, io.ReadCloser, <- chan StatusMessage) error
	Save([]string, io.Writer) error
}

// NewFromJSON creates an Image configuration from json.
func NewFromJSON(src []byte) (*Image, error) {
	img := &Image{}

	if err := json.Unmarshal(src, img); err != nil {
		return nil, err
	}
	if img.RootFS == nil {
		return nil, errors.New("Invalid image JSON, no RootFS key.")
	}

	img.rawJSON = src

	return img, nil
}
