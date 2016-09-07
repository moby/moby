package daemon

import (
	containertypes "github.com/docker/engine-api/types/container"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestMergeAndVerifyLogConfigNilConfig(c *check.C) {
	d := &Daemon{defaultLogConfig: containertypes.LogConfig{Type: "json-file", Config: map[string]string{"max-file": "1"}}}
	cfg := containertypes.LogConfig{Type: d.defaultLogConfig.Type}
	if err := d.mergeAndVerifyLogConfig(&cfg); err != nil {
		c.Fatal(err)
	}
}
