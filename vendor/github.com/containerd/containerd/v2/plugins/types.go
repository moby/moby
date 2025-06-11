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

// plugins package stores all the plugin types used by containerd internally.
//
// External plugins should copy from these types and avoid importing this
// package.
package plugins

import "github.com/containerd/plugin"

const (
	// InternalPlugin implements an internal plugin to containerd
	InternalPlugin plugin.Type = "io.containerd.internal.v1"
	// RuntimePlugin implements a runtime
	RuntimePlugin plugin.Type = "io.containerd.runtime.v1"
	// RuntimePluginV2 implements a runtime v2
	RuntimePluginV2 plugin.Type = "io.containerd.runtime.v2"
	// ServicePlugin implements an internal service
	ServicePlugin plugin.Type = "io.containerd.service.v1"
	// GRPCPlugin implements a grpc service
	GRPCPlugin plugin.Type = "io.containerd.grpc.v1"
	// TTRPCPlugin implements a ttrpc shim service
	TTRPCPlugin plugin.Type = "io.containerd.ttrpc.v1"
	// SnapshotPlugin implements a snapshotter
	SnapshotPlugin plugin.Type = "io.containerd.snapshotter.v1"
	// TaskMonitorPlugin implements a task monitor
	TaskMonitorPlugin plugin.Type = "io.containerd.monitor.task.v1"
	// TaskMonitorPlugin implements a container monitor
	ContainerMonitorPlugin plugin.Type = "io.containerd.monitor.container.v1"
	// DiffPlugin implements a differ
	DiffPlugin plugin.Type = "io.containerd.differ.v1"
	// MetadataPlugin implements a metadata store
	MetadataPlugin plugin.Type = "io.containerd.metadata.v1"
	// ContentPlugin implements a content store
	ContentPlugin plugin.Type = "io.containerd.content.v1"
	// GCPlugin implements garbage collection policy
	GCPlugin plugin.Type = "io.containerd.gc.v1"
	// EventPlugin implements event handling
	EventPlugin plugin.Type = "io.containerd.event.v1"
	// LeasePlugin implements lease manager
	LeasePlugin plugin.Type = "io.containerd.lease.v1"
	// StreamingPlugin implements a stream manager
	StreamingPlugin plugin.Type = "io.containerd.streaming.v1"
	// TracingProcessorPlugin implements an open telemetry span processor
	TracingProcessorPlugin plugin.Type = "io.containerd.tracing.processor.v1"
	// NRIApiPlugin implements the NRI adaptation interface for containerd.
	NRIApiPlugin plugin.Type = "io.containerd.nri.v1"
	// TransferPlugin implements a transfer service
	TransferPlugin plugin.Type = "io.containerd.transfer.v1"
	// SandboxStorePlugin implements a sandbox store
	SandboxStorePlugin plugin.Type = "io.containerd.sandbox.store.v1"
	// PodSandboxPlugin is a special sandbox controller which use pause container as a sandbox.
	PodSandboxPlugin plugin.Type = "io.containerd.podsandbox.controller.v1"
	// SandboxControllerPlugin implements a sandbox controller
	SandboxControllerPlugin plugin.Type = "io.containerd.sandbox.controller.v1"
	// ImageVerifierPlugin implements an image verifier service
	ImageVerifierPlugin plugin.Type = "io.containerd.image-verifier.v1"
	// WarningPlugin implements a warning service
	WarningPlugin plugin.Type = "io.containerd.warning.v1"
	// CRIServicePlugin implements a cri service
	CRIServicePlugin plugin.Type = "io.containerd.cri.v1"
	// ShimPlugin implements a shim service
	ShimPlugin plugin.Type = "io.containerd.shim.v1"
	// HTTPHandler implements an http handler
	HTTPHandler plugin.Type = "io.containerd.http.v1"
)

const (
	// RuntimeRuncV2 is the runc runtime that supports multiple containers per shim
	RuntimeRuncV2 = "io.containerd.runc.v2"

	// RuntimeRunhcsV1 is the runtime type for runhcs.
	RuntimeRunhcsV1 = "io.containerd.runhcs.v1"

	DeprecationsPlugin = "deprecations"
)

const (
	// PropertyRootDir sets the root directory property for a plugin
	PropertyRootDir = "io.containerd.plugin.root"
	// PropertyStateDir sets the state directory property for a plugin
	PropertyStateDir = "io.containerd.plugin.state"
	// PropertyGRPCAddress is the grpc address used for client connections to containerd
	PropertyGRPCAddress = "io.containerd.plugin.grpc.address"
	// PropertyTTRPCAddress is the ttrpc address used for client connections to containerd
	PropertyTTRPCAddress = "io.containerd.plugin.ttrpc.address"
)

const (
	SnapshotterRootDir = "root"
)
