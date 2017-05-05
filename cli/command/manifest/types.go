package manifest

import (
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/opencontainers/go-digest"
)

// fallbackError wraps an error that can possibly allow fallback to a different
// endpoint.
type fallbackError struct {
	// err is the error being wrapped.
	err error
	// confirmedV2 is set to true if it was confirmed that the registry
	// supports the v2 protocol. This is used to limit fallbacks to the v1
	// protocol.
	confirmedV2 bool
	transportOK bool
}

// Error renders the FallbackError as a string.
func (f fallbackError) Error() string {
	return f.err.Error()
}

// ImgManifestInspect contains info to output for a manifest object.
type ImgManifestInspect struct {
	RefName         string                 `json:"ref"`
	Size            int64                  `json:"size"`
	MediaType       string                 `json:"media_type"`
	Tag             string                 `json:"tag"`
	Digest          digest.Digest          `json:"digest"`
	RepoTags        []string               `json:"repotags"`
	Comment         string                 `json:"comment"`
	Created         string                 `json:"created"`
	ContainerConfig *containerTypes.Config `json:"container_config"`
	DockerVersion   string                 `json:"docker_version"`
	Author          string                 `json:"author"`
	Config          *containerTypes.Config `json:"config"`
	References      []string               `json:"references"`
	LayerDigests    []string               `json:"layers_digests"`
	// The following are top-level objects because nested json from a file
	// won't unmarshal correctly.
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	OSVersion    string   `json:"os.version,omitempty"`
	OSFeatures   []string `json:"os.features,omitempty"`
	Variant      string   `json:"variant,omitempty"`
	Features     []string `json:"features,omitempty"`
	// This one's prettier at the end
	CanonicalJSON []byte `json:"json"`
}
