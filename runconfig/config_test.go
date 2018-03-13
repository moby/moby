package runconfig // import "github.com/docker/docker/runconfig"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"runtime"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
)

type f struct {
	file       string
	entrypoint strslice.StrSlice
}

func TestDecodeContainerConfig(t *testing.T) {

	var (
		fixtures []f
		image    string
	)

	if runtime.GOOS != "windows" {
		image = "ubuntu"
		fixtures = []f{
			{"fixtures/unix/container_config_1_14.json", strslice.StrSlice{}},
			{"fixtures/unix/container_config_1_17.json", strslice.StrSlice{"bash"}},
			{"fixtures/unix/container_config_1_19.json", strslice.StrSlice{"bash"}},
		}
	} else {
		image = "windows"
		fixtures = []f{
			{"fixtures/windows/container_config_1_19.json", strslice.StrSlice{"cmd"}},
		}
	}

	for _, f := range fixtures {
		b, err := ioutil.ReadFile(f.file)
		if err != nil {
			t.Fatal(err)
		}

		c, h, _, err := decodeContainerConfig(bytes.NewReader(b))
		if err != nil {
			t.Fatal(fmt.Errorf("Error parsing %s: %v", f, err))
		}

		if c.Image != image {
			t.Fatalf("Expected %s image, found %s\n", image, c.Image)
		}

		if len(c.Entrypoint) != len(f.entrypoint) {
			t.Fatalf("Expected %v, found %v\n", f.entrypoint, c.Entrypoint)
		}

		if h != nil && h.Memory != 1000 {
			t.Fatalf("Expected memory to be 1000, found %d\n", h.Memory)
		}
	}
}

// TestDecodeContainerConfigIsolation validates isolation passed
// to the daemon in the hostConfig structure. Note this is platform specific
// as to what level of container isolation is supported.
func TestDecodeContainerConfigIsolation(t *testing.T) {

	// An Invalid isolation level
	if _, _, _, err := callDecodeContainerConfigIsolation("invalid"); err != nil {
		if !strings.Contains(err.Error(), `Invalid isolation: "invalid"`) {
			t.Fatal(err)
		}
	}

	// Blank isolation (== default)
	if _, _, _, err := callDecodeContainerConfigIsolation(""); err != nil {
		t.Fatal("Blank isolation should have succeeded")
	}

	// Default isolation
	if _, _, _, err := callDecodeContainerConfigIsolation("default"); err != nil {
		t.Fatal("default isolation should have succeeded")
	}

	// Process isolation (Valid on Windows only)
	if runtime.GOOS == "windows" {
		if _, _, _, err := callDecodeContainerConfigIsolation("process"); err != nil {
			t.Fatal("process isolation should have succeeded")
		}
	} else {
		if _, _, _, err := callDecodeContainerConfigIsolation("process"); err != nil {
			if !strings.Contains(err.Error(), `Invalid isolation: "process"`) {
				t.Fatal(err)
			}
		}
	}

	// Hyper-V Containers isolation (Valid on Windows only)
	if runtime.GOOS == "windows" {
		if _, _, _, err := callDecodeContainerConfigIsolation("hyperv"); err != nil {
			t.Fatal("hyperv isolation should have succeeded")
		}
	} else {
		if _, _, _, err := callDecodeContainerConfigIsolation("hyperv"); err != nil {
			if !strings.Contains(err.Error(), `Invalid isolation: "hyperv"`) {
				t.Fatal(err)
			}
		}
	}
}

// callDecodeContainerConfigIsolation is a utility function to call
// DecodeContainerConfig for validating isolation
func callDecodeContainerConfigIsolation(isolation string) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {
	var (
		b   []byte
		err error
	)
	w := ContainerConfigWrapper{
		Config: &container.Config{},
		HostConfig: &container.HostConfig{
			NetworkMode: "none",
			Isolation:   container.Isolation(isolation)},
	}
	if b, err = json.Marshal(w); err != nil {
		return nil, nil, nil, fmt.Errorf("Error on marshal %s", err.Error())
	}
	return decodeContainerConfig(bytes.NewReader(b))
}

type decodeConfigTestcase struct {
	doc                string
	wrapper            ContainerConfigWrapper
	expectedErr        string
	expectedConfig     *container.Config
	expectedHostConfig *container.HostConfig
	goos               string
}

func runDecodeContainerConfigTestCase(testcase decodeConfigTestcase) func(t *testing.T) {
	return func(t *testing.T) {
		raw := marshal(t, testcase.wrapper, testcase.doc)
		config, hostConfig, _, err := decodeContainerConfig(bytes.NewReader(raw))
		if testcase.expectedErr != "" {
			if !assert.Check(t, is.ErrorContains(err, "")) {
				return
			}
			assert.Check(t, is.Contains(err.Error(), testcase.expectedErr))
			return
		}
		assert.Check(t, err)
		assert.Check(t, is.DeepEqual(testcase.expectedConfig, config))
		assert.Check(t, is.DeepEqual(testcase.expectedHostConfig, hostConfig))
	}
}

func marshal(t *testing.T, w ContainerConfigWrapper, doc string) []byte {
	b, err := json.Marshal(w)
	assert.NilError(t, err, "%s: failed to encode config wrapper", doc)
	return b
}

func containerWrapperWithVolume(volume string) ContainerConfigWrapper {
	return ContainerConfigWrapper{
		Config: &container.Config{
			Volumes: map[string]struct{}{
				volume: {},
			},
		},
		HostConfig: &container.HostConfig{},
	}
}

func containerWrapperWithBind(bind string) ContainerConfigWrapper {
	return ContainerConfigWrapper{
		Config: &container.Config{
			Volumes: map[string]struct{}{},
		},
		HostConfig: &container.HostConfig{
			Binds: []string{bind},
		},
	}
}
