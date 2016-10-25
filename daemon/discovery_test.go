package daemon

import (
	"testing"
	"time"
)

func TestDiscoveryOpts(t *testing.T) {
	clusterOpts := map[string]string{"discovery.heartbeat": "10", "discovery.ttl": "5"}
	heartbeat, ttl, err := discoveryOpts(clusterOpts)
	if err == nil {
		t.Fatalf("discovery.ttl < discovery.heartbeat must fail")
	}

	clusterOpts = map[string]string{"discovery.heartbeat": "10", "discovery.ttl": "10"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err == nil {
		t.Fatalf("discovery.ttl == discovery.heartbeat must fail")
	}

	clusterOpts = map[string]string{"discovery.heartbeat": "-10", "discovery.ttl": "10"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err == nil {
		t.Fatalf("negative discovery.heartbeat must fail")
	}

	clusterOpts = map[string]string{"discovery.heartbeat": "10", "discovery.ttl": "-10"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err == nil {
		t.Fatalf("negative discovery.ttl must fail")
	}

	clusterOpts = map[string]string{"discovery.heartbeat": "invalid"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err == nil {
		t.Fatalf("invalid discovery.heartbeat must fail")
	}

	clusterOpts = map[string]string{"discovery.ttl": "invalid"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err == nil {
		t.Fatalf("invalid discovery.ttl must fail")
	}

	clusterOpts = map[string]string{"discovery.heartbeat": "10", "discovery.ttl": "20"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err != nil {
		t.Fatal(err)
	}

	if heartbeat != 10*time.Second {
		t.Fatalf("Heartbeat - Expected : %v, Actual : %v", 10*time.Second, heartbeat)
	}

	if ttl != 20*time.Second {
		t.Fatalf("TTL - Expected : %v, Actual : %v", 20*time.Second, ttl)
	}

	clusterOpts = map[string]string{"discovery.heartbeat": "10"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err != nil {
		t.Fatal(err)
	}

	if heartbeat != 10*time.Second {
		t.Fatalf("Heartbeat - Expected : %v, Actual : %v", 10*time.Second, heartbeat)
	}

	expected := 10 * defaultDiscoveryTTLFactor * time.Second
	if ttl != expected {
		t.Fatalf("TTL - Expected : %v, Actual : %v", expected, ttl)
	}

	clusterOpts = map[string]string{"discovery.ttl": "30"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err != nil {
		t.Fatal(err)
	}

	if ttl != 30*time.Second {
		t.Fatalf("TTL - Expected : %v, Actual : %v", 30*time.Second, ttl)
	}

	expected = 30 * time.Second / defaultDiscoveryTTLFactor
	if heartbeat != expected {
		t.Fatalf("Heartbeat - Expected : %v, Actual : %v", expected, heartbeat)
	}

	clusterOpts = map[string]string{}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err != nil {
		t.Fatal(err)
	}

	if heartbeat != defaultDiscoveryHeartbeat {
		t.Fatalf("Heartbeat - Expected : %v, Actual : %v", defaultDiscoveryHeartbeat, heartbeat)
	}

	expected = defaultDiscoveryHeartbeat * defaultDiscoveryTTLFactor
	if ttl != expected {
		t.Fatalf("TTL - Expected : %v, Actual : %v", expected, ttl)
	}
}

func TestModifiedDiscoverySettings(t *testing.T) {
	cases := []struct {
		current  *Config
		modified *Config
		expected bool
	}{
		{
			current:  discoveryConfig("foo", "bar", map[string]string{}),
			modified: discoveryConfig("foo", "bar", map[string]string{}),
			expected: false,
		},
		{
			current:  discoveryConfig("foo", "bar", map[string]string{"foo": "bar"}),
			modified: discoveryConfig("foo", "bar", map[string]string{"foo": "bar"}),
			expected: false,
		},
		{
			current:  discoveryConfig("foo", "bar", map[string]string{}),
			modified: discoveryConfig("foo", "bar", nil),
			expected: false,
		},
		{
			current:  discoveryConfig("foo", "bar", nil),
			modified: discoveryConfig("foo", "bar", map[string]string{}),
			expected: false,
		},
		{
			current:  discoveryConfig("foo", "bar", nil),
			modified: discoveryConfig("baz", "bar", nil),
			expected: true,
		},
		{
			current:  discoveryConfig("foo", "bar", nil),
			modified: discoveryConfig("foo", "baz", nil),
			expected: true,
		},
		{
			current:  discoveryConfig("foo", "bar", nil),
			modified: discoveryConfig("foo", "bar", map[string]string{"foo": "bar"}),
			expected: true,
		},
	}

	for _, c := range cases {
		got := modifiedDiscoverySettings(c.current, c.modified.ClusterStore, c.modified.ClusterAdvertise, c.modified.ClusterOpts)
		if c.expected != got {
			t.Fatalf("expected %v, got %v: current config %v, new config %v", c.expected, got, c.current, c.modified)
		}
	}
}

func discoveryConfig(backendAddr, advertiseAddr string, opts map[string]string) *Config {
	return &Config{
		CommonConfig: CommonConfig{
			ClusterStore:     backendAddr,
			ClusterAdvertise: advertiseAddr,
			ClusterOpts:      opts,
		},
	}
}
