package runconfig

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/pkg/stringutils"
)

func TestDecodeContainerConfig(t *testing.T) {
	fixtures := []struct {
		file       string
		entrypoint *stringutils.StrSlice
	}{
		{"fixtures/container_config_1_14.json", stringutils.NewStrSlice()},
		{"fixtures/container_config_1_17.json", stringutils.NewStrSlice("bash")},
		{"fixtures/container_config_1_19.json", stringutils.NewStrSlice("bash")},
	}

	for _, f := range fixtures {
		b, err := ioutil.ReadFile(f.file)
		if err != nil {
			t.Fatal(err)
		}

		c, h, err := DecodeContainerConfig(bytes.NewReader(b))
		if err != nil {
			t.Fatal(fmt.Errorf("Error parsing %s: %v", f, err))
		}

		if c.Image != "ubuntu" {
			t.Fatalf("Expected ubuntu image, found %s\n", c.Image)
		}

		if c.Entrypoint.Len() != f.entrypoint.Len() {
			t.Fatalf("Expected %v, found %v\n", f.entrypoint, c.Entrypoint)
		}

		if h.Memory != 1000 {
			t.Fatalf("Expected memory to be 1000, found %d\n", h.Memory)
		}
	}
}
