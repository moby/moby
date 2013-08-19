package docker

import (
	"os"
	"testing"
)

func Test(t *testing.T) {
	os.Setenv("TEST", "1")
}
