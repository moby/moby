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

package manager

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/containerd/containerd/v2/core/mount"
)

const formatCheck = "{{"

type mountFormatter struct{}

func (mountFormatter) Transform(_ context.Context, m mount.Mount, a []mount.ActiveMount) (mount.Mount, error) {
	if sc := formatString(m.Source); sc != nil {
		f, err := sc(a)
		if err != nil {
			return m, err
		}
		m.Source = f
	}

	if tc := formatString(m.Target); tc != nil {
		f, err := tc(a)
		if err != nil {
			return m, err
		}
		m.Target = f
	}

	var o []string
	for i := range m.Options {
		if oc := formatString(m.Options[i]); oc != nil {
			f, err := oc(a)
			if err != nil {
				return m, err
			}
			if o == nil {
				o = make([]string, len(m.Options))
				copy(o, m.Options)
			}
			o[i] = f
		}
	}
	if o != nil {
		m.Options = o
	}
	return m, nil
}

func formatString(s string) func([]mount.ActiveMount) (string, error) {
	if !strings.Contains(s, formatCheck) {
		return nil
	}

	return func(a []mount.ActiveMount) (string, error) {
		// TODO: The formatting is very easy, don't use template
		fm := template.FuncMap{
			"source": func(i int) (string, error) {
				if i < 0 || i >= len(a) {
					return "", fmt.Errorf("index out of bounds: %d, has %d active mounts", i, len(a))
				}
				return a[i].Source, nil
			},
			"target": func(i int) (string, error) {
				if i < 0 || i >= len(a) {
					return "", fmt.Errorf("index out of bounds: %d, has %d active mounts", i, len(a))
				}
				return a[i].Target, nil
			},
			"mount": func(i int) (string, error) {
				if i < 0 || i >= len(a) {
					return "", fmt.Errorf("index out of bounds: %d, has %d active mounts", i, len(a))
				}
				return a[i].MountPoint, nil
			},
			"overlay": func(start, end int) (string, error) {
				var dirs []string
				if start > end {
					if start >= len(a) || end < 0 {
						return "", fmt.Errorf("invalid range: %d-%d, has %d active mounts", start, end, len(a))
					}
					for i := start; i >= end; i-- {
						dirs = append(dirs, a[i].MountPoint)
					}
				} else {
					if start < 0 || end >= len(a) {
						return "", fmt.Errorf("invalid range: %d-%d, has %d active mounts", start, end, len(a))
					}
					for i := start; i <= end; i++ {
						dirs = append(dirs, a[i].MountPoint)
					}
				}
				return strings.Join(dirs, ":"), nil
			},
		}
		t, err := template.New("").Funcs(fm).Parse(s)
		if err != nil {
			return "", err
		}

		buf := bytes.NewBuffer(nil)
		if err := t.Execute(buf, nil); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
}
