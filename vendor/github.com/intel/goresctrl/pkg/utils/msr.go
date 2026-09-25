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

package utils

import (
	"encoding/binary"
	"fmt"
	"os"

	goresctrlpath "github.com/intel/goresctrl/pkg/path"
)

func ReadMSR(cpu ID, msr int64) (uint64, error) {
	path := goresctrlpath.Path("dev/cpu", fmt.Sprintf("%d", cpu), "msr")
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}

	defer file.Close() // nolint:errcheck

	// Whence is the point of reference for offset
	// 0 = Beginning of file
	// 1 = Current position
	// 2 = End of file
	whence := int(0)
	_, err = file.Seek(msr, whence)
	if err != nil {
		return 0, err
	}

	data := make([]byte, 8)

	_, err = file.Read(data)
	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint64(data), nil
}
