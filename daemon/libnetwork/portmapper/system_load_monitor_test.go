package portmapper

import (
	"testing"
	"time"
)

func TestNewSystemLoadMonitor(t *testing.T) {
	at := NewAdaptiveTimeout()
	slm := NewSystemLoadMonitor(at)

	if slm.adaptiveTimeout != at {
		t.Error("SystemLoadMonitor should reference the provided AdaptiveTimeout")
	}

	if slm.updateInterval != 30*time.Second {
		t.Errorf("Expected update interval 30s, got %v", slm.updateInterval)
	}

	if slm.IsRunning() {
		t.Error("SystemLoadMonitor should not be running initially")
	}
}

func TestSystemLoadMonitorStartStop(t *testing.T) {
	at := NewAdaptiveTimeout()
	slm := NewSystemLoadMonitor(at)

	// Test start
	slm.Start()
	if !slm.IsRunning() {
		t.Error("SystemLoadMonitor should be running after Start()")
	}

	// Test double start (should not cause issues)
	slm.Start()
	if !slm.IsRunning() {
		t.Error("SystemLoadMonitor should still be running after double Start()")
	}

	// Test stop
	slm.Stop()
	if slm.IsRunning() {
		t.Error("SystemLoadMonitor should not be running after Stop()")
	}

	// Test double stop (should not cause issues)
	slm.Stop()
	if slm.IsRunning() {
		t.Error("SystemLoadMonitor should still not be running after double Stop()")
	}
}

func TestCalculateLoadFactor(t *testing.T) {
	at := NewAdaptiveTimeout()
	slm := NewSystemLoadMonitor(at)

	loadFactor := slm.calculateLoadFactor()

	// Load factor should be reasonable
	if loadFactor < 0.5 {
		t.Errorf("Load factor seems too low: %v", loadFactor)
	}

	if loadFactor > 5.0 {
		t.Errorf("Load factor seems too high: %v", loadFactor)
	}
}

func TestGetCurrentLoadFactor(t *testing.T) {
	at := NewAdaptiveTimeout()
	slm := NewSystemLoadMonitor(at)

	loadFactor1 := slm.GetCurrentLoadFactor()
	loadFactor2 := slm.calculateLoadFactor()

	// Both methods should return the same value
	if loadFactor1 != loadFactor2 {
		t.Errorf("GetCurrentLoadFactor() and calculateLoadFactor() should return same value: %v vs %v", 
			loadFactor1, loadFactor2)
	}
}

func TestSetUpdateInterval(t *testing.T) {
	at := NewAdaptiveTimeout()
	slm := NewSystemLoadMonitor(at)

	newInterval := 10 * time.Second
	slm.SetUpdateInterval(newInterval)

	if slm.updateInterval != newInterval {
		t.Errorf("Expected update interval %v, got %v", newInterval, slm.updateInterval)
	}
}

func TestLoadFactorComponents(t *testing.T) {
	at := NewAdaptiveTimeout()
	slm := NewSystemLoadMonitor(at)

	// Test that load factor calculation doesn't panic
	loadFactor := slm.calculateLoadFactor()

	// Should be a positive number
	if loadFactor <= 0 {
		t.Errorf("Load factor should be positive, got %v", loadFactor)
	}

	// Should be within reasonable bounds
	if loadFactor > 10 {
		t.Errorf("Load factor seems unreasonably high: %v", loadFactor)
	}
}

func TestSystemLoadMonitorIntegration(t *testing.T) {
	at := NewAdaptiveTimeout()
	slm := NewSystemLoadMonitor(at)

	// Get initial load factor from adaptive timeout
	initialLoadFactor := at.loadFactor

	// Start monitoring
	slm.Start()
	defer slm.Stop()

	// Manually trigger update
	slm.updateLoadFactor()

	// Load factor in adaptive timeout should potentially be updated
	// (may be the same if system conditions haven't changed significantly)
	updatedLoadFactor := at.loadFactor

	// Both should be reasonable values
	if initialLoadFactor <= 0 || updatedLoadFactor <= 0 {
		t.Errorf("Load factors should be positive: initial=%v, updated=%v", 
			initialLoadFactor, updatedLoadFactor)
	}
}

func TestMonitoringLoop(t *testing.T) {
	at := NewAdaptiveTimeout()
	slm := NewSystemLoadMonitor(at)

	// Set a very short update interval for testing
	slm.SetUpdateInterval(100 * time.Millisecond)

	// Start monitoring
	slm.Start()
	defer slm.Stop()

	// Wait for a few update cycles
	time.Sleep(350 * time.Millisecond)

	// Monitor should still be running
	if !slm.IsRunning() {
		t.Error("Monitor should still be running after update cycles")
	}

	// Stop and verify it stops
	slm.Stop()
	if slm.IsRunning() {
		t.Error("Monitor should be stopped")
	}
}