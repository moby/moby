package kernel

import (
	"testing"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
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
		assert.Check(t, writeSystemProperty(k, "10000"))
		newV, err := readSystemProperty(k)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(newV, "10000"))
		// Restore value
		assert.Check(t, writeSystemProperty(k, v))
	}
}
