package config

import (
	"testing"

	_ "github.com/docker/libnetwork/netutils"
)

func TestInvalidConfig(t *testing.T) {
	_, err := ParseConfig("invalid.toml")
	if err == nil {
		t.Fatal("Invalid Configuration file must fail")
	}
}

func TestConfig(t *testing.T) {
	_, err := ParseConfig("libnetwork.toml")
	if err != nil {
		t.Fatal("Error parsing a valid configuration file :", err)
	}
}
