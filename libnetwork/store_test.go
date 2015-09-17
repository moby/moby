package libnetwork

import (
	"testing"

	"github.com/docker/libnetwork/config"
)

func TestZooKeeperBackend(t *testing.T) {
	testNewController(t, "zk", "127.0.0.1:2181")
}

func testNewController(t *testing.T, provider, url string) error {
	netOptions := []config.Option{}
	netOptions = append(netOptions, config.OptionKVProvider(provider))
	netOptions = append(netOptions, config.OptionKVProviderURL(url))

	_, err := New(netOptions...)
	return err
}
