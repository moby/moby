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

package builtin

import (
	"context"

	"github.com/containerd/nri/pkg/adaptation/builtin"
	"github.com/containerd/nri/pkg/log"

	validator "github.com/containerd/nri/plugins/default-validator"
)

type (
	// DefaultValidatorConfig is an alias for DefaultValidatorConfig from main package.
	DefaultValidatorConfig = validator.DefaultValidatorConfig
)

// GetDefaultValidator returns a configured instance of the default validator.
// If default validation is disabled nil is returned.
func GetDefaultValidator(cfg *DefaultValidatorConfig) *builtin.BuiltinPlugin {
	if cfg == nil || !cfg.Enable {
		log.Infof(context.TODO(), "built-in NRI default validator is disabled")
		return nil
	}

	v := validator.NewDefaultValidator(cfg)
	return &builtin.BuiltinPlugin{
		Base:  "default-validator",
		Index: "00",
		Handlers: builtin.BuiltinHandlers{
			ValidateContainerAdjustment: v.ValidateContainerAdjustment,
		},
	}
}
