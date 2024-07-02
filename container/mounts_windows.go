package container // import "github.com/docker/docker/container"

// Mount contains information for a mount operation.
type Mount struct {
	// TODO: fix windows
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Writable    bool   `json:"writable"`
}
