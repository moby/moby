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

package plugin

import (
	"github.com/containerd/nri/pkg/api"
)

const (
	// AnnotationDomain is the domain used for NRI-specific annotations.
	AnnotationDomain = "noderesource.dev"

	// RequiredPluginsAnnotation can be used to annotate pods with a list
	// of pod- or container-specific plugins which must process containers
	// during creation. If enabled, the default validator checks for this
	// and rejects the creation of containers which fail this check.
	RequiredPluginsAnnotation = "required-plugins." + AnnotationDomain
)

// GetEffectiveAnnotation retrieves a custom annotation from a pod which
// applies to given container. The syntax allows both pod- and container-
// scoped annotations. Container-scoped annotations take precedence over
// pod-scoped ones. The key syntax defines the scope of the annotation.
//   - container-scope: <key>/container.<container-name>
//   - pod-scope: <key>/pod, or just <key>
func GetEffectiveAnnotation(pod *api.PodSandbox, key, container string) (string, bool) {
	annotations := pod.GetAnnotations()
	if len(annotations) == 0 {
		return "", false
	}

	keys := []string{
		key + "/container." + container,
		key + "/pod",
		key,
	}

	for _, k := range keys {
		if v, ok := annotations[k]; ok {
			return v, true
		}
	}

	return "", false
}
