/*
   Copyright © The CDI Authors

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

package validation

import (
	"fmt"
	"strings"

	"github.com/container-orchestrated-devices/container-device-interface/internal/validation/k8s"
)

// ValidateSpecAnnotations checks whether spec annotations are valid.
func ValidateSpecAnnotations(name string, any interface{}) error {
	if any == nil {
		return nil
	}

	switch v := any.(type) {
	case map[string]interface{}:
		annotations := make(map[string]string)
		for k, v := range v {
			if s, ok := v.(string); ok {
				annotations[k] = s
			} else {
				return fmt.Errorf("invalid annotation %v.%v; %v is not a string", name, k, any)
			}
		}
		return validateSpecAnnotations(name, annotations)
	}

	return nil
}

// validateSpecAnnotations checks whether spec annotations are valid.
func validateSpecAnnotations(name string, annotations map[string]string) error {
	path := "annotations"
	if name != "" {
		path = strings.Join([]string{name, path}, ".")
	}

	return k8s.ValidateAnnotations(annotations, path)
}
