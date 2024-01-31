/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package deprecation

type Warning string

const (
	// Prefix is a standard prefix for all Warnings, used for filtering plugin Exports
	Prefix = "io.containerd.deprecation/"
	// PullSchema1Image is a warning for the use of schema 1 images
	PullSchema1Image Warning = Prefix + "pull-schema-1-image"
	// GoPluginLibrary is a warning for the use of dynamic library Go plugins
	GoPluginLibrary Warning = Prefix + "go-plugin-library"
	// CRIRegistryMirrors is a warning for the use of the `mirrors` property
	CRIRegistryMirrors Warning = Prefix + "cri-registry-mirrors"
	// CRIRegistryAuths is a warning for the use of the `auths` property
	CRIRegistryAuths Warning = Prefix + "cri-registry-auths"
	// CRIRegistryConfigs is a warning for the use of the `configs` property
	CRIRegistryConfigs Warning = Prefix + "cri-registry-configs"
)

var messages = map[Warning]string{
	PullSchema1Image: "Schema 1 images are deprecated since containerd v1.7 and removed in containerd v2.0. " +
		`Since containerd v1.7.8, schema 1 images are identified by the "io.containerd.image/converted-docker-schema1" label.`,
	GoPluginLibrary: "Dynamically-linked Go plugins as containerd runtimes are deprecated since containerd v2.0 and removed in containerd v2.1.",
	CRIRegistryMirrors: "The `mirrors` property of `[plugins.\"io.containerd.grpc.v1.cri\".registry]` is deprecated since containerd v1.5 and will be removed in containerd v2.0." +
		"Use `config_path` instead.",
	CRIRegistryAuths: "The `auths` property of `[plugins.\"io.containerd.grpc.v1.cri\".registry]` is deprecated since containerd v1.3 and will be removed in containerd v2.0." +
		"Use `ImagePullSecrets` instead.",
	CRIRegistryConfigs: "The `configs` property of `[plugins.\"io.containerd.grpc.v1.cri\".registry]` is deprecated since containerd v1.5 and will be removed in containerd v2.0." +
		"Use `config_path` instead.",
}

// Valid checks whether a given Warning is valid
func Valid(id Warning) bool {
	_, ok := messages[id]
	return ok
}

// Message returns the human-readable message for a given Warning
func Message(id Warning) (string, bool) {
	msg, ok := messages[id]
	return msg, ok
}
