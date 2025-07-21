package container

import (
	"fmt"
	"strings"
	"time"
)

// HealthStatus is a string representation of the container's health.
//
// It currently is an alias for string, but may become a distinct type in future.
type HealthStatus = string

// Health states
const (
	NoHealthcheck HealthStatus = "none"      // Indicates there is no healthcheck
	Starting      HealthStatus = "starting"  // Starting indicates that the container is not yet ready
	Healthy       HealthStatus = "healthy"   // Healthy indicates that the container is running correctly
	Unhealthy     HealthStatus = "unhealthy" // Unhealthy indicates that the container has a problem
)

// Health stores information about the container's healthcheck results
type Health struct {
	Status        HealthStatus         // Status is one of [Starting], [Healthy] or [Unhealthy].
	FailingStreak int                  // FailingStreak is the number of consecutive failures
	Log           []*HealthcheckResult // Log contains the last few results (oldest first)
}

// HealthcheckResult stores information about a single run of a healthcheck probe
type HealthcheckResult struct {
	Start    time.Time // Start is the time this check started
	End      time.Time // End is the time this check ended
	ExitCode int       // ExitCode meanings: 0=healthy, 1=unhealthy, 2=reserved (considered unhealthy), else=error running probe
	Output   string    // Output from last check
}

var validHealths = []string{
	NoHealthcheck, Starting, Healthy, Unhealthy,
}

// ValidateHealthStatus checks if the provided string is a valid
// container [HealthStatus].
func ValidateHealthStatus(s HealthStatus) error {
	switch s {
	case NoHealthcheck, Starting, Healthy, Unhealthy:
		return nil
	default:
		return errInvalidParameter{error: fmt.Errorf("invalid value for health (%s): must be one of %s", s, strings.Join(validHealths, ", "))}
	}
}
