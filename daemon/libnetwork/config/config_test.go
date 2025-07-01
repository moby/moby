package config

import (
	"strings"
	"testing"

	"github.com/docker/docker/libnetwork/netlabel"
)

func TestOptionsLabels(t *testing.T) {
	c := &Config{}
	l := []string{
		"com.docker.network.key1=value1",
		"com.docker.storage.key1=value1",
		"com.docker.network.driver.key1=value1",
		"com.docker.network.driver.key2=value2",
	}
	f := OptionLabels(l)
	f(c)
	if len(c.Labels) != 3 {
		t.Fatalf("Expecting 3 labels, seen %d", len(c.Labels))
	}
	for _, l := range c.Labels {
		if !strings.HasPrefix(l, netlabel.Prefix) {
			t.Fatalf("config must accept only libnetwork labels. Not : %s", l)
		}
	}
}
