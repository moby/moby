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

package warning

import (
	"context"
	"sync"
	"time"

	"github.com/containerd/log"

	deprecation "github.com/containerd/containerd/pkg/deprecation"
	"github.com/containerd/containerd/plugin"
)

type Service interface {
	Emit(context.Context, deprecation.Warning)
	Warnings() []Warning
}

func init() {
	plugin.Register(&plugin.Registration{
		Type: plugin.WarningPlugin,
		ID:   plugin.DeprecationsPlugin,
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			return &service{warnings: make(map[deprecation.Warning]time.Time)}, nil
		},
	})
}

type Warning struct {
	ID             deprecation.Warning
	LastOccurrence time.Time
	Message        string
}

var _ Service = (*service)(nil)

type service struct {
	warnings map[deprecation.Warning]time.Time
	m        sync.RWMutex
}

func (s *service) Emit(ctx context.Context, warning deprecation.Warning) {
	if !deprecation.Valid(warning) {
		log.G(ctx).WithField("warningID", string(warning)).Warn("invalid deprecation warning")
		return
	}
	s.m.Lock()
	defer s.m.Unlock()
	s.warnings[warning] = time.Now()
}
func (s *service) Warnings() []Warning {
	s.m.RLock()
	defer s.m.RUnlock()
	var warnings []Warning
	for k, v := range s.warnings {
		msg, ok := deprecation.Message(k)
		if !ok {
			continue
		}
		warnings = append(warnings, Warning{
			ID:             k,
			LastOccurrence: v,
			Message:        msg,
		})
	}
	return warnings
}
