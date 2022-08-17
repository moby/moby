package types // import "github.com/docker/docker/api/types"
import "github.com/docker/docker/api/types/registry"

// AuthConfig contains authorization information for connecting to a Registry.
//
// Deprecated: use github.com/docker/docker/api/types/registry.AuthConfig
type AuthConfig = registry.AuthConfig
