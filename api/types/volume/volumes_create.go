package volume // import "github.com/docker/docker/api/types/volume"

// VolumesCreateBody are the request parameters for creating a volume.
//
// TODO: replace with a generated type when swagger-gen supports generating
// types for operation parameters
type VolumesCreateBody struct {
	Driver     string            `json:"Driver"`
	DriverOpts map[string]string `json:"DriverOpts"`
	Labels     map[string]string `json:"Labels"`
	Name       string            `json:"Name"`
}
