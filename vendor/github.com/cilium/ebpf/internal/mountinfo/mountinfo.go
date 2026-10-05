// Package mountinfo parses /proc/self/mountinfo to discover filesystem
// mounts visible in the calling process's mount namespace.
package mountinfo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/platform"
)

// Entry represents one parsed line from /proc/self/mountinfo.
type Entry struct {
	// MountPoint is the absolute path where the filesystem is mounted,
	// with octal escapes (\040, \011, \012, \134) decoded.
	MountPoint string

	// Root is the path within the source filesystem that's exposed at
	// MountPoint, octal-decoded. It is "/" for a full filesystem mount and
	// some other path (e.g. "/events") when a subdirectory of the source
	// filesystem has been bind-mounted onto MountPoint. Callers that want
	// to use MountPoint as the root of a filesystem should reject entries
	// where Root != "/".
	Root string

	// FSType is the filesystem type, e.g. "bpf", "tracefs", "debugfs".
	FSType string
}

// Read parses /proc/self/mountinfo and returns one Entry per mount line.
// The result is cached for the lifetime of the process; mounts that appear
// or disappear after the first call are not reflected.
//
// On non-Linux, returns an error wrapping [internal.ErrNotSupportedOnOS].
func Read() ([]Entry, error) {
	return readOnce()
}

// FindByFSType returns the mount points for filesystems of the given type,
// in order of appearance, with duplicates removed.
func FindByFSType(fstype string) ([]string, error) {
	entries, err := Read()
	if err != nil {
		return nil, err
	}
	return filterByFSType(entries, fstype), nil
}

func filterByFSType(entries []Entry, fstype string) []string {
	var mounts []string
	for _, e := range entries {
		if e.FSType != fstype {
			continue
		}
		// Number of matching mounts is expected to be very low; linear
		// search is faster and more cache-friendly than a map at this
		// scale, and avoids the allocations a map would incur.
		if !slices.Contains(mounts, e.MountPoint) {
			mounts = append(mounts, e.MountPoint)
		}
	}
	return mounts
}

// parseEntries parses mountinfo data read from r.
func parseEntries(r io.Reader) ([]Entry, error) {
	var entries []Entry
	// Format of /proc/self/mountinfo:
	//
	// {id} {parent id} {major:minor} {root} {mount point} {mount options} [optional fields...] - {filesystem type} {source} {superblock options}
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			line = strings.TrimRight(line, "\n")
			if line != "" {
				entry, perr := parseLine(line)
				if perr != nil {
					return nil, perr
				}
				entries = append(entries, entry)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read mountinfo: %w", err)
		}
	}

	return entries, nil
}

func parseLine(line string) (Entry, error) {
	firstHalfStr, secondHalfStr, ok := strings.Cut(line, " - ")
	if !ok {
		return Entry{}, fmt.Errorf("invalid mountinfo line, missing dash: %q", line)
	}

	secondHalf := strings.Fields(strings.TrimSpace(secondHalfStr))
	if len(secondHalf) == 0 {
		return Entry{}, fmt.Errorf("invalid mountinfo line, too few fields after dash: %q", line)
	}

	firstHalf := strings.Fields(strings.TrimSpace(firstHalfStr))
	if len(firstHalf) < 6 {
		return Entry{}, fmt.Errorf("invalid mountinfo line, too few fields: %q", line)
	}

	return Entry{
		Root:       unescape(firstHalf[3]),
		MountPoint: unescape(firstHalf[4]),
		FSType:     secondHalf[0],
	}, nil
}

// show_mountinfo in the kernel has the escape set of ' \t\n\\'. Instead of
// a full octal unescaper, only replace these specific characters.
var unescaper = strings.NewReplacer(
	`\040`, " ",
	`\011`, "\t",
	`\012`, "\n",
	`\134`, `\`,
)

func unescape(s string) string {
	return unescaper.Replace(s)
}

var readOnce = sync.OnceValues(func() ([]Entry, error) {
	if !platform.IsLinux {
		return nil, fmt.Errorf("mountinfo: %w", internal.ErrNotSupportedOnOS)
	}

	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, fmt.Errorf("open mountinfo: %w", err)
	}
	defer f.Close()

	return parseEntries(f)
})
