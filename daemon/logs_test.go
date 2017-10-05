package daemon

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

func TestMergeAndVerifyLogConfigNilConfig(t *testing.T) {
	d := &Daemon{defaultLogConfig: containertypes.LogConfig{Type: "json-file", Config: map[string]string{"max-file": "1"}}}
	cfg := containertypes.LogConfig{Type: d.defaultLogConfig.Type}
	if err := d.mergeAndVerifyLogConfig(&cfg); err != nil {
		t.Fatal(err)
	}
}

func TestMergeAndVerifyLogConfig(t *testing.T) {
	timezone := "Asia/Jerusalem"
	logType := "json-file"
	d := &Daemon{defaultLogConfig: containertypes.LogConfig{Type: logType, Timezone: timezone}}
	cfg := containertypes.LogConfig{}
	if err := d.mergeAndVerifyLogConfig(&cfg); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, timezone, cfg.Timezone)
	assert.Equal(t, logType, cfg.Type)
}
