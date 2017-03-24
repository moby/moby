package service

import (
	"reflect"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/testutil/assert"
)

func TestMemBytesString(t *testing.T) {
	var mem opts.MemBytes = 1048576
	assert.Equal(t, mem.String(), "1MiB")
}

func TestMemBytesSetAndValue(t *testing.T) {
	var mem opts.MemBytes
	assert.NilError(t, mem.Set("5kb"))
	assert.Equal(t, mem.Value(), int64(5120))
}

func TestNanoCPUsString(t *testing.T) {
	var cpus opts.NanoCPUs = 6100000000
	assert.Equal(t, cpus.String(), "6.100")
}

func TestNanoCPUsSetAndValue(t *testing.T) {
	var cpus opts.NanoCPUs
	assert.NilError(t, cpus.Set("0.35"))
	assert.Equal(t, cpus.Value(), int64(350000000))
}

func TestDurationOptString(t *testing.T) {
	dur := time.Duration(300 * 10e8)
	duration := DurationOpt{value: &dur}
	assert.Equal(t, duration.String(), "5m0s")
}

func TestDurationOptSetAndValue(t *testing.T) {
	var duration DurationOpt
	assert.NilError(t, duration.Set("300s"))
	assert.Equal(t, *duration.Value(), time.Duration(300*10e8))
	assert.NilError(t, duration.Set("-300s"))
	assert.Equal(t, *duration.Value(), time.Duration(-300*10e8))
}

func TestPositiveDurationOptSetAndValue(t *testing.T) {
	var duration PositiveDurationOpt
	assert.NilError(t, duration.Set("300s"))
	assert.Equal(t, *duration.Value(), time.Duration(300*10e8))
	assert.Error(t, duration.Set("-300s"), "cannot be negative")
}

func TestUint64OptString(t *testing.T) {
	value := uint64(2345678)
	opt := Uint64Opt{value: &value}
	assert.Equal(t, opt.String(), "2345678")

	opt = Uint64Opt{}
	assert.Equal(t, opt.String(), "")
}

func TestUint64OptSetAndValue(t *testing.T) {
	var opt Uint64Opt
	assert.NilError(t, opt.Set("14445"))
	assert.Equal(t, *opt.Value(), uint64(14445))
}

func TestHealthCheckOptionsToHealthConfig(t *testing.T) {
	dur := time.Second
	opt := healthCheckOptions{
		cmd:      "curl",
		interval: PositiveDurationOpt{DurationOpt{value: &dur}},
		timeout:  PositiveDurationOpt{DurationOpt{value: &dur}},
		retries:  10,
	}
	config, err := opt.toHealthConfig()
	assert.NilError(t, err)
	assert.Equal(t, reflect.DeepEqual(config, &container.HealthConfig{
		Test:     []string{"CMD-SHELL", "curl"},
		Interval: time.Second,
		Timeout:  time.Second,
		Retries:  10,
	}), true)
}

func TestHealthCheckOptionsToHealthConfigNoHealthcheck(t *testing.T) {
	opt := healthCheckOptions{
		noHealthcheck: true,
	}
	config, err := opt.toHealthConfig()
	assert.NilError(t, err)
	assert.Equal(t, reflect.DeepEqual(config, &container.HealthConfig{
		Test: []string{"NONE"},
	}), true)
}

func TestHealthCheckOptionsToHealthConfigConflict(t *testing.T) {
	opt := healthCheckOptions{
		cmd:           "curl",
		noHealthcheck: true,
	}
	_, err := opt.toHealthConfig()
	assert.Error(t, err, "--no-healthcheck conflicts with --health-* options")
}
