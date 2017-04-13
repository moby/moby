package service

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/opts"
	"github.com/stretchr/testify/assert"
)

func TestMemBytesString(t *testing.T) {
	var mem opts.MemBytes = 1048576
	assert.Equal(t, "1MiB", mem.String())
}

func TestMemBytesSetAndValue(t *testing.T) {
	var mem opts.MemBytes
	assert.NoError(t, mem.Set("5kb"))
	assert.Equal(t, int64(5120), mem.Value())
}

func TestNanoCPUsString(t *testing.T) {
	var cpus opts.NanoCPUs = 6100000000
	assert.Equal(t, "6.100", cpus.String())
}

func TestNanoCPUsSetAndValue(t *testing.T) {
	var cpus opts.NanoCPUs
	assert.NoError(t, cpus.Set("0.35"))
	assert.Equal(t, int64(350000000), cpus.Value())
}

func TestDurationOptString(t *testing.T) {
	dur := time.Duration(300 * 10e8)
	duration := DurationOpt{value: &dur}
	assert.Equal(t, "5m0s", duration.String())
}

func TestDurationOptSetAndValue(t *testing.T) {
	var duration DurationOpt
	assert.NoError(t, duration.Set("300s"))
	assert.Equal(t, time.Duration(300*10e8), *duration.Value())
	assert.NoError(t, duration.Set("-300s"))
	assert.Equal(t, time.Duration(-300*10e8), *duration.Value())
}

func TestPositiveDurationOptSetAndValue(t *testing.T) {
	var duration PositiveDurationOpt
	assert.NoError(t, duration.Set("300s"))
	assert.Equal(t, time.Duration(300*10e8), *duration.Value())
	assert.EqualError(t, duration.Set("-300s"), "duration cannot be negative")
}

func TestUint64OptString(t *testing.T) {
	value := uint64(2345678)
	opt := Uint64Opt{value: &value}
	assert.Equal(t, "2345678", opt.String())

	opt = Uint64Opt{}
	assert.Equal(t, "", opt.String())
}

func TestUint64OptSetAndValue(t *testing.T) {
	var opt Uint64Opt
	assert.NoError(t, opt.Set("14445"))
	assert.Equal(t, uint64(14445), *opt.Value())
}

func TestHealthCheckOptionsToHealthConfig(t *testing.T) {
	dur := time.Second
	opt := healthCheckOptions{
		cmd:         "curl",
		interval:    PositiveDurationOpt{DurationOpt{value: &dur}},
		timeout:     PositiveDurationOpt{DurationOpt{value: &dur}},
		startPeriod: PositiveDurationOpt{DurationOpt{value: &dur}},
		retries:     10,
	}
	config, err := opt.toHealthConfig()
	assert.NoError(t, err)
	assert.Equal(t, &container.HealthConfig{
		Test:        []string{"CMD-SHELL", "curl"},
		Interval:    time.Second,
		Timeout:     time.Second,
		StartPeriod: time.Second,
		Retries:     10,
	}, config)
}

func TestHealthCheckOptionsToHealthConfigNoHealthcheck(t *testing.T) {
	opt := healthCheckOptions{
		noHealthcheck: true,
	}
	config, err := opt.toHealthConfig()
	assert.NoError(t, err)
	assert.Equal(t, &container.HealthConfig{
		Test: []string{"NONE"},
	}, config)
}

func TestHealthCheckOptionsToHealthConfigConflict(t *testing.T) {
	opt := healthCheckOptions{
		cmd:           "curl",
		noHealthcheck: true,
	}
	_, err := opt.toHealthConfig()
	assert.EqualError(t, err, "--no-healthcheck conflicts with --health-* options")
}
