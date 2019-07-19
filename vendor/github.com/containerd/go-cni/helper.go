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

package cni

import (
	"fmt"

	"github.com/containernetworking/cni/pkg/types/current"
)

func validateInterfaceConfig(ipConf *current.IPConfig, ifs int) error {
	if ipConf == nil {
		return fmt.Errorf("invalid IP configuration")
	}
	if ipConf.Interface != nil && *ipConf.Interface > ifs {
		return fmt.Errorf("invalid IP configuration with invalid interface %d", *ipConf.Interface)
	}
	return nil
}

func getIfName(prefix string, i int) string {
	return fmt.Sprintf("%s%d", prefix, i)
}

func defaultInterface(prefix string) string {
	return getIfName(prefix, 0)
}
