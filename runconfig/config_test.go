package runconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"runtime"
	"strings"
	"testing"

	"os"

	"github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	//TODO: Should run for Solaris
	if runtime.GOOS == "solaris" {
		t.Skip()
	}

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

func TestDecodeContainerConfigWithVolumes(t *testing.T) {
	var testcases = []decodeConfigTestcase{
		{
			doc:         "no paths volume",
			wrapper:     containerWrapperWithVolume(":"),
			expectedErr: `invalid volume specification: ':'`,
		},
		{
			doc:         "no paths bind",
			wrapper:     containerWrapperWithBind(":"),
			expectedErr: `invalid volume specification: ':'`,
		},
		{
			doc:         "no paths or mode volume",
			wrapper:     containerWrapperWithVolume("::"),
			expectedErr: `invalid volume specification: '::'`,
		},
		{
			doc:         "no paths or mode bind",
			wrapper:     containerWrapperWithBind("::"),
			expectedErr: `invalid volume specification: '::'`,
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.doc, runDecodeContainerConfigTestCase(testcase))
	}
}

func TestDecodeContainerConfigWithVolumesUnix(t *testing.T) {
	skip.IfCondition(t, runtime.GOOS == "windows")

	baseErr := `invalid mount config for type "volume": invalid specification: `
	var testcases = []decodeConfigTestcase{
		{
			doc:         "root to root volume",
			wrapper:     containerWrapperWithVolume("/:/"),
			expectedErr: `invalid volume specification: '/:/'`,
		},
		{
			doc:         "root to root bind",
			wrapper:     containerWrapperWithBind("/:/"),
			expectedErr: `invalid volume specification: '/:/'`,
		},
		{
			doc:         "no destination path volume",
			wrapper:     containerWrapperWithVolume(`/tmp:`),
			expectedErr: ` invalid volume specification: '/tmp:'`,
		},
		{
			doc:         "no destination path bind",
			wrapper:     containerWrapperWithBind(`/tmp:`),
			expectedErr: ` invalid volume specification: '/tmp:'`,
		},
		{
			doc:         "no destination path or mode volume",
			wrapper:     containerWrapperWithVolume(`/tmp::`),
			expectedErr: `invalid mount config for type "bind": field Target must not be empty`,
		},
		{
			doc:         "no destination path or mode bind",
			wrapper:     containerWrapperWithBind(`/tmp::`),
			expectedErr: `invalid mount config for type "bind": field Target must not be empty`,
		},
		{
			doc:         "too many sections volume",
			wrapper:     containerWrapperWithVolume(`/tmp:/tmp:/tmp:/tmp`),
			expectedErr: `invalid volume specification: '/tmp:/tmp:/tmp:/tmp'`,
		},
		{
			doc:         "too many sections bind",
			wrapper:     containerWrapperWithBind(`/tmp:/tmp:/tmp:/tmp`),
			expectedErr: `invalid volume specification: '/tmp:/tmp:/tmp:/tmp'`,
		},
		{
			doc:         "just root volume",
			wrapper:     containerWrapperWithVolume("/"),
			expectedErr: baseErr + `destination can't be '/'`,
		},
		{
			doc:         "just root bind",
			wrapper:     containerWrapperWithBind("/"),
			expectedErr: baseErr + `destination can't be '/'`,
		},
		{
			doc:     "bind mount passed as a volume",
			wrapper: containerWrapperWithVolume(`/foo:/bar`),
			expectedConfig: &container.Config{
				Volumes: map[string]struct{}{`/foo:/bar`: {}},
			},
			expectedHostConfig: &container.HostConfig{NetworkMode: "default"},
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.doc, runDecodeContainerConfigTestCase(testcase))
	}
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
			if !assert.Error(t, err) {
				return
			}
			assert.Contains(t, err.Error(), testcase.expectedErr)
			return
		}
		assert.NoError(t, err)
		assert.Equal(t, testcase.expectedConfig, config)
		assert.Equal(t, testcase.expectedHostConfig, hostConfig)
	}
}

func TestDecodeContainerConfigWithVolumesWindows(t *testing.T) {
	skip.IfCondition(t, runtime.GOOS != "windows")

	tmpDir := os.Getenv("TEMP")
	systemDrive := os.Getenv("SystemDrive")
	var testcases = []decodeConfigTestcase{
		{
			doc:         "root to root volume",
			wrapper:     containerWrapperWithVolume(systemDrive + `\:c:\`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "root to root bind",
			wrapper:     containerWrapperWithBind(systemDrive + `\:c:\`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "no destination path volume",
			wrapper:     containerWrapperWithVolume(tmpDir + `\:`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "no destination path bind",
			wrapper:     containerWrapperWithBind(tmpDir + `\:`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "no destination path or mode volume",
			wrapper:     containerWrapperWithVolume(tmpDir + `\::`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "no destination path or mode bind",
			wrapper:     containerWrapperWithBind(tmpDir + `\::`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "too many sections volume",
			wrapper:     containerWrapperWithVolume(tmpDir + ":" + tmpDir + ":" + tmpDir + ":" + tmpDir),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "too many sections bind",
			wrapper:     containerWrapperWithBind(tmpDir + ":" + tmpDir + ":" + tmpDir + ":" + tmpDir),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "no drive letter volume",
			wrapper:     containerWrapperWithVolume(`\tmp`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "no drive letter bind",
			wrapper:     containerWrapperWithBind(`\tmp`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "root to c-drive volume",
			wrapper:     containerWrapperWithVolume(systemDrive + `\:c:`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "root to c-drive bind",
			wrapper:     containerWrapperWithBind(systemDrive + `\:c:`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "container path without driver letter volume",
			wrapper:     containerWrapperWithVolume(`c:\windows:\somewhere`),
			expectedErr: `invalid volume specification: `,
		},
		{
			doc:         "container path without driver letter bind",
			wrapper:     containerWrapperWithBind(`c:\windows:\somewhere`),
			expectedErr: `invalid volume specification: `,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.doc, runDecodeContainerConfigTestCase(testcase))
	}
}

func marshal(t *testing.T, w ContainerConfigWrapper, doc string) []byte {
	b, err := json.Marshal(w)
	require.NoError(t, err, "%s: failed to encode config wrapper", doc)
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
