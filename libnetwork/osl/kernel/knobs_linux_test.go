package kernel

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	_ "github.com/docker/libnetwork/testutils"
)

func TestReadWriteKnobs(t *testing.T) {
	for _, k := range []string{
		"net.ipv4.neigh.default.gc_thresh1",
		"net.ipv4.neigh.default.gc_thresh2",
		"net.ipv4.neigh.default.gc_thresh3",
	} {
		// Check if the test is able to read the value
		v, err := readSystemProperty(k)
		if err != nil {
			logrus.WithError(err).Warnf("Path %v not readable", k)
			// the path is not there, skip this key
			continue
		}
		// Test the write
		assert.NoError(t, writeSystemProperty(k, "10000"))
		newV, err := readSystemProperty(k)
		assert.NoError(t, err)
		assert.Equal(t, newV, "10000")
		// Restore value
		assert.NoError(t, writeSystemProperty(k, v))
	}
}
