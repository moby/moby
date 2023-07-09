package dockerfile // import "github.com/docker/docker/builder/dockerfile"

import (
	"fmt"
	"io"
	"sort"

	"github.com/docker/docker/runconfig/opts"
)

// builtinAllowedBuildArgs is list of built-in allowed build args
// these args are considered transparent and are excluded from the image history.
// Filtering from history is implemented in dispatchers.go
var builtinAllowedBuildArgs = map[string]bool{
	"HTTP_PROXY":  true,
	"http_proxy":  true,
	"HTTPS_PROXY": true,
	"https_proxy": true,
	"FTP_PROXY":   true,
	"ftp_proxy":   true,
	"NO_PROXY":    true,
	"no_proxy":    true,
	"ALL_PROXY":   true,
	"all_proxy":   true,
}

// BuildArgs manages arguments used by the builder
type BuildArgs struct {
	// args that are allowed for expansion/substitution and passing to commands in 'run'.
	allowedBuildArgs map[string]*string
	// args defined before the first `FROM` in a Dockerfile
	allowedMetaArgs map[string]*string
	// args referenced by the Dockerfile
	referencedArgs map[string]struct{}
	// args provided by the user on the command line
	argsFromOptions map[string]*string
}

// NewBuildArgs creates a new BuildArgs type
func NewBuildArgs(argsFromOptions map[string]*string) *BuildArgs {
	return &BuildArgs{
		allowedBuildArgs: make(map[string]*string),
		allowedMetaArgs:  make(map[string]*string),
		referencedArgs:   make(map[string]struct{}),
		argsFromOptions:  argsFromOptions,
	}
}

// Clone returns a copy of the BuildArgs type
func (b *BuildArgs) Clone() *BuildArgs {
	result := NewBuildArgs(b.argsFromOptions)
	for k, v := range b.allowedBuildArgs {
		result.allowedBuildArgs[k] = v
	}
	for k, v := range b.allowedMetaArgs {
		result.allowedMetaArgs[k] = v
	}
	for k := range b.referencedArgs {
		result.referencedArgs[k] = struct{}{}
	}
	return result
}

// MergeReferencedArgs merges referenced args from another BuildArgs
// object into the current one
func (b *BuildArgs) MergeReferencedArgs(other *BuildArgs) {
	for k := range other.referencedArgs {
		b.referencedArgs[k] = struct{}{}
	}
}

// WarnOnUnusedBuildArgs checks if there are any leftover build-args that were
// passed but not consumed during build. Print a warning, if there are any.
func (b *BuildArgs) WarnOnUnusedBuildArgs(out io.Writer) {
	var leftoverArgs []string
	for arg := range b.argsFromOptions {
		_, isReferenced := b.referencedArgs[arg]
		_, isBuiltin := builtinAllowedBuildArgs[arg]
		if !isBuiltin && !isReferenced {
			leftoverArgs = append(leftoverArgs, arg)
		}
	}
	if len(leftoverArgs) > 0 {
		sort.Strings(leftoverArgs)
		fmt.Fprintf(out, "[Warning] One or more build-args %v were not consumed\n", leftoverArgs)
	}
}

// ResetAllowed clears the list of args that are allowed to be used by a
// directive
func (b *BuildArgs) ResetAllowed() {
	b.allowedBuildArgs = make(map[string]*string)
}

// AddMetaArg adds a new meta arg that can be used by FROM directives
func (b *BuildArgs) AddMetaArg(key string, value *string) {
	b.allowedMetaArgs[key] = value
}

// AddArg adds a new arg that can be used by directives
func (b *BuildArgs) AddArg(key string, value *string) {
	b.allowedBuildArgs[key] = value
	b.referencedArgs[key] = struct{}{}
}

// IsReferencedOrNotBuiltin checks if the key is a built-in arg, or if it has been
// referenced by the Dockerfile. Returns true if the arg is not a builtin or
// if the builtin has been referenced in the Dockerfile.
func (b *BuildArgs) IsReferencedOrNotBuiltin(key string) bool {
	_, isBuiltin := builtinAllowedBuildArgs[key]
	_, isAllowed := b.allowedBuildArgs[key]
	return isAllowed || !isBuiltin
}

// GetAllAllowed returns a mapping with all the allowed args
func (b *BuildArgs) GetAllAllowed() map[string]string {
	return b.getAllFromMapping(b.allowedBuildArgs)
}

// GetAllMeta returns a mapping with all the meta args
func (b *BuildArgs) GetAllMeta() map[string]string {
	return b.getAllFromMapping(b.allowedMetaArgs)
}

func (b *BuildArgs) getAllFromMapping(source map[string]*string) map[string]string {
	m := make(map[string]string)

	keys := keysFromMaps(source, builtinAllowedBuildArgs)
	for _, key := range keys {
		v, ok := b.getBuildArg(key, source)
		if ok {
			m[key] = v
		}
	}
	return m
}

// FilterAllowed returns all allowed args without the filtered args
func (b *BuildArgs) FilterAllowed(filter []string) []string {
	envs := []string{}
	configEnv := opts.ConvertKVStringsToMap(filter)

	for key, val := range b.GetAllAllowed() {
		if _, ok := configEnv[key]; !ok {
			envs = append(envs, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return envs
}

func (b *BuildArgs) getBuildArg(key string, mapping map[string]*string) (string, bool) {
	defaultValue, exists := mapping[key]
	// Return override from options if one is defined
	if v, ok := b.argsFromOptions[key]; ok && v != nil {
		return *v, ok
	}

	if defaultValue == nil {
		if v, ok := b.allowedMetaArgs[key]; ok && v != nil {
			return *v, ok
		}
		return "", false
	}
	return *defaultValue, exists
}

func keysFromMaps(source map[string]*string, builtin map[string]bool) []string {
	keys := []string{}
	for key := range source {
		keys = append(keys, key)
	}
	for key := range builtin {
		keys = append(keys, key)
	}
	return keys
}
