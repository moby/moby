package portmapper

import (
	"testing"
	"time"
)

func TestNewAdaptiveTimeout(t *testing.T) {
	at := NewAdaptiveTimeout()
	
	if at.baseTimeout != 2*time.Second {
		t.Errorf("Expected base timeout 2s, got %v", at.baseTimeout)
	}
	
	if at.maxTimeout != 16*time.Second {
		t.Errorf("Expected max timeout 16s, got %v", at.maxTimeout)
	}
	
	if at.loadFactor != 1.0 {
		t.Errorf("Expected load factor 1.0, got %v", at.loadFactor)
	}
	
	timeout := at.GetTimeout()
	if timeout != at.baseTimeout {
		t.Errorf("Expected initial timeout to be base timeout %v, got %v", at.baseTimeout, timeout)
	}
}

func TestRecordStartupTime(t *testing.T) {
	at := NewAdaptiveTimeout()
	
	// Record some startup times
	at.RecordStartupTime(1 * time.Second)
	at.RecordStartupTime(2 * time.Second)
	at.RecordStartupTime(3 * time.Second)
	
	avgTime, sampleCount, _ := at.GetStats()
	expectedAvg := 2 * time.Second
	
	if sampleCount != 3 {
		t.Errorf("Expected 3 samples, got %d", sampleCount)
	}
	
	if avgTime != expectedAvg {
		t.Errorf("Expected average time %v, got %v", expectedAvg, avgTime)
	}
}

func TestGetTimeoutWithHistory(t *testing.T) {
	at := NewAdaptiveTimeout()
	
	// Record fast startup times
	at.RecordStartupTime(500 * time.Millisecond)
	at.RecordStartupTime(600 * time.Millisecond)
	at.RecordStartupTime(700 * time.Millisecond)
	
	timeout := at.GetTimeout()
	
	// Should be based on average (600ms) * 2.5 = 1.5s, but clamped to base timeout (2s)
	if timeout < at.baseTimeout {
		t.Errorf("Timeout should not be less than base timeout")
	}
	
	if timeout > at.maxTimeout {
		t.Errorf("Timeout should not exceed max timeout")
	}
}

func TestGetTimeoutWithSlowStarts(t *testing.T) {
	at := NewAdaptiveTimeout()
	
	// Record slow startup times
	at.RecordStartupTime(5 * time.Second)
	at.RecordStartupTime(6 * time.Second)
	at.RecordStartupTime(7 * time.Second)
	
	timeout := at.GetTimeout()
	
	// Should be higher due to slow starts
	if timeout <= at.baseTimeout {
		t.Errorf("Expected timeout to be higher than base timeout for slow starts")
	}
}

func TestConsecutiveSlowStarts(t *testing.T) {
	at := NewAdaptiveTimeout()
	
	// Record consecutive slow starts
	at.RecordStartupTime(5 * time.Second) // Slow start
	timeout1 := at.GetTimeout()
	
	at.RecordStartupTime(6 * time.Second) // Another slow start
	timeout2 := at.GetTimeout()
	
	// Second timeout should be higher due to consecutive slow starts
	if timeout2 <= timeout1 {
		t.Errorf("Expected timeout to increase with consecutive slow starts")
	}
	
	// Record a fast start to reset
	at.RecordStartupTime(500 * time.Millisecond)
	timeout3 := at.GetTimeout()
	
	// Should be lower after fast start
	if timeout3 >= timeout2 {
		t.Errorf("Expected timeout to decrease after fast start")
	}
}

func TestUpdateLoadFactor(t *testing.T) {
	at := NewAdaptiveTimeout()
	
	// Test normal load factor
	at.UpdateLoadFactor(1.5)
	if at.loadFactor != 1.5 {
		t.Errorf("Expected load factor 1.5, got %v", at.loadFactor)
	}
	
	// Test clamping to minimum
	at.UpdateLoadFactor(0.1)
	if at.loadFactor != 0.5 {
		t.Errorf("Expected load factor to be clamped to 0.5, got %v", at.loadFactor)
	}
	
	// Test clamping to maximum
	at.UpdateLoadFactor(5.0)
	if at.loadFactor != 3.0 {
		t.Errorf("Expected load factor to be clamped to 3.0, got %v", at.loadFactor)
	}
}

func TestLoadFactorImpactOnTimeout(t *testing.T) {
	at := NewAdaptiveTimeout()
	
	// Record some startup times
	at.RecordStartupTime(1 * time.Second)
	at.RecordStartupTime(1 * time.Second)
	
	// Get timeout with normal load factor
	timeout1 := at.GetTimeout()
	
	// Increase load factor
	at.UpdateLoadFactor(2.0)
	timeout2 := at.GetTimeout()
	
	// Timeout should be higher with higher load factor
	if timeout2 <= timeout1 {
		t.Errorf("Expected timeout to increase with higher load factor")
	}
}

func TestMaxSamplesLimit(t *testing.T) {
	at := NewAdaptiveTimeout()
	
	// Record more samples than the limit
	for i := 0; i < 15; i++ {
		at.RecordStartupTime(time.Duration(i+1) * time.Second)
	}
	
	_, sampleCount, _ := at.GetStats()
	
	if sampleCount != at.maxSamples {
		t.Errorf("Expected sample count to be limited to %d, got %d", at.maxSamples, sampleCount)
	}
}

func TestTimeoutBounds(t *testing.T) {
	at := NewAdaptiveTimeout()
	
	// Test with very fast startup times
	at.RecordStartupTime(10 * time.Millisecond)
	at.RecordStartupTime(20 * time.Millisecond)
	
	timeout := at.GetTimeout()
	if timeout < at.baseTimeout {
		t.Errorf("Timeout should not be less than base timeout %v, got %v", at.baseTimeout, timeout)
	}
	
	// Test with very slow startup times and high load factor
	at.UpdateLoadFactor(3.0)
	at.RecordStartupTime(10 * time.Second)
	at.RecordStartupTime(12 * time.Second)
	
	timeout = at.GetTimeout()
	if timeout > at.maxTimeout {
		t.Errorf("Timeout should not exceed max timeout %v, got %v", at.maxTimeout, timeout)
	}
}

func TestConcurrentAccess(t *testing.T) {
	at := NewAdaptiveTimeout()
	
	// Test concurrent access
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(i int) {
			at.RecordStartupTime(time.Duration(i+1) * time.Second)
			at.GetTimeout()
			at.UpdateLoadFactor(1.5)
			at.GetStats()
			done <- true
		}(i)
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	// Should not panic and should have recorded some samples
	_, sampleCount, _ := at.GetStats()
	if sampleCount == 0 {
		t.Error("Expected some samples to be recorded")
	}
}