package runtime

// Runtime describes an OCI runtime configuration
type Runtime struct {
	Path string   `json:"path"`
	Args []string `json:"runtimeArgs,omitempty"`
}

type Info struct {
	Name           string
	DefaultRuntime bool
	Runtime        Runtime
}

const (
	GetPath = "RuntimeDriver.Path"
	GetArgs = "RuntimeDriver.Args"
)

type GetRuntimesResponse struct {
	Runtimes []Info `json:"runtimes"`
}

type PluginPathResponse struct {
	Path string `json:"path"`
}

type PluginArgsResponse struct {
	Args []string `json:"args,omitempty"`
}
