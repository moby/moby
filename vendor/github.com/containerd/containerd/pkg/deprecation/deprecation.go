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
	// CRISystemdCgroupV1 is a warning for the `systemd_cgroup` property
	CRISystemdCgroupV1 Warning = Prefix + "cri-systemd-cgroup-v1"
	// CRIUntrustedWorkloadRuntime is a warning for the `untrusted_workload_runtime` property
	CRIUntrustedWorkloadRuntime Warning = Prefix + "cri-untrusted-workload-runtime"
	// CRIDefaultRuntime is a warning for the `default_runtime` property
	CRIDefaultRuntime Warning = Prefix + "cri-default-runtime"
	// CRIRuntimeEngine is a warning for the `runtime_engine` property
	CRIRuntimeEngine Warning = Prefix + "cri-runtime-engine"
	// CRIRuntimeRoot is a warning for the `runtime_root` property
	CRIRuntimeRoot Warning = Prefix + "cri-runtime-root"
	// CRIRegistryMirrors is a warning for the use of the `mirrors` property
	CRIRegistryMirrors Warning = Prefix + "cri-registry-mirrors"
	// CRIRegistryAuths is a warning for the use of the `auths` property
	CRIRegistryAuths Warning = Prefix + "cri-registry-auths"
	// CRIRegistryConfigs is a warning for the use of the `configs` property
	CRIRegistryConfigs Warning = Prefix + "cri-registry-configs"
	// CRIAPIV1Alpha2 is a warning for the use of CRI-API v1alpha2
	CRIAPIV1Alpha2 Warning = Prefix + "cri-api-v1alpha2"
	// AUFSSnapshotter is a warning for the use of the aufs snapshotter
	AUFSSnapshotter Warning = Prefix + "aufs-snapshotter"
	// RestartLogpath is a warning for the containerd.io/restart.logpath label
	RestartLogpath Warning = Prefix + "restart-logpath"
	// RuntimeV1 is a warning for the io.containerd.runtime.v1.linux runtime
	RuntimeV1 Warning = Prefix + "runtime-v1"
	// RuntimeRuncV1 is a warning for the io.containerd.runc.v1 runtime
	RuntimeRuncV1 Warning = Prefix + "runtime-runc-v1"
	// CRICRIUPath is a warning for the use of the `CriuPath` property
	CRICRIUPath Warning = Prefix + "cri-criu-path"
	// OTLPTracingConfig is a warning for the use of the `otlp` property
	TracingOTLPConfig Warning = Prefix + "tracing-processor-config"
	// TracingServiceConfig is a warning for the use of the `tracing` property
	TracingServiceConfig Warning = Prefix + "tracing-service-config"
)

var messages = map[Warning]string{
	PullSchema1Image: "Schema 1 images are deprecated since containerd v1.7 and removed in containerd v2.0. " +
		`Since containerd v1.7.8, schema 1 images are identified by the "io.containerd.image/converted-docker-schema1" label.`,
	GoPluginLibrary: "Dynamically-linked Go plugins as containerd runtimes will be deprecated in containerd v2.0 and removed in containerd v2.1.",
	CRISystemdCgroupV1: "The `systemd_cgroup` property (old form) of `[plugins.\"io.containerd.grpc.v1.cri\"] is deprecated since containerd v1.3 and will be removed in containerd v2.0. " +
		"Use `SystemdCgroup` in [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.runc.options] options instead.",
	CRIUntrustedWorkloadRuntime: "The `untrusted_workload_runtime` property of [plugins.\"io.containerd.grpc.v1.cri\".containerd] is deprecated since containerd v1.2 and will be removed in containerd v2.0. " +
		"Create an `untrusted` runtime in `runtimes` instead.",
	CRIDefaultRuntime: "The `default_runtime` property of [plugins.\"io.containerd.grpc.v1.cri\".containerd] is deprecated since containerd v1.3 and will be removed in containerd v2.0. " +
		"Use `default_runtime_name` instead.",
	CRIRuntimeEngine: "The `runtime_engine` property of [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.*] is deprecated since containerd v1.3 and will be removed in containerd v2.0. " +
		"Use a v2 runtime and `options` instead.",
	CRIRuntimeRoot: "The `runtime_root` property of [plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.*] is deprecated since containerd v1.3 and will be removed in containerd v2.0. " +
		"Use a v2 runtime and `options.Root` instead.",
	CRIRegistryMirrors: "The `mirrors` property of `[plugins.\"io.containerd.grpc.v1.cri\".registry]` is deprecated since containerd v1.5 and will be removed in containerd v2.0. " +
		"Use `config_path` instead.",
	CRIRegistryAuths: "The `auths` property of `[plugins.\"io.containerd.grpc.v1.cri\".registry]` is deprecated since containerd v1.3 and will be removed in containerd v2.0. " +
		"Use `ImagePullSecrets` instead.",
	CRIRegistryConfigs: "The `configs` property of `[plugins.\"io.containerd.grpc.v1.cri\".registry]` is deprecated since containerd v1.5 and will be removed in containerd v2.0. " +
		"Use `config_path` instead.",
	CRIAPIV1Alpha2:  "CRI API v1alpha2 is deprecated since containerd v1.7 and removed in containerd v2.0. Use CRI API v1 instead.",
	AUFSSnapshotter: "The aufs snapshotter is deprecated since containerd v1.5 and removed in containerd v2.0. Use the overlay snapshotter instead.",
	RestartLogpath:  "The `containerd.io/restart.logpath` label is deprecated since containerd v1.5 and removed in containerd v2.0. Use `containerd.io/restart.loguri` instead.",
	RuntimeV1:       "The `io.containerd.runtime.v1.linux` runtime is deprecated since containerd v1.4 and removed in containerd v2.0. Use the `io.containerd.runc.v2` runtime instead.",
	RuntimeRuncV1:   "The `io.containerd.runc.v1` runtime is deprecated since containerd v1.4 and removed in containerd v2.0. Use the `io.containerd.runc.v2` runtime instead.",
	CRICRIUPath: "The `CriuPath` property of `[plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.*.options]` is deprecated since containerd v1.7 and will be removed in containerd v2.0. " +
		"Use a criu binary in $PATH instead.",
	TracingOTLPConfig: "The `otlp` property of `[plugins.\"io.containerd.tracing.processor.v1\".otlp]` is deprecated since containerd v1.6 and will be removed in containerd v2.0." +
		"Use OTLP environment variables instead: https://opentelemetry.io/docs/specs/otel/protocol/exporter/",
	TracingServiceConfig: "The `tracing` property of `[plugins.\"io.containerd.internal.v1\".tracing]` is deprecated since containerd v1.6 and will be removed in containerd v2.0." +
		"Use OTEL environment variables instead: https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/",
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
