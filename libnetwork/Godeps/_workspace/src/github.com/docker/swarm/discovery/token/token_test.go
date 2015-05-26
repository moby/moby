package token

import (
	"testing"
	"time"

	"github.com/docker/swarm/discovery"
	"github.com/stretchr/testify/assert"
)

func TestInitialize(t *testing.T) {
	discovery := &Discovery{}
	err := discovery.Initialize("token", 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, discovery.token, "token")
	assert.Equal(t, discovery.url, DiscoveryURL)

	err = discovery.Initialize("custom/path/token", 0, 0)
	assert.NoError(t, err)
	assert.Equal(t, discovery.token, "token")
	assert.Equal(t, discovery.url, "https://custom/path")

	err = discovery.Initialize("", 0, 0)
	assert.Error(t, err)
}

func TestRegister(t *testing.T) {
	d := &Discovery{token: "TEST_TOKEN", url: DiscoveryURL, heartbeat: 1}
	expected := "127.0.0.1:2675"
	expectedEntries, err := discovery.CreateEntries([]string{expected})
	assert.NoError(t, err)

	// Register
	assert.NoError(t, d.Register(expected))

	// Watch
	ch, errCh := d.Watch(nil)
	select {
	case entries := <-ch:
		assert.True(t, entries.Equals(expectedEntries))
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out")
	}

	assert.NoError(t, d.Register(expected))
}
