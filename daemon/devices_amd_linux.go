package daemon

import (
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func setAMDGPUs(s *specs.Spec, dev *deviceInstance) error {
	req := dev.req
	if req.Count != 0 && len(req.DeviceIDs) > 0 {
		return errConflictCountDeviceIDs
	}

	switch {
	case len(req.DeviceIDs) > 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES="+strings.Join(req.DeviceIDs, ","))
	case req.Count > 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES="+strings.Join(countToDevices(req.Count), ","))
	case req.Count < 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES=all")
	case req.Count == 0:
		s.Process.Env = append(s.Process.Env, "AMD_VISIBLE_DEVICES=void")
	}

	return nil
}
