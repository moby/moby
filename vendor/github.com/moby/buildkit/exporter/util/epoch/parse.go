package epoch

import (
	"fmt"
	"strconv"
	"time"

	"github.com/moby/buildkit/exporter"
	imageexptypes "github.com/moby/buildkit/exporter/containerimage/exptypes"
	commonexptypes "github.com/moby/buildkit/exporter/exptypes"
	"github.com/pkg/errors"
)

const (
	frontendSourceDateEpochArg = "build-arg:SOURCE_DATE_EPOCH"
)

type Epoch struct {
	Value *time.Time
}

func ParseBuildArgs(opt map[string]string) (string, bool) {
	v, ok := opt[frontendSourceDateEpochArg]
	return v, ok
}

func ParseExporterAttrs(opt map[string]string) (*Epoch, map[string]string, error) {
	rest := make(map[string]string, len(opt))

	var tm *Epoch

	for k, v := range opt {
		switch k {
		case string(commonexptypes.OptKeySourceDateEpoch):
			var err error
			value, err := parseTime(k, v)
			if err != nil {
				return nil, nil, err
			}
			tm = &Epoch{Value: value}
		default:
			rest[k] = v
		}
	}

	return tm, rest, nil
}

func ParseSource(inp *exporter.Source, p *imageexptypes.Platform) (*time.Time, error) {
	key := commonexptypes.ExporterEpochKey
	if p != nil {
		if v, ok := inp.Metadata[fmt.Sprintf("%s/%s", key, p.ID)]; ok {
			epoch, err := parseTime("", string(v))
			if err != nil {
				return nil, errors.Wrapf(err, "invalid SOURCE_DATE_EPOCH from frontend: %q", v)
			}
			return epoch, nil
		}
	}
	if v, ok := inp.Metadata[key]; ok {
		epoch, err := parseTime("", string(v))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid SOURCE_DATE_EPOCH from frontend: %q", v)
		}
		return epoch, nil
	}
	return nil, nil
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
