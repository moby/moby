package mount

import (
	"bufio"
	"fmt"
	"os"
)

const (
	mountinfoFormat = "%d %d %d:%d %s %s %s - %s %s %s"
)

// Represents one line from /proc/self/mountinfo
type procEntry struct {
	id, parent, major, minor           int
	source, mountpoint, fstype, device string
	vfsopts, opts                      string
}

// Parse /proc/self/mountinfo because comparing Dev and ino does not work from bind mounts
func parseMountTable() ([]*procEntry, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	out := []*procEntry{}
	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		p := &procEntry{}
		if _, err := fmt.Sscanf(s.Text(), mountinfoFormat,
			&p.id, &p.parent, &p.major, &p.minor,
			&p.source, &p.mountpoint, &p.vfsopts, &p.fstype,
			&p.device, &p.opts); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
