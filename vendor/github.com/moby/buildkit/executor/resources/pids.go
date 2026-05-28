package resources

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	resourcestypes "github.com/moby/buildkit/executor/resources/types"
	"github.com/pkg/errors"
)

const (
	pidsCurrentFile = "pids.current"
)

func getCgroupPIDsStat(path string) (*resourcestypes.PIDsStat, error) {
	pidsStat := &resourcestypes.PIDsStat{}

	v, err := parseSingleValueFile(filepath.Join(path, pidsCurrentFile))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		pidsStat.Current = &v
	}

	return pidsStat, nil
}

func parseSingleValueFile(filePath string) (uint64, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read %s", filePath)
	}

	valueStr := strings.TrimSpace(string(content))
	value, err := strconv.ParseUint(valueStr, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse value: %s", valueStr)
	}

	return value, nil
}
