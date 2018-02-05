package dmesg // import "github.com/docker/docker/pkg/dmesg"

import (
	"testing"
)

func TestDmesg(t *testing.T) {
	t.Logf("dmesg output follows:\n%v", string(Dmesg(512)))
}
