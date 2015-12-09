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
		t.Fatalf("Heatbeat - Expected : %v, Actual : %v", 10*time.Second, heartbeat)
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
		t.Fatalf("Heatbeat - Expected : %v, Actual : %v", 10*time.Second, heartbeat)
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
		t.Fatalf("Heatbeat - Expected : %v, Actual : %v", expected, heartbeat)
	}

	clusterOpts = map[string]string{}
	heartbeat, ttl, err = discoveryOpts(clusterOpts)
	if err != nil {
		t.Fatal(err)
	}

	if heartbeat != defaultDiscoveryHeartbeat {
		t.Fatalf("Heatbeat - Expected : %v, Actual : %v", defaultDiscoveryHeartbeat, heartbeat)
	}

	expected = defaultDiscoveryHeartbeat * defaultDiscoveryTTLFactor
	if ttl != expected {
		t.Fatalf("TTL - Expected : %v, Actual : %v", expected, ttl)
	}
}
