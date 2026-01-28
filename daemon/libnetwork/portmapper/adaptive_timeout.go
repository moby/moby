package portmapper

import (
	"sync"
	"time"
)

// AdaptiveTimeout manages dynamic timeout calculation for proxy startup
// based on system load and historical startup times.
type AdaptiveTimeout struct {
	mu                sync.RWMutex
	startupTimes      []time.Duration
	maxSamples        int
	baseTimeout       time.Duration
	maxTimeout        time.Duration
	loadFactor        float64
	consecutiveSlowStarts int
}

// NewAdaptiveTimeout creates a new adaptive timeout manager.
func NewAdaptiveTimeout() *AdaptiveTimeout {
	return &AdaptiveTimeout{
		startupTimes:     make([]time.Duration, 0, 10),
		maxSamples:       10,
		baseTimeout:      2 * time.Second,
		maxTimeout:       16 * time.Second,
		loadFactor:       1.0,
	}
}

// RecordStartupTime records a successful proxy startup time for future calculations.
func (at *AdaptiveTimeout) RecordStartupTime(duration time.Duration) {
	at.mu.Lock()
	defer at.mu.Unlock()

	if len(at.startupTimes) >= at.maxSamples {
		at.startupTimes = at.startupTimes[1:]
	}
	at.startupTimes = append(at.startupTimes, duration)

	// Reset consecutive slow starts on successful startup
	if duration < at.baseTimeout*2 {
		at.consecutiveSlowStarts = 0
	} else {
		at.consecutiveSlowStarts++
	}
}

// GetTimeout calculates the appropriate timeout based on historical data and system conditions.
func (at *AdaptiveTimeout) GetTimeout() time.Duration {
	at.mu.RLock()
	defer at.mu.RUnlock()

	if len(at.startupTimes) == 0 {
		return at.baseTimeout
	}

	// Calculate average startup time
	var total time.Duration
	for _, t := range at.startupTimes {
		total += t
	}
	avgTime := total / time.Duration(len(at.startupTimes))

	// Apply load factor and consecutive slow start penalty
	timeout := time.Duration(float64(avgTime) * at.loadFactor * 2.5)
	
	// Add penalty for consecutive slow starts
	if at.consecutiveSlowStarts > 0 {
		penalty := time.Duration(at.consecutiveSlowStarts) * time.Second
		timeout += penalty
	}

	// Ensure timeout is within bounds
	if timeout < at.baseTimeout {
		timeout = at.baseTimeout
	}
	if timeout > at.maxTimeout {
		timeout = at.maxTimeout
	}

	return timeout
}

// UpdateLoadFactor updates the system load factor for timeout calculation.
func (at *AdaptiveTimeout) UpdateLoadFactor(factor float64) {
	at.mu.Lock()
	defer at.mu.Unlock()
	
	if factor < 0.5 {
		factor = 0.5
	}
	if factor > 3.0 {
		factor = 3.0
	}
	
	at.loadFactor = factor
}

// GetStats returns current statistics for monitoring.
func (at *AdaptiveTimeout) GetStats() (avgTime time.Duration, sampleCount int, currentTimeout time.Duration) {
	at.mu.RLock()
	defer at.mu.RUnlock()

	if len(at.startupTimes) == 0 {
		return 0, 0, at.baseTimeout
	}

	var total time.Duration
	for _, t := range at.startupTimes {
		total += t
	}
	avgTime = total / time.Duration(len(at.startupTimes))
	
	return avgTime, len(at.startupTimes), at.GetTimeout()
}