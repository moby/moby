/*
Copyright 2021 Intel Corporation

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

package rdt

import (
	"fmt"

	"github.com/intel/goresctrl/pkg/kubernetes"
)

const (
	// RdtContainerAnnotation is the CRI level container annotation for setting
	// the RDT class (CLOS) of a container
	RdtContainerAnnotation = "io.kubernetes.cri.rdt-class"

	// RdtPodAnnotation is a Pod annotation for setting the RDT class (CLOS) of
	// all containers of the pod
	RdtPodAnnotation = "rdt.resources.beta.kubernetes.io/pod"

	// RdtPodAnnotationContainerPrefix is prefix for per-container Pod annotation
	// for setting the RDT class (CLOS) of one container of the pod
	RdtPodAnnotationContainerPrefix = "rdt.resources.beta.kubernetes.io/container."
)

// ContainerClassFromAnnotations determines the effective RDT class of a
// container from the Pod annotations and CRI level container annotations of a
// container. Verifies that the class exists in goresctrl configuration and that
// it is allowed to be used.
func ContainerClassFromAnnotations(containerName string, containerAnnotations, podAnnotations map[string]string) (string, error) {
	clsName, clsOrigin := kubernetes.ContainerClassFromAnnotations(
		RdtContainerAnnotation, RdtPodAnnotation, RdtPodAnnotationContainerPrefix,
		containerName, containerAnnotations, podAnnotations)

	if clsOrigin != kubernetes.ClassOriginNotFound {
		if rdt == nil {
			return "", fmt.Errorf("RDT not initialized, class %q not available", clsName)
		}

		// Verify validity of class name
		if !IsQualifiedClassName(clsName) {
			return "", fmt.Errorf("unqualified RDT class name %q", clsName)
		}

		// If RDT has been initialized we check that the class exists
		if _, err := rdt.getClass(clsName); err != nil {
			return "", fmt.Errorf("RDT class not found: %w", err)
		}

		// If classes have been configured by goresctrl
		if clsConf, ok := rdt.conf.Classes[unaliasClassName(clsName)]; ok {
			// Check that the class is allowed
			if clsOrigin == kubernetes.ClassOriginPodAnnotation && clsConf.Kubernetes.DenyPodAnnotation {
				return "", fmt.Errorf("RDT class %q not allowed from Pod annotations", clsName)
			} else if clsOrigin == kubernetes.ClassOriginContainerAnnotation && clsConf.Kubernetes.DenyContainerAnnotation {
				return "", fmt.Errorf("RDT class %q not allowed from Container annotation", clsName)
			}
		}
	}

	return clsName, nil
}
