package usergroup

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/idtools"
)

type subIDRange struct {
	Start  int
	Length int
}

type subIDRanges []subIDRange

func (e subIDRanges) Len() int           { return len(e) }
func (e subIDRanges) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e subIDRanges) Less(i, j int) bool { return e[i].Start < e[j].Start }

const (
	subuidFileName = "/etc/subuid"
	subgidFileName = "/etc/subgid"
)

func createIDMap(subidRanges subIDRanges) []idtools.IDMap {
	idMap := []idtools.IDMap{}

	containerID := 0
	for _, idrange := range subidRanges {
		idMap = append(idMap, idtools.IDMap{
			ContainerID: containerID,
			HostID:      idrange.Start,
			Size:        idrange.Length,
		})
		containerID = containerID + idrange.Length
	}
	return idMap
}

func parseSubuid(username string) (subIDRanges, error) {
	return parseSubidFile(subuidFileName, username)
}

func parseSubgid(username string) (subIDRanges, error) {
	return parseSubidFile(subgidFileName, username)
}

// parseSubidFile will read the appropriate file (/etc/subuid or /etc/subgid)
// and return all found subIDRanges for a specified username. If the special value
// "ALL" is supplied for username, then all subIDRanges in the file will be returned
func parseSubidFile(path, username string) (subIDRanges, error) {
	var rangeList subIDRanges

	subidFile, err := os.Open(path)
	if err != nil {
		return rangeList, err
	}
	defer subidFile.Close()

	s := bufio.NewScanner(subidFile)
	for s.Scan() {
		text := strings.TrimSpace(s.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		parts := strings.Split(text, ":")
		if len(parts) != 3 {
			return rangeList, fmt.Errorf("Cannot parse subuid/gid information: Format not correct for %s file", path)
		}
		if parts[0] == username || username == "ALL" {
			startid, err := strconv.ParseUint(parts[1], 10, 32)
			if err != nil {
				return rangeList, fmt.Errorf("String to int conversion failed during subuid/gid parsing of %s: %v", path, err)
			}
			length, err := strconv.ParseUint(parts[2], 10, 32)
			if err != nil {
				return rangeList, fmt.Errorf("String to int conversion failed during subuid/gid parsing of %s: %v", path, err)
			}
			rangeList = append(rangeList, subIDRange{int(startid), int(length)})
		}
	}

	return rangeList, s.Err()
}
