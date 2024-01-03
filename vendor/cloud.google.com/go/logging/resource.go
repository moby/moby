// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"runtime"
	"strings"
	"sync"

	"cloud.google.com/go/logging/internal"
	mrpb "google.golang.org/genproto/googleapis/api/monitoredres"
)

// CommonResource sets the monitored resource associated with all log entries
// written from a Logger. If not provided, the resource is automatically
// detected based on the running environment (on GCE, GCR, GCF and GAE Standard only).
// This value can be overridden per-entry by setting an Entry's Resource field.
func CommonResource(r *mrpb.MonitoredResource) LoggerOption { return commonResource{r} }

type commonResource struct{ *mrpb.MonitoredResource }

func (r commonResource) set(l *Logger) { l.commonResource = r.MonitoredResource }

type resource struct {
	pb    *mrpb.MonitoredResource
	attrs internal.ResourceAttributesGetter
	once  *sync.Once
}

var detectedResource = &resource{
	attrs: internal.ResourceAttributes(),
	once:  new(sync.Once),
}

func (r *resource) metadataProjectID() string {
	return r.attrs.Metadata("project/project-id")
}

func (r *resource) metadataZone() string {
	zone := r.attrs.Metadata("instance/zone")
	if zone != "" {
		return zone[strings.LastIndex(zone, "/")+1:]
	}
	return ""
}

func (r *resource) metadataRegion() string {
	region := r.attrs.Metadata("instance/region")
	if region != "" {
		return region[strings.LastIndex(region, "/")+1:]
	}
	return ""
}

// isMetadataActive queries valid response on "/computeMetadata/v1/" URL
func (r *resource) isMetadataActive() bool {
	data := r.attrs.Metadata("")
	return data != ""
}

// isAppEngine returns true for both standard and flex
func (r *resource) isAppEngine() bool {
	service := r.attrs.EnvVar("GAE_SERVICE")
	version := r.attrs.EnvVar("GAE_VERSION")
	instance := r.attrs.EnvVar("GAE_INSTANCE")
	return service != "" && version != "" && instance != ""
}

func detectAppEngineResource() *mrpb.MonitoredResource {
	projectID := detectedResource.metadataProjectID()
	if projectID == "" {
		projectID = detectedResource.attrs.EnvVar("GOOGLE_CLOUD_PROJECT")
	}
	if projectID == "" {
		return nil
	}
	zone := detectedResource.metadataZone()
	service := detectedResource.attrs.EnvVar("GAE_SERVICE")
	version := detectedResource.attrs.EnvVar("GAE_VERSION")

	return &mrpb.MonitoredResource{
		Type: "gae_app",
		Labels: map[string]string{
			"project_id": projectID,
			"module_id":  service,
			"version_id": version,
			"zone":       zone,
		},
	}
}

func (r *resource) isCloudFunction() bool {
	target := r.attrs.EnvVar("FUNCTION_TARGET")
	signature := r.attrs.EnvVar("FUNCTION_SIGNATURE_TYPE")
	// note that this envvar is also present in Cloud Run environments
	service := r.attrs.EnvVar("K_SERVICE")
	return target != "" && signature != "" && service != ""
}

func detectCloudFunction() *mrpb.MonitoredResource {
	projectID := detectedResource.metadataProjectID()
	if projectID == "" {
		return nil
	}
	region := detectedResource.metadataRegion()
	functionName := detectedResource.attrs.EnvVar("K_SERVICE")
	return &mrpb.MonitoredResource{
		Type: "cloud_function",
		Labels: map[string]string{
			"project_id":    projectID,
			"region":        region,
			"function_name": functionName,
		},
	}
}

func (r *resource) isCloudRun() bool {
	config := r.attrs.EnvVar("K_CONFIGURATION")
	// note that this envvar is also present in Cloud Function environments
	service := r.attrs.EnvVar("K_SERVICE")
	revision := r.attrs.EnvVar("K_REVISION")
	return config != "" && service != "" && revision != ""
}

func detectCloudRunResource() *mrpb.MonitoredResource {
	projectID := detectedResource.metadataProjectID()
	if projectID == "" {
		return nil
	}
	region := detectedResource.metadataRegion()
	config := detectedResource.attrs.EnvVar("K_CONFIGURATION")
	service := detectedResource.attrs.EnvVar("K_SERVICE")
	revision := detectedResource.attrs.EnvVar("K_REVISION")
	return &mrpb.MonitoredResource{
		Type: "cloud_run_revision",
		Labels: map[string]string{
			"project_id":         projectID,
			"location":           region,
			"service_name":       service,
			"revision_name":      revision,
			"configuration_name": config,
		},
	}
}

func (r *resource) isKubernetesEngine() bool {
	clusterName := r.attrs.Metadata("instance/attributes/cluster-name")
	if clusterName == "" {
		return false
	}
	return true
}

func detectKubernetesResource() *mrpb.MonitoredResource {
	projectID := detectedResource.metadataProjectID()
	if projectID == "" {
		return nil
	}
	zone := detectedResource.metadataZone()
	clusterName := detectedResource.attrs.Metadata("instance/attributes/cluster-name")
	namespaceName := detectedResource.attrs.ReadAll("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if namespaceName == "" {
		// if automountServiceAccountToken is disabled allow to customize
		// the namespace via environment
		namespaceName = detectedResource.attrs.EnvVar("NAMESPACE_NAME")
	}
	// note: if deployment customizes hostname, HOSTNAME envvar will have invalid content
	podName := detectedResource.attrs.EnvVar("HOSTNAME")
	// there is no way to derive container name from within container; use custom envvar if available
	containerName := detectedResource.attrs.EnvVar("CONTAINER_NAME")
	return &mrpb.MonitoredResource{
		Type: "k8s_container",
		Labels: map[string]string{
			"cluster_name":   clusterName,
			"location":       zone,
			"project_id":     projectID,
			"pod_name":       podName,
			"namespace_name": namespaceName,
			"container_name": containerName,
		},
	}
}

func (r *resource) isComputeEngine() bool {
	preempted := r.attrs.Metadata("instance/preempted")
	platform := r.attrs.Metadata("instance/cpu-platform")
	appBucket := r.attrs.Metadata("instance/attributes/gae_app_bucket")
	return preempted != "" && platform != "" && appBucket == ""
}

func detectComputeEngineResource() *mrpb.MonitoredResource {
	projectID := detectedResource.metadataProjectID()
	if projectID == "" {
		return nil
	}
	id := detectedResource.attrs.Metadata("instance/id")
	zone := detectedResource.metadataZone()
	return &mrpb.MonitoredResource{
		Type: "gce_instance",
		Labels: map[string]string{
			"project_id":  projectID,
			"instance_id": id,
			"zone":        zone,
		},
	}
}

func detectResource() *mrpb.MonitoredResource {
	detectedResource.once.Do(func() {
		if detectedResource.isMetadataActive() {
			name := systemProductName()
			switch {
			case name == "Google App Engine", detectedResource.isAppEngine():
				detectedResource.pb = detectAppEngineResource()
			case name == "Google Cloud Functions", detectedResource.isCloudFunction():
				detectedResource.pb = detectCloudFunction()
			case name == "Google Cloud Run", detectedResource.isCloudRun():
				detectedResource.pb = detectCloudRunResource()
			// cannot use name validation for GKE and GCE because
			// both of them set product name to "Google Compute Engine"
			case detectedResource.isKubernetesEngine():
				detectedResource.pb = detectKubernetesResource()
			case detectedResource.isComputeEngine():
				detectedResource.pb = detectComputeEngineResource()
			}
		}
	})
	return detectedResource.pb
}

// systemProductName reads resource type on the Linux-based environments such as
// Cloud Functions, Cloud Run, GKE, GCE, GAE, etc.
func systemProductName() string {
	if runtime.GOOS != "linux" {
		// We don't have any non-Linux clues available, at least yet.
		return ""
	}
	slurp := detectedResource.attrs.ReadAll("/sys/class/dmi/id/product_name")
	return strings.TrimSpace(slurp)
}

var resourceInfo = map[string]struct{ rtype, label string }{
	"organizations":   {"organization", "organization_id"},
	"folders":         {"folder", "folder_id"},
	"projects":        {"project", "project_id"},
	"billingAccounts": {"billing_account", "account_id"},
}

func monitoredResource(parent string) *mrpb.MonitoredResource {
	parts := strings.SplitN(parent, "/", 2)
	if len(parts) != 2 {
		return globalResource(parent)
	}
	info, ok := resourceInfo[parts[0]]
	if !ok {
		return globalResource(parts[1])
	}
	return &mrpb.MonitoredResource{
		Type:   info.rtype,
		Labels: map[string]string{info.label: parts[1]},
	}
}

func globalResource(projectID string) *mrpb.MonitoredResource {
	return &mrpb.MonitoredResource{
		Type: "global",
		Labels: map[string]string{
			"project_id": projectID,
		},
	}
}
