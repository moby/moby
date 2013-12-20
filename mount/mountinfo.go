package mount

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const (
	// We only parse upto the mountinfo because that is all we
	// care about right now
	mountinfoFormat = "%d %d %d:%d %s %s %s"
)

// Represents one line from /proc/self/mountinfo
type procEntry struct {
	id, parent, major, minor int
	source, mountpoint, opts string
}

// Parse /proc/self/mountinfo because comparing Dev and ino does not work from bind mounts
func parseMountTable() ([]*procEntry, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseInfoFile(f)
}

func parseInfoFile(r io.Reader) ([]*procEntry, error) {
	var (
		s   = bufio.NewScanner(r)
		out = []*procEntry{}
	)

	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		var (
			p    = &procEntry{}
			text = s.Text()
		)
		if _, err := fmt.Sscanf(text, mountinfoFormat,
			&p.id, &p.parent, &p.major, &p.minor,
			&p.source, &p.mountpoint, &p.opts); err != nil {
			return nil, fmt.Errorf("Scanning '%s' failed: %s", text, err)
		}
		out = append(out, p)
	}
	return out, nil
}
