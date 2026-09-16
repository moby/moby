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

	"tags.cncf.io/container-device-interface/internal/validation/k8s"
)

// ValidateSpecAnnotations checks whether spec annotations are valid.
func ValidateSpecAnnotations(name string, specAnnotations any) error {
	values, ok := specAnnotations.(map[string]any)
	if !ok {
		return nil
	}

	annotations := make(map[string]string, len(values))
	for k, v := range values {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("invalid annotation %v.%v; %v is not a string", name, k, v)
		}
		annotations[k] = s
	}

	path := "annotations"
	if name != "" {
		path = name + "." + path
	}
	return k8s.ValidateAnnotations(annotations, path)
}
