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

package continuity

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// TODO(stevvooe): This needs a lot of work before we can call it useful.

type groupIndex struct {
	byName map[string]*group
	byGID  map[int]*group
}

func getGroupIndex() (*groupIndex, error) {
	f, err := os.Open("/etc/group")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	groups, err := parseGroups(f)
	if err != nil {
		return nil, err
	}

	return newGroupIndex(groups), nil
}

func newGroupIndex(groups []group) *groupIndex {
	gi := &groupIndex{
		byName: make(map[string]*group),
		byGID:  make(map[int]*group),
	}

	for i, group := range groups {
		gi.byGID[group.gid] = &groups[i]
		gi.byName[group.name] = &groups[i]
	}

	return gi
}

type group struct {
	name    string
	gid     int
	members []string
}

func getGroupName(gid int) (string, error) {
	f, err := os.Open("/etc/group")
	if err != nil {
		return "", err
	}
	defer f.Close()

	groups, err := parseGroups(f)
	if err != nil {
		return "", err
	}

	for _, group := range groups {
		if group.gid == gid {
			return group.name, nil
		}
	}

	return "", fmt.Errorf("no group for gid")
}

// parseGroups parses an /etc/group file for group names, ids and membership.
// This is unix specific.
func parseGroups(rd io.Reader) ([]group, error) {
	var groups []group
	scanner := bufio.NewScanner(rd)

	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "#") {
			continue // skip comment
		}

		parts := strings.SplitN(scanner.Text(), ":", 4)

		if len(parts) != 4 {
			return nil, fmt.Errorf("bad entry: %q", scanner.Text())
		}

		name, _, sgid, smembers := parts[0], parts[1], parts[2], parts[3]

		gid, err := strconv.Atoi(sgid)
		if err != nil {
			return nil, fmt.Errorf("bad gid: %q", gid)
		}

		members := strings.Split(smembers, ",")

		groups = append(groups, group{
			name:    name,
			gid:     gid,
			members: members,
		})
	}

	if scanner.Err() != nil {
		return nil, scanner.Err()
	}

	return groups, nil
}
