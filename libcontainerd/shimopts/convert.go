package shimopts

import (
	runhcsoptions "github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	runtimeoptions "github.com/containerd/containerd/pkg/runtimeoptions/v1"
	"github.com/containerd/containerd/plugin"
	runcoptions "github.com/containerd/containerd/runtime/v2/runc/options"
	"github.com/pelletier/go-toml"
)

// Generate converts opts into a runtime options value for the runtimeType which
// can be passed into containerd.
func Generate(runtimeType string, opts map[string]interface{}) (interface{}, error) {
	// This is horrible, but we have no other choice. The containerd client
	// can only handle options values which can be marshaled into a
	// typeurl.Any. And we're in good company: cri-containerd handles shim
	// options in the same way.
	var out interface{}
	switch runtimeType {
	case plugin.RuntimeRuncV1, plugin.RuntimeRuncV2:
		out = &runcoptions.Options{}
	case "io.containerd.runhcs.v1":
		out = &runhcsoptions.Options{}
	default:
		out = &runtimeoptions.Options{}
	}

	// We can't use mergo.Map as it is too strict about type-assignability
	// with numeric types.
	tree, err := toml.TreeFromMap(opts)
	if err != nil {
		return nil, err
	}
	if err := tree.Unmarshal(out); err != nil {
		return nil, err
	}
	return out, nil
}
