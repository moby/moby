//go:build !windows

package container // import "github.com/docker/docker/container"

// Mount contains information for a mount operation.
type Mount struct {
	// ID is the unique identifier of this mount
	// TODO Q: should this field be exported to json?
	ID                     string `json:"-"`
	Source                 string `json:"source"`
	Destination            string `json:"destination"`
	Writable               bool   `json:"writable"`
	Data                   string `json:"data"`
	Propagation            string `json:"mountpropagation"`
	NonRecursive           bool   `json:"nonrecursive"`
	ReadOnlyNonRecursive   bool   `json:"readonlynonrecursive"`
	ReadOnlyForceRecursive bool   `json:"readonlyforcerecursive"`
}
