package main

import (
	"testing"
	"time"
)

func TestGoduration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{2 * time.Hour, "2 * time.Hour"},
		{5 * time.Minute, "5 * time.Minute"},
		{10 * time.Second, "10 * time.Second"},
		{1500 * time.Millisecond, "1 * time.Second"}, // Will print as seconds if possible
		{0, "0 * time.Nanosecond"},
		{123 * time.Nanosecond, "123 * time.Nanosecond"},
	}

	for _, tt := range tests {
		got := goduration(tt.input)
		if got != tt.expected {
			t.Errorf("goduration(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
