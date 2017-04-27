package container

import (
	"github.com/docker/docker/api/types"
	"testing"
)

func TestCalculateMemUsageUnixNoCache(t *testing.T) {
	// Given
	stats := types.MemoryStats{Usage: 500, Stats: map[string]uint64{"cache": 400}}

	// When
	result := calculateMemUsageUnixNoCache(stats)

	// Then
	if result != 100 {
		t.Errorf("mem = %d, want 100", result)
	}
}

func TestCalculateMemPercentUnixNoCache(t *testing.T) {
	// Given
	someLimit := float64(100.0)
	noLimit := float64(0.0)
	used := float64(70.0)

	// When and Then
	t.Run("Limit is set", func(t *testing.T) {
		result := calculateMemPercentUnixNoCache(someLimit, used)
		expected := float64(70.0)
		if result != expected {
			t.Errorf("percent = %f, want %f", result, expected)
		}
	})
	t.Run("No limit, no cgroup data", func(t *testing.T) {
		result := calculateMemPercentUnixNoCache(noLimit, used)
		expected := float64(0.0)
		if result != expected {
			t.Errorf("percent = %f, want %f", result, expected)
		}
	})
}
