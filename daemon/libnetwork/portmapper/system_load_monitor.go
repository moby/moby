package portmapper

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/containerd/log"
)

// SystemLoadMonitor monitors system load and updates adaptive timeout accordingly.
type SystemLoadMonitor struct {
	mu              sync.RWMutex
	adaptiveTimeout *AdaptiveTimeout
	ctx             context.Context
	cancel          context.CancelFunc
	running         bool
	updateInterval  time.Duration
}

// NewSystemLoadMonitor creates a new system load monitor.
func NewSystemLoadMonitor(at *AdaptiveTimeout) *SystemLoadMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &SystemLoadMonitor{
		adaptiveTimeout: at,
		ctx:             ctx,
		cancel:          cancel,
		updateInterval:  30 * time.Second,
	}
}

// Start begins monitoring system load and updating timeout factors.
func (slm *SystemLoadMonitor) Start() {
	slm.mu.Lock()
	defer slm.mu.Unlock()

	if slm.running {
		return
	}

	slm.running = true
	go slm.monitorLoop()
}

// Stop stops the system load monitoring.
func (slm *SystemLoadMonitor) Stop() {
	slm.mu.Lock()
	defer slm.mu.Unlock()

	if !slm.running {
		return
	}

	slm.running = false
	slm.cancel()
}

// monitorLoop runs the monitoring loop.
func (slm *SystemLoadMonitor) monitorLoop() {
	ticker := time.NewTicker(slm.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-slm.ctx.Done():
			return
		case <-ticker.C:
			slm.updateLoadFactor()
		}
	}
}

// updateLoadFactor calculates and updates the load factor based on system metrics.
func (slm *SystemLoadMonitor) updateLoadFactor() {
	loadFactor := slm.calculateLoadFactor()
	slm.adaptiveTimeout.UpdateLoadFactor(loadFactor)

	log.G(slm.ctx).WithField("load_factor", loadFactor).Debug("Updated adaptive timeout load factor")
}

// calculateLoadFactor calculates load factor based on available system metrics.
func (slm *SystemLoadMonitor) calculateLoadFactor() float64 {
	// Base load factor
	loadFactor := 1.0

	// Factor in goroutine count (proxy of system activity)
	numGoroutines := runtime.NumGoroutine()
	if numGoroutines > 1000 {
		loadFactor += 0.5
	} else if numGoroutines > 500 {
		loadFactor += 0.3
	} else if numGoroutines > 100 {
		loadFactor += 0.1
	}

	// Factor in memory pressure
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// If heap is growing rapidly, increase load factor
	heapInUseMB := memStats.HeapInuse / 1024 / 1024
	if heapInUseMB > 1000 {
		loadFactor += 0.4
	} else if heapInUseMB > 500 {
		loadFactor += 0.2
	}

	// Factor in GC pressure
	if memStats.NumGC > 0 {
		// Recent GC activity indicates system stress
		gcPauseMicros := memStats.PauseNs[(memStats.NumGC+255)%256] / 1000
		if gcPauseMicros > 10000 { // > 10ms GC pause
			loadFactor += 0.3
		} else if gcPauseMicros > 5000 { // > 5ms GC pause
			loadFactor += 0.1
		}
	}

	// Factor in CPU count (more CPUs = better parallelism)
	numCPU := runtime.NumCPU()
	if numCPU >= 8 {
		loadFactor *= 0.9 // Reduce load factor for high-CPU systems
	} else if numCPU <= 2 {
		loadFactor *= 1.2 // Increase load factor for low-CPU systems
	}

	return loadFactor
}

// GetCurrentLoadFactor returns the current calculated load factor.
func (slm *SystemLoadMonitor) GetCurrentLoadFactor() float64 {
	return slm.calculateLoadFactor()
}

// IsRunning returns whether the monitor is currently running.
func (slm *SystemLoadMonitor) IsRunning() bool {
	slm.mu.RLock()
	defer slm.mu.RUnlock()
	return slm.running
}

// SetUpdateInterval sets the interval for load factor updates.
func (slm *SystemLoadMonitor) SetUpdateInterval(interval time.Duration) {
	slm.mu.Lock()
	defer slm.mu.Unlock()
	slm.updateInterval = interval
}