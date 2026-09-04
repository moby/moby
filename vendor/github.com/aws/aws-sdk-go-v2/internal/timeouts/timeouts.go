package timeouts

import (
	"os"
	"sync"
	"time"
)

// DefaultReadTimeout is the SDK's default read timeout for a service with no
// entry in serviceInactivityTimeoutMillis.
const DefaultReadTimeout = 5 * time.Minute

const enableReadTimeoutEnvVar = "AWS_ENABLE_DEFAULT_SOCKET_TIMEOUT_2026"

var enableFromEnv = sync.OnceValue(func() bool {
	return os.Getenv(enableReadTimeoutEnvVar) == "true"
})

// GetServiceReadTimeout reports the SDK's default read timeout for a service,
// and whether one applies.
func GetServiceReadTimeout(serviceID string) (time.Duration, bool) {
	if enableReadTimeout2026 {
		if !readTimeout2026Rollout[serviceID] {
			return 0, false
		}
	} else if !enableFromEnv() {
		return 0, false
	}

	ms, ok := serviceInactivityTimeoutMillis[serviceID]
	if !ok {
		return DefaultReadTimeout, true
	}
	if ms < 0 {
		return 0, false
	}

	return time.Duration(ms) * time.Millisecond, true
}
