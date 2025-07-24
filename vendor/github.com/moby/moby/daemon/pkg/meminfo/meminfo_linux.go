package meminfo

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

// readMemInfo retrieves memory statistics of the host system and returns a
// Memory type.
func readMemInfo() (*Memory, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return parseMemInfo(file)
}

// parseMemInfo parses the /proc/meminfo file into
// a Memory object given an io.Reader to the file.
// Throws error if there are problems reading from the file
func parseMemInfo(reader io.Reader) (*Memory, error) {
	meminfo := &Memory{}
	scanner := bufio.NewScanner(reader)
	memAvailable := int64(-1)
	for scanner.Scan() {
		// Expected format: ["MemTotal:", "1234", "kB"]
		parts := strings.Fields(scanner.Text())

		// Sanity checks: Skip malformed entries.
		if len(parts) < 3 || parts[2] != "kB" {
			continue
		}

		// Convert to bytes.
		size, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		// Convert to KiB
		bytes := int64(size) * 1024

		switch parts[0] {
		case "MemTotal:":
			meminfo.MemTotal = bytes
		case "MemFree:":
			meminfo.MemFree = bytes
		case "MemAvailable:":
			memAvailable = bytes
		case "SwapTotal:":
			meminfo.SwapTotal = bytes
		case "SwapFree:":
			meminfo.SwapFree = bytes
		}
	}
	if memAvailable != -1 {
		meminfo.MemFree = memAvailable
	}

	// Handle errors that may have occurred during the reading of the file.
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return meminfo, nil
}
