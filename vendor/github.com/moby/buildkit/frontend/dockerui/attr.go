package dockerui

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/platforms"
	"github.com/docker/go-units"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/tonistiigi/go-csvvalue"
)

func parsePlatforms(v string) ([]ocispecs.Platform, error) {
	var pp []ocispecs.Platform
	for _, v := range strings.Split(v, ",") {
		p, err := platforms.Parse(v)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse target platform %s", v)
		}
		pp = append(pp, platforms.Normalize(p))
	}
	return pp, nil
}

func parseResolveMode(v string) (llb.ResolveMode, error) {
	switch v {
	case pb.AttrImageResolveModeDefault, "":
		return llb.ResolveModeDefault, nil
	case pb.AttrImageResolveModeForcePull:
		return llb.ResolveModeForcePull, nil
	case pb.AttrImageResolveModePreferLocal:
		return llb.ResolveModePreferLocal, nil
	default:
		return 0, errors.Errorf("invalid image-resolve-mode: %s", v)
	}
}

func parseExtraHosts(v string) ([]llb.HostIP, error) {
	if v == "" {
		return nil, nil
	}
	out := make([]llb.HostIP, 0)
	fields, err := csvvalue.Fields(v, nil)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		key, val, ok := strings.Cut(strings.ToLower(field), "=")
		if !ok {
			return nil, errors.Errorf("invalid key-value pair %s", field)
		}
		ip := net.ParseIP(val)
		if ip == nil {
			return nil, errors.Errorf("failed to parse IP %s", val)
		}
		out = append(out, llb.HostIP{Host: key, IP: ip})
	}
	return out, nil
}

func parseShmSize(v string) (int64, error) {
	if len(v) == 0 {
		return 0, nil
	}
	kb, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, err
	}
	return kb, nil
}

func parseUlimits(v string) ([]*pb.Ulimit, error) {
	if v == "" {
		return nil, nil
	}
	out := make([]*pb.Ulimit, 0)
	fields, err := csvvalue.Fields(v, nil)
	if err != nil {
		return nil, err
	}
	for _, field := range fields {
		ulimit, err := units.ParseUlimit(field)
		if err != nil {
			return nil, err
		}
		out = append(out, &pb.Ulimit{
			Name: ulimit.Name,
			Soft: ulimit.Soft,
			Hard: ulimit.Hard,
		})
	}
	return out, nil
}

func parseNetMode(v string) (pb.NetMode, error) {
	if v == "" {
		return llb.NetModeSandbox, nil
	}
	switch v {
	case "none":
		return llb.NetModeNone, nil
	case "host":
		return llb.NetModeHost, nil
	case "sandbox":
		return llb.NetModeSandbox, nil
	default:
		return 0, errors.Errorf("invalid netmode %s", v)
	}
}

func parseSourceDateEpoch(v string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	sde, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid SOURCE_DATE_EPOCH: %s", v)
	}
	tm := time.Unix(sde, 0).UTC()
	return &tm, nil
}

func parseLocalSessionIDs(opt map[string]string) map[string]string {
	m := map[string]string{}
	for k, v := range opt {
		if strings.HasPrefix(k, localSessionIDPrefix) {
			m[strings.TrimPrefix(k, localSessionIDPrefix)] = v
		}
	}
	return m
}

func filter(opt map[string]string, key string) map[string]string {
	m := map[string]string{}
	for k, v := range opt {
		if strings.HasPrefix(k, key) {
			m[strings.TrimPrefix(k, key)] = v
		}
	}
	return m
}
