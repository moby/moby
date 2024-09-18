package epoch

import (
	"strconv"
	"time"

	"github.com/moby/buildkit/exporter"
	commonexptypes "github.com/moby/buildkit/exporter/exptypes"
	"github.com/pkg/errors"
)

const (
	frontendSourceDateEpochArg = "build-arg:SOURCE_DATE_EPOCH"
)

func ParseBuildArgs(opt map[string]string) (string, bool) {
	v, ok := opt[frontendSourceDateEpochArg]
	return v, ok
}

func ParseExporterAttrs(opt map[string]string) (*time.Time, map[string]string, error) {
	rest := make(map[string]string, len(opt))

	var tm *time.Time

	for k, v := range opt {
		switch k {
		case string(commonexptypes.OptKeySourceDateEpoch):
			var err error
			tm, err = parseTime(k, v)
			if err != nil {
				return nil, nil, err
			}
		default:
			rest[k] = v
		}
	}

	return tm, rest, nil
}

func ParseSource(inp *exporter.Source) (*time.Time, bool, error) {
	if v, ok := inp.Metadata[commonexptypes.ExporterEpochKey]; ok {
		epoch, err := parseTime("", string(v))
		if err != nil {
			return nil, false, errors.Wrapf(err, "invalid SOURCE_DATE_EPOCH from frontend: %q", v)
		}
		return epoch, true, nil
	}
	return nil, false, nil
}

func parseTime(key, value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	sde, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid %s: %s", key, err)
	}
	tm := time.Unix(sde, 0).UTC()
	return &tm, nil
}
