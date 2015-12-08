package runconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/stringutils"
)

type f struct {
	file       string
	entrypoint *stringutils.StrSlice
}

func TestDecodeContainerConfig(t *testing.T) {

	var (
		fixtures []f
		image    string
	)

	if runtime.GOOS != "windows" {
		image = "ubuntu"
		fixtures = []f{
			{"fixtures/unix/container_config_1_14.json", stringutils.NewStrSlice()},
			{"fixtures/unix/container_config_1_17.json", stringutils.NewStrSlice("bash")},
			{"fixtures/unix/container_config_1_19.json", stringutils.NewStrSlice("bash")},
		}
	} else {
		image = "windows"
		fixtures = []f{
			{"fixtures/windows/container_config_1_19.json", stringutils.NewStrSlice("cmd")},
		}
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

		if c.Image != image {
			t.Fatalf("Expected %s image, found %s\n", image, c.Image)
		}

		if c.Entrypoint.Len() != f.entrypoint.Len() {
			t.Fatalf("Expected %v, found %v\n", f.entrypoint, c.Entrypoint)
		}

		if h != nil && h.Memory != 1000 {
			t.Fatalf("Expected memory to be 1000, found %d\n", h.Memory)
		}
	}
}

// TestDecodeContainerConfigIsolation validates the isolation level passed
// to the daemon in the hostConfig structure. Note this is platform specific
// as to what level of container isolation is supported.
func TestDecodeContainerConfigIsolation(t *testing.T) {

	// An invalid isolation level
	if _, _, err := callDecodeContainerConfigIsolation("invalid"); err != nil {
		if !strings.Contains(err.Error(), `invalid --isolation: "invalid"`) {
			t.Fatal(err)
		}
	}

	// Blank isolation level (== default)
	if _, _, err := callDecodeContainerConfigIsolation(""); err != nil {
		t.Fatal("Blank isolation should have succeeded")
	}

	// Default isolation level
	if _, _, err := callDecodeContainerConfigIsolation("default"); err != nil {
		t.Fatal("default isolation should have succeeded")
	}

	// Hyper-V Containers isolation level (Valid on Windows only)
	if runtime.GOOS == "windows" {
		if _, _, err := callDecodeContainerConfigIsolation("hyperv"); err != nil {
			t.Fatal("hyperv isolation should have succeeded")
		}
	} else {
		if _, _, err := callDecodeContainerConfigIsolation("hyperv"); err != nil {
			if !strings.Contains(err.Error(), `invalid --isolation: "hyperv"`) {
				t.Fatal(err)
			}
		}
	}
}

// callDecodeContainerConfigIsolation is a utility function to call
// DecodeContainerConfig for validating isolation levels
func callDecodeContainerConfigIsolation(isolation string) (*Config, *HostConfig, error) {
	var (
		b   []byte
		err error
	)
	w := ContainerConfigWrapper{
		Config: &Config{},
		HostConfig: &HostConfig{
			NetworkMode: "none",
			Isolation:   IsolationLevel(isolation)},
	}
	if b, err = json.Marshal(w); err != nil {
		return nil, nil, fmt.Errorf("Error on marshal %s", err.Error())
	}
	return DecodeContainerConfig(bytes.NewReader(b))
}
