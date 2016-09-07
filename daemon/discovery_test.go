package daemon

import (
	"time"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestDiscoveryOpts(c *check.C) {
	clusterOpts := map[string]string{"discovery.heartbeat": "10", "discovery.ttl": "5"}
	heartbeat, ttl, err := discoveryOpts(clusterOpts)
	if err == nil {
		c.Fatalf("discovery.ttl < discovery.heartbeat must fail")
	}

	clusterOpts = map[string]string{"discovery.heartbeat": "10", "discovery.ttl": "10"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err == nil {
		c.Fatalf("discovery.ttl == discovery.heartbeat must fail")
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
		c.Fatalf("invalid discovery.heartbeat must fail")
	}

	clusterOpts = map[string]string{"discovery.ttl": "invalid"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err == nil {
		c.Fatalf("invalid discovery.ttl must fail")
	}

	clusterOpts = map[string]string{"discovery.heartbeat": "10", "discovery.ttl": "20"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err != nil {
		c.Fatal(err)
	}

	if heartbeat != 10*time.Second {
		c.Fatalf("Heartbeat - Expected : %v, Actual : %v", 10*time.Second, heartbeat)
	}

	if ttl != 20*time.Second {
		c.Fatalf("TTL - Expected : %v, Actual : %v", 20*time.Second, ttl)
	}

	clusterOpts = map[string]string{"discovery.heartbeat": "10"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err != nil {
		c.Fatal(err)
	}

	if heartbeat != 10*time.Second {
		c.Fatalf("Heartbeat - Expected : %v, Actual : %v", 10*time.Second, heartbeat)
	}

	expected := 10 * defaultDiscoveryTTLFactor * time.Second
	if ttl != expected {
		c.Fatalf("TTL - Expected : %v, Actual : %v", expected, ttl)
	}

	clusterOpts = map[string]string{"discovery.ttl": "30"}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err != nil {
		c.Fatal(err)
	}

	if ttl != 30*time.Second {
		c.Fatalf("TTL - Expected : %v, Actual : %v", 30*time.Second, ttl)
	}

	expected = 30 * time.Second / defaultDiscoveryTTLFactor
	if heartbeat != expected {
		c.Fatalf("Heartbeat - Expected : %v, Actual : %v", expected, heartbeat)
	}

	clusterOpts = map[string]string{}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err != nil {
		c.Fatal(err)
	}

	if heartbeat != defaultDiscoveryHeartbeat {
		c.Fatalf("Heartbeat - Expected : %v, Actual : %v", defaultDiscoveryHeartbeat, heartbeat)
	}

	expected = defaultDiscoveryHeartbeat * defaultDiscoveryTTLFactor
	if ttl != expected {
		c.Fatalf("TTL - Expected : %v, Actual : %v", expected, ttl)
	}
}

func (s *DockerSuite) TestModifiedDiscoverySettings(c *check.C) {
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

	for _, ca := range cases {
		got := modifiedDiscoverySettings(ca.current, ca.modified.ClusterStore, ca.modified.ClusterAdvertise, ca.modified.ClusterOpts)
		if ca.expected != got {
			c.Fatalf("expected %v, got %v: current config %v, new config %v", ca.expected, got, ca.current, ca.modified)
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
