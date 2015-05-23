package config

import (
	"testing"
)

func TestInvalidConfig(t *testing.T) {
	_, err := ParseConfig("invalid.toml")
	if err == nil {
		t.Fatal("Invalid Configuration file must fail")
	}
}

func TestConfig(t *testing.T) {
	cfg, err := ParseConfig("libnetwork.toml")
	if err != nil {
		t.Fatal("Error parsing a valid configuration file :", err)
	}
}
