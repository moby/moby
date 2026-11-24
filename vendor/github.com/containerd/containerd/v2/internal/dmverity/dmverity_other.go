//go:build !linux

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

package dmverity

import (
	"context"
	"fmt"
)

var errUnsupported = fmt.Errorf("dmverity is only supported on Linux systems")

func IsSupported() (bool, error) {
	return false, errUnsupported
}

func Format(_ string, _ string, _ *DmverityOptions) (string, error) {
	return "", errUnsupported
}

func FormatLayer(_ context.Context, _ string, _ *DmverityOptions) error {
	return errUnsupported
}

func Open(_ string, _ string, _ string, _ string, _ uint64, _ *DmverityOptions) (string, error) {
	return "", errUnsupported
}

func Close(_ string) error {
	return errUnsupported
}

func VerifyDevice(_ string, _ string) error {
	return errUnsupported
}
