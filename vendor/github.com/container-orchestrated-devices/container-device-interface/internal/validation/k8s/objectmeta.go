/*
Copyright 2014 The Kubernetes Authors.

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

// Adapted from k8s.io/apimachinery/pkg/api/validation:
// https://github.com/kubernetes/apimachinery/blob/7687996c715ee7d5c8cf1e3215e607eb065a4221/pkg/api/validation/objectmeta.go

package k8s

import (
	"fmt"
	"strings"

	"github.com/container-orchestrated-devices/container-device-interface/internal/multierror"
)

// TotalAnnotationSizeLimitB defines the maximum size of all annotations in characters.
const TotalAnnotationSizeLimitB int = 256 * (1 << 10) // 256 kB

// ValidateAnnotations validates that a set of annotations are correctly defined.
func ValidateAnnotations(annotations map[string]string, path string) error {
	errors := multierror.New()
	for k := range annotations {
		// The rule is QualifiedName except that case doesn't matter, so convert to lowercase before checking.
		for _, msg := range IsQualifiedName(strings.ToLower(k)) {
			errors = multierror.Append(errors, fmt.Errorf("%v.%v is invalid: %v", path, k, msg))
		}
	}
	if err := ValidateAnnotationsSize(annotations); err != nil {
		errors = multierror.Append(errors, fmt.Errorf("%v is too long: %v", path, err))
	}
	return errors
}

// ValidateAnnotationsSize validates that a set of annotations is not too large.
func ValidateAnnotationsSize(annotations map[string]string) error {
	var totalSize int64
	for k, v := range annotations {
		totalSize += (int64)(len(k)) + (int64)(len(v))
	}
	if totalSize > (int64)(TotalAnnotationSizeLimitB) {
		return fmt.Errorf("annotations size %d is larger than limit %d", totalSize, TotalAnnotationSizeLimitB)
	}
	return nil
}
