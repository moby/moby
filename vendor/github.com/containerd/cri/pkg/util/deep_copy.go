/*
Copyright 2017 The Kubernetes Authors.

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

package util

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// DeepCopy makes a deep copy from src into dst.
func DeepCopy(dst interface{}, src interface{}) error {
	if dst == nil {
		return errors.New("dst cannot be nil")
	}
	if src == nil {
		return errors.New("src cannot be nil")
	}
	bytes, err := json.Marshal(src)
	if err != nil {
		return errors.Wrap(err, "unable to marshal src")
	}
	err = json.Unmarshal(bytes, dst)
	if err != nil {
		return errors.Wrap(err, "unable to unmarshal into dst")
	}
	return nil
}
