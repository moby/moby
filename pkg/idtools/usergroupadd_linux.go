package idtools // import "github.com/docker/docker/pkg/idtools"

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// add a user and/or group to Linux /etc/passwd, /etc/group using standard
// Linux distribution commands:
// adduser --system --shell /bin/false --disabled-login --disabled-password --no-create-home --group <username>
// useradd -r -s /bin/false <username>

var (
	once        sync.Once
	userCommand string
	idOutRegexp = regexp.MustCompile(`uid=([0-9]+).*gid=([0-9]+)`)
)

const (
	// default length for a UID/GID subordinate range
	defaultRangeLen   = 65536
	defaultRangeStart = 100000
)

// AddNamespaceRangesUser takes a username and uses the standard system
// utility to create a system user/group pair used to hold the
// /etc/sub{uid,gid} ranges which will be used for user namespace
// mapping ranges in containers.
func AddNamespaceRangesUser(name string) (int, int, error) {
	if err := addUser(name); err != nil {
		return -1, -1, fmt.Errorf("error adding user %q: %v", name, err)
	}

	// Query the system for the created uid and gid pair
	out, err := exec.Command("id", name).CombinedOutput()
	if err != nil {
		return -1, -1, fmt.Errorf("error trying to find uid/gid for new user %q: %v", name, err)
	}
	matches := idOutRegexp.FindStringSubmatch(strings.TrimSpace(string(out)))
	if len(matches) != 3 {
		return -1, -1, fmt.Errorf("can't find uid, gid from `id` output: %q", string(out))
	}
	uid, err := strconv.Atoi(matches[1])
	if err != nil {
		return -1, -1, fmt.Errorf("can't convert found uid (%s) to int: %v", matches[1], err)
	}
	gid, err := strconv.Atoi(matches[2])
	if err != nil {
		return -1, -1, fmt.Errorf("Can't convert found gid (%s) to int: %v", matches[2], err)
	}

	// Now we need to create the subuid/subgid ranges for our new user/group (system users
	// do not get auto-created ranges in subuid/subgid)

	if err := createSubordinateRanges(name); err != nil {
		return -1, -1, fmt.Errorf("couldn't create subordinate ID ranges: %v", err)
	}
	return uid, gid, nil
}

func addUser(name string) error {
	once.Do(func() {
		// set up which commands are used for adding users/groups dependent on distro
		if _, err := resolveBinary("adduser"); err == nil {
			userCommand = "adduser"
		} else if _, err := resolveBinary("useradd"); err == nil {
			userCommand = "useradd"
		}
	})
	var args []string
	switch userCommand {
	case "adduser":
		args = []string{"--system", "--shell", "/bin/false", "--no-create-home", "--disabled-login", "--disabled-password", "--group", name}
	case "useradd":
		args = []string{"-r", "-s", "/bin/false", name}
	default:
		return fmt.Errorf("cannot add user; no useradd/adduser binary found")
	}

	if out, err := exec.Command(userCommand, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add user with error: %v; output: %q", err, string(out))
	}
	return nil
}

func createSubordinateRanges(name string) error {
	// first, we should verify that ranges weren't automatically created
	// by the distro tooling
	ranges, err := parseSubuid(name)
	if err != nil {
		return fmt.Errorf("error while looking for subuid ranges for user %q: %v", name, err)
	}
	if len(ranges) == 0 {
		// no UID ranges; let's create one
		startID, err := findNextUIDRange()
		if err != nil {
			return fmt.Errorf("can't find available subuid range: %v", err)
		}
		idRange := fmt.Sprintf("%d-%d", startID, startID+defaultRangeLen-1)
		out, err := exec.Command("usermod", "-v", idRange, name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("unable to add subuid range to user: %q; output: %s, err: %v", name, out, err)
		}
	}

	ranges, err = parseSubgid(name)
	if err != nil {
		return fmt.Errorf("error while looking for subgid ranges for user %q: %v", name, err)
	}
	if len(ranges) == 0 {
		// no GID ranges; let's create one
		startID, err := findNextGIDRange()
		if err != nil {
			return fmt.Errorf("can't find available subgid range: %v", err)
		}
		idRange := fmt.Sprintf("%d-%d", startID, startID+defaultRangeLen-1)
		out, err := exec.Command("usermod", "-w", idRange, name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("unable to add subgid range to user: %q; output: %s, err: %v", name, out, err)
		}
	}
	return nil
}

func findNextUIDRange() (int, error) {
	ranges, err := parseSubuid("ALL")
	if err != nil {
		return -1, fmt.Errorf("couldn't parse all ranges in /etc/subuid file: %v", err)
	}
	sort.Sort(ranges)
	return findNextRangeStart(ranges)
}

func findNextGIDRange() (int, error) {
	ranges, err := parseSubgid("ALL")
	if err != nil {
		return -1, fmt.Errorf("couldn't parse all ranges in /etc/subgid file: %v", err)
	}
	sort.Sort(ranges)
	return findNextRangeStart(ranges)
}

func findNextRangeStart(rangeList ranges) (int, error) {
	startID := defaultRangeStart
	for _, arange := range rangeList {
		if wouldOverlap(arange, startID) {
			startID = arange.Start + arange.Length
		}
	}
	return startID, nil
}

func wouldOverlap(arange subIDRange, ID int) bool {
	low := ID
	high := ID + defaultRangeLen
	if (low >= arange.Start && low <= arange.Start+arange.Length) ||
		(high <= arange.Start+arange.Length && high >= arange.Start) {
		return true
	}
	return false
}
