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

package logtest

import (
	"bytes"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
)

type testHook struct {
	t   testing.TB
	fmt logrus.Formatter
	mu  sync.Mutex
}

func (*testHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *testHook) Fire(e *logrus.Entry) error {
	s, err := h.fmt.Format(e)
	if err != nil {
		return err
	}

	// Because the logger could be called from multiple goroutines,
	// but t.Log() is not designed for.
	h.mu.Lock()
	defer h.mu.Unlock()
	h.t.Log(string(bytes.TrimRight(s, "\n")))

	return nil
}
