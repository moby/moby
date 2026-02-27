package shimopts

import (
	runhcsoptions "github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	runcoptions "github.com/containerd/containerd/api/types/runc/options"
	runtimeoptions "github.com/containerd/containerd/api/types/runtimeoptions/v1"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/pelletier/go-toml/v2"
)

// Generate converts opts into a runtime options value for the runtimeType which
// can be passed into containerd.
func Generate(runtimeType string, opts map[string]any) (any, error) {
	// This is horrible, but we have no other choice. The containerd client
	// can only handle options values which can be marshaled into a
	// typeurl.Any. And we're in good company: cri-containerd handles shim
	// options in the same way.
	var out any
	switch runtimeType {
	case plugins.RuntimeRuncV2:
		out = &runcoptions.Options{}
	case "io.containerd.runhcs.v1":
		out = &runhcsoptions.Options{}
	default:
		out = &runtimeoptions.Options{}
	}

	// We can't use mergo.Map as it is too strict about type-assignability
	// with numeric types.
	b, err := toml.Marshal(opts)
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(b, out); err != nil {
		return nil, err
	}
	return out, nil
}
