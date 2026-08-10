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

package blockio

import (
	"github.com/intel/goresctrl/pkg/kubernetes"
)

const (
	// BlockioContainerAnnotation is the CRI level container annotation for setting
	// the blockio class of a container
	BlockioContainerAnnotation = "io.kubernetes.cri.blockio-class"

	// BlockioPodAnnotation is a Pod annotation for setting the blockio class of
	// all containers of the pod
	BlockioPodAnnotation = "blockio.resources.beta.kubernetes.io/pod"

	// BlockioPodAnnotationContainerPrefix is prefix for per-container Pod annotation
	// for setting the blockio class of one container of the pod
	BlockioPodAnnotationContainerPrefix = "blockio.resources.beta.kubernetes.io/container."
)

// ContainerClassFromAnnotations determines the effective blockio
// class of a container from the Pod annotations and CRI level
// container annotations of a container. If the class is not specified
// by any annotation, returns empty class name. Returned error is
// reserved (nil).
func ContainerClassFromAnnotations(containerName string, containerAnnotations, podAnnotations map[string]string) (string, error) {
	clsName, _ := kubernetes.ContainerClassFromAnnotations(
		BlockioContainerAnnotation, BlockioPodAnnotation, BlockioPodAnnotationContainerPrefix,
		containerName, containerAnnotations, podAnnotations)
	return clsName, nil
}
