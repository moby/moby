package fluentd // import "github.com/docker/docker/daemon/logger/fluentd"
import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateLogOptReconnectInterval(t *testing.T) {
	invalidIntervals := []string{"-1", "1", "-1s", "99ms", "11s"}
	for _, v := range invalidIntervals {
		t.Run("invalid "+v, func(t *testing.T) {
			err := ValidateLogOpt(map[string]string{asyncReconnectIntervalKey: v})
			assert.ErrorContains(t, err, "invalid value for fluentd-async-reconnect-interval:")
		})
	}

	validIntervals := []string{"100ms", "10s"}
	for _, v := range validIntervals {
		t.Run("valid "+v, func(t *testing.T) {
			err := ValidateLogOpt(map[string]string{asyncReconnectIntervalKey: v})
			assert.NilError(t, err)
		})
	}
}
