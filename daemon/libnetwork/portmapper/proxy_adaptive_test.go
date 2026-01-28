package portmapper

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

func TestStartProxyWithAdaptiveTimeout(t *testing.T) {
	// Skip if docker-proxy is not available
	proxyPath := "/usr/bin/docker-proxy"
	if _, err := os.Stat(proxyPath); os.IsNotExist(err) {
		t.Skip("docker-proxy not found, skipping integration test")
	}

	// Reset adaptive timeout for clean test
	adaptiveTimeout = NewAdaptiveTimeout()

	pb := types.PortBinding{
		Proto:    types.TCP,
		HostIP:   net.ParseIP("127.0.0.1"),
		HostPort: 0, // Let the system choose a port
		IP:       net.ParseIP("127.0.0.1"),
		Port:     8080,
	}

	// First startup - should use base timeout
	initialTimeout := adaptiveTimeout.GetTimeout()
	if initialTimeout != 2*time.Second {
		t.Errorf("Expected initial timeout to be 2s, got %v", initialTimeout)
	}

	// Start proxy
	stop, err := StartProxy(pb, proxyPath, nil)
	if err != nil {
		t.Fatalf("Failed to start proxy: %v", err)
	}
	defer func() {
		if stop != nil {
			stop()
		}
	}()

	// Verify startup time was recorded
	_, sampleCount, _ := adaptiveTimeout.GetStats()
	if sampleCount != 1 {
		t.Errorf("Expected 1 sample to be recorded, got %d", sampleCount)
	}

	// Stop the proxy
	if err := stop(); err != nil {
		t.Errorf("Failed to stop proxy: %v", err)
	}
	stop = nil
}

func TestAdaptiveTimeoutProgression(t *testing.T) {
	// Reset adaptive timeout
	adaptiveTimeout = NewAdaptiveTimeout()

	// Simulate multiple startup times
	startupTimes := []time.Duration{
		500 * time.Millisecond,
		600 * time.Millisecond,
		700 * time.Millisecond,
		1 * time.Second,
		1200 * time.Millisecond,
	}

	var timeouts []time.Duration
	for _, startupTime := range startupTimes {
		adaptiveTimeout.RecordStartupTime(startupTime)
		timeout := adaptiveTimeout.GetTimeout()
		timeouts = append(timeouts, timeout)
	}

	// Verify that timeout adapts based on recorded times
	if len(timeouts) != len(startupTimes) {
		t.Errorf("Expected %d timeouts, got %d", len(startupTimes), len(timeouts))
	}

	// All timeouts should be within bounds
	for i, timeout := range timeouts {
		if timeout < 2*time.Second {
			t.Errorf("Timeout %d should not be less than base timeout: %v", i, timeout)
		}
		if timeout > 16*time.Second {
			t.Errorf("Timeout %d should not exceed max timeout: %v", i, timeout)
		}
	}
}

func TestLoadFactorIntegration(t *testing.T) {
	// Reset adaptive timeout
	adaptiveTimeout = NewAdaptiveTimeout()

	// Record some baseline startup times
	adaptiveTimeout.RecordStartupTime(1 * time.Second)
	adaptiveTimeout.RecordStartupTime(1 * time.Second)
	
	baselineTimeout := adaptiveTimeout.GetTimeout()

	// Simulate high system load
	adaptiveTimeout.UpdateLoadFactor(2.5)
	highLoadTimeout := adaptiveTimeout.GetTimeout()

	// High load should result in higher timeout
	if highLoadTimeout <= baselineTimeout {
		t.Errorf("Expected higher timeout under high load: baseline=%v, high_load=%v", 
			baselineTimeout, highLoadTimeout)
	}

	// Simulate low system load
	adaptiveTimeout.UpdateLoadFactor(0.8)
	lowLoadTimeout := adaptiveTimeout.GetTimeout()

	// Low load should result in lower timeout than high load
	if lowLoadTimeout >= highLoadTimeout {
		t.Errorf("Expected lower timeout under low load: high_load=%v, low_load=%v", 
			highLoadTimeout, lowLoadTimeout)
	}
}

func TestConsecutiveSlowStartsPenalty(t *testing.T) {
	// Reset adaptive timeout
	adaptiveTimeout = NewAdaptiveTimeout()

	// Record normal startup time
	adaptiveTimeout.RecordStartupTime(1 * time.Second)
	normalTimeout := adaptiveTimeout.GetTimeout()

	// Record consecutive slow starts
	adaptiveTimeout.RecordStartupTime(5 * time.Second) // First slow start
	firstSlowTimeout := adaptiveTimeout.GetTimeout()

	adaptiveTimeout.RecordStartupTime(6 * time.Second) // Second slow start
	secondSlowTimeout := adaptiveTimeout.GetTimeout()

	// Verify penalty is applied
	if firstSlowTimeout <= normalTimeout {
		t.Errorf("Expected penalty after first slow start: normal=%v, first_slow=%v", 
			normalTimeout, firstSlowTimeout)
	}

	if secondSlowTimeout <= firstSlowTimeout {
		t.Errorf("Expected higher penalty after consecutive slow starts: first=%v, second=%v", 
			firstSlowTimeout, secondSlowTimeout)
	}

	// Record fast start to reset penalty
	adaptiveTimeout.RecordStartupTime(500 * time.Millisecond)
	resetTimeout := adaptiveTimeout.GetTimeout()

	// Should be lower after reset
	if resetTimeout >= secondSlowTimeout {
		t.Errorf("Expected timeout to decrease after fast start: slow=%v, reset=%v", 
			secondSlowTimeout, resetTimeout)
	}
}

func TestTimeoutBoundaryConditions(t *testing.T) {
	// Reset adaptive timeout
	adaptiveTimeout = NewAdaptiveTimeout()

	// Test minimum boundary
	adaptiveTimeout.RecordStartupTime(10 * time.Millisecond)
	adaptiveTimeout.UpdateLoadFactor(0.5)
	minTimeout := adaptiveTimeout.GetTimeout()

	if minTimeout < 2*time.Second {
		t.Errorf("Timeout should not go below base timeout: got %v", minTimeout)
	}

	// Test maximum boundary
	adaptiveTimeout.RecordStartupTime(20 * time.Second)
	adaptiveTimeout.UpdateLoadFactor(3.0)
	// Add consecutive slow start penalty
	adaptiveTimeout.RecordStartupTime(25 * time.Second)
	maxTimeout := adaptiveTimeout.GetTimeout()

	if maxTimeout > 16*time.Second {
		t.Errorf("Timeout should not exceed max timeout: got %v", maxTimeout)
	}
}