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

/*
   This file is copied and customized based on
   https://github.com/moby/moby/blob/master/pkg/idtools/idtools.go
*/

package userns

import (
	"errors"
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
)

const invalidID = 1<<32 - 1

var invalidUser = User{Uid: invalidID, Gid: invalidID}

// User is a Uid and Gid pair of a user
//
//nolint:revive
type User struct {
	Uid uint32
	Gid uint32
}

// IDMap contains the mappings of Uids and Gids.
//
//nolint:revive
type IDMap struct {
	UidMap []specs.LinuxIDMapping `json:"UidMap"`
	GidMap []specs.LinuxIDMapping `json:"GidMap"`
}

// ToHost returns the host user ID pair for the container ID pair.
func (i IDMap) ToHost(pair User) (User, error) {
	var (
		target User
		err    error
	)
	target.Uid, err = toHost(pair.Uid, i.UidMap)
	if err != nil {
		return invalidUser, err
	}
	target.Gid, err = toHost(pair.Gid, i.GidMap)
	if err != nil {
		return invalidUser, err
	}
	return target, nil
}

// toHost takes an id mapping and a remapped ID, and translates the
// ID to the mapped host ID. If no map is provided, then the translation
// assumes a 1-to-1 mapping and returns the passed in id #
func toHost(contID uint32, idMap []specs.LinuxIDMapping) (uint32, error) {
	if idMap == nil {
		return contID, nil
	}
	for _, m := range idMap {
		high, err := safeSum(m.ContainerID, m.Size)
		if err != nil {
			break
		}
		if contID >= m.ContainerID && contID < high {
			hostID, err := safeSum(m.HostID, contID-m.ContainerID)
			if err != nil || hostID == invalidID {
				break
			}
			return hostID, nil
		}
	}
	return invalidID, fmt.Errorf("container ID %d cannot be mapped to a host ID", contID)
}

// safeSum returns the sum of x and y. or an error if the result overflows
func safeSum(x, y uint32) (uint32, error) {
	z := x + y
	if z < x || z < y {
		return invalidID, errors.New("ID overflow")
	}
	return z, nil
}
