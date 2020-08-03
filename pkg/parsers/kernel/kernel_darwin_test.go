package kernel

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetRelease(t *testing.T) {
	// example output of "system_profiler SPSoftwareDataType"
	const spSoftwareDataType = `Software:

    System Software Overview:

      System Version: macOS 10.14.6 (18G4032)
      Kernel Version: Darwin 18.7.0
      Boot Volume: fastfood
      Boot Mode: Normal
      Computer Name: Macintosh
      User Name: Foobar (foobar)
      Secure Virtual Memory: Enabled
      System Integrity Protection: Enabled
      Time since boot: 6 days 23:16
`
	release, err := getRelease(spSoftwareDataType)
	assert.NilError(t, err)
	assert.Equal(t, release, "18.7.0")
}
