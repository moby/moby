package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/fs"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	swarmagent "github.com/moby/swarmkit/v2/agent"
	swarmapi "github.com/moby/swarmkit/v2/api"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/windows/registry"
	"gotest.tools/v3/assert"
)

func TestSetWindowsCredentialSpecInSpec(t *testing.T) {
	// we need a temp directory to act as the daemon's root
	tmpDaemonRoot := fs.NewDir(t, t.Name()).Path()
	defer func() {
		assert.NilError(t, os.RemoveAll(tmpDaemonRoot))
	}()

	daemon := &Daemon{
		root: tmpDaemonRoot,
	}

	t.Run("it does nothing if there are no security options", func(t *testing.T) {
		spec := &specs.Spec{}

		err := daemon.setWindowsCredentialSpec(&container.Container{}, spec)
		assert.NilError(t, err)
		assert.Check(t, spec.Windows == nil)

		err = daemon.setWindowsCredentialSpec(&container.Container{HostConfig: &containertypes.HostConfig{}}, spec)
		assert.NilError(t, err)
		assert.Check(t, spec.Windows == nil)

		err = daemon.setWindowsCredentialSpec(&container.Container{HostConfig: &containertypes.HostConfig{SecurityOpt: []string{}}}, spec)
		assert.NilError(t, err)
		assert.Check(t, spec.Windows == nil)
	})

	dummyContainerID := "dummy-container-ID"
	containerFactory := func(secOpt string) *container.Container {
		if !strings.Contains(secOpt, "=") {
			secOpt = "credentialspec=" + secOpt
		}
		return &container.Container{
			ID: dummyContainerID,
			HostConfig: &containertypes.HostConfig{
				SecurityOpt: []string{secOpt},
			},
		}
	}

	credSpecsDir := filepath.Join(tmpDaemonRoot, credentialSpecFileLocation)
	dummyCredFileContents := `{"We don't need no": "education"}`

	t.Run("happy path with a 'file://' option", func(t *testing.T) {
		spec := &specs.Spec{}

		// let's render a dummy cred file
		err := os.Mkdir(credSpecsDir, os.ModePerm)
		assert.NilError(t, err)
		dummyCredFileName := "dummy-cred-spec.json"
		dummyCredFilePath := filepath.Join(credSpecsDir, dummyCredFileName)
		err = os.WriteFile(dummyCredFilePath, []byte(dummyCredFileContents), 0o644)
		defer func() {
			assert.NilError(t, os.Remove(dummyCredFilePath))
		}()
		assert.NilError(t, err)

		err = daemon.setWindowsCredentialSpec(containerFactory("file://"+dummyCredFileName), spec)
		assert.NilError(t, err)

		if assert.Check(t, spec.Windows != nil) {
			assert.Equal(t, dummyCredFileContents, spec.Windows.CredentialSpec)
		}
	})

	t.Run("it's not allowed to use a 'file://' option with an absolute path", func(t *testing.T) {
		spec := &specs.Spec{}

		err := daemon.setWindowsCredentialSpec(containerFactory(`file://C:\path\to\my\credspec.json`), spec)
		assert.ErrorContains(t, err, "invalid credential spec: file:// path cannot be absolute")

		assert.Check(t, spec.Windows == nil)
	})

	t.Run("it's not allowed to use a 'file://' option breaking out of the cred specs' directory", func(t *testing.T) {
		spec := &specs.Spec{}

		err := daemon.setWindowsCredentialSpec(containerFactory(`file://..\credspec.json`), spec)
		assert.ErrorContains(t, err, fmt.Sprintf("invalid credential spec: file:// path must be under %s", credSpecsDir))

		assert.Check(t, spec.Windows == nil)
	})

	t.Run("when using a 'file://' option pointing to a file that doesn't exist, it fails gracefully", func(t *testing.T) {
		spec := &specs.Spec{}

		err := daemon.setWindowsCredentialSpec(containerFactory("file://i-dont-exist.json"), spec)
		assert.Check(t, is.ErrorContains(err, fmt.Sprintf("failed to load credential spec for container %s", dummyContainerID)))
		assert.Check(t, is.ErrorIs(err, os.ErrNotExist))
		assert.Check(t, spec.Windows == nil)
	})

	t.Run("happy path with a 'registry://' option", func(t *testing.T) {
		valueName := "my-cred-spec"
		key := &dummyRegistryKey{
			getStringValueFunc: func(name string) (val string, valtype uint32, err error) {
				assert.Equal(t, valueName, name)
				return dummyCredFileContents, 0, nil
			},
		}
		defer setRegistryOpenKeyFunc(t, key)()

		spec := &specs.Spec{}
		assert.NilError(t, daemon.setWindowsCredentialSpec(containerFactory("registry://"+valueName), spec))

		if assert.Check(t, spec.Windows != nil) {
			assert.Equal(t, dummyCredFileContents, spec.Windows.CredentialSpec)
		}
		assert.Check(t, key.closed)
	})

	t.Run("when using a 'registry://' option and opening the registry key fails, it fails gracefully", func(t *testing.T) {
		dummyError := fmt.Errorf("dummy error")
		defer setRegistryOpenKeyFunc(t, &dummyRegistryKey{}, dummyError)()

		spec := &specs.Spec{}
		err := daemon.setWindowsCredentialSpec(containerFactory("registry://my-cred-spec"), spec)
		assert.ErrorContains(t, err, fmt.Sprintf("registry key %s could not be opened: %v", credentialSpecRegistryLocation, dummyError))

		assert.Check(t, spec.Windows == nil)
	})

	t.Run("when using a 'registry://' option pointing to a value that doesn't exist, it fails gracefully", func(t *testing.T) {
		valueName := "my-cred-spec"
		key := &dummyRegistryKey{
			getStringValueFunc: func(name string) (val string, valtype uint32, err error) {
				assert.Equal(t, valueName, name)
				return "", 0, registry.ErrNotExist
			},
		}
		defer setRegistryOpenKeyFunc(t, key)()

		spec := &specs.Spec{}
		err := daemon.setWindowsCredentialSpec(containerFactory("registry://"+valueName), spec)
		assert.ErrorContains(t, err, fmt.Sprintf("registry credential spec %q for container %s was not found", valueName, dummyContainerID))

		assert.Check(t, key.closed)
	})

	t.Run("when using a 'registry://' option and reading the registry value fails, it fails gracefully", func(t *testing.T) {
		dummyError := fmt.Errorf("dummy error")
		valueName := "my-cred-spec"
		key := &dummyRegistryKey{
			getStringValueFunc: func(name string) (val string, valtype uint32, err error) {
				assert.Equal(t, valueName, name)
				return "", 0, dummyError
			},
		}
		defer setRegistryOpenKeyFunc(t, key)()

		spec := &specs.Spec{}
		err := daemon.setWindowsCredentialSpec(containerFactory("registry://"+valueName), spec)
		assert.ErrorContains(t, err, fmt.Sprintf("error reading credential spec %q from registry for container %s: %v", valueName, dummyContainerID, dummyError))

		assert.Check(t, key.closed)
	})

	t.Run("happy path with a 'config://' option", func(t *testing.T) {
		configID := "my-cred-spec"

		dependencyManager := swarmagent.NewDependencyManager(nil)
		dependencyManager.Configs().Add(swarmapi.Config{
			ID: configID,
			Spec: swarmapi.ConfigSpec{
				Data: []byte(dummyCredFileContents),
			},
		})

		task := &swarmapi.Task{
			Spec: swarmapi.TaskSpec{
				Runtime: &swarmapi.TaskSpec_Container{
					Container: &swarmapi.ContainerSpec{
						Configs: []*swarmapi.ConfigReference{
							{
								ConfigID: configID,
							},
						},
					},
				},
			},
		}

		cntr := containerFactory("config://" + configID)
		cntr.DependencyStore = swarmagent.Restrict(dependencyManager, task)

		spec := &specs.Spec{}
		err := daemon.setWindowsCredentialSpec(cntr, spec)
		assert.NilError(t, err)

		if assert.Check(t, spec.Windows != nil) {
			assert.Equal(t, dummyCredFileContents, spec.Windows.CredentialSpec)
		}
	})

	t.Run("using a 'config://' option on a container not managed by swarmkit is not allowed, and results in a generic error message to hide that purely internal API", func(t *testing.T) {
		spec := &specs.Spec{}

		err := daemon.setWindowsCredentialSpec(containerFactory("config://whatever"), spec)
		assert.Equal(t, errInvalidCredentialSpecSecOpt, err)

		assert.Check(t, spec.Windows == nil)
	})

	t.Run("happy path with a 'raw://' option", func(t *testing.T) {
		spec := &specs.Spec{}

		err := daemon.setWindowsCredentialSpec(containerFactory("raw://"+dummyCredFileContents), spec)
		assert.NilError(t, err)

		if assert.Check(t, spec.Windows != nil) {
			assert.Equal(t, dummyCredFileContents, spec.Windows.CredentialSpec)
		}
	})

	t.Run("it's not case sensitive in the option names", func(t *testing.T) {
		spec := &specs.Spec{}

		err := daemon.setWindowsCredentialSpec(containerFactory("CreDENtiaLSPeC=rAw://"+dummyCredFileContents), spec)
		assert.NilError(t, err)

		if assert.Check(t, spec.Windows != nil) {
			assert.Equal(t, dummyCredFileContents, spec.Windows.CredentialSpec)
		}
	})

	t.Run("it rejects unknown options", func(t *testing.T) {
		spec := &specs.Spec{}

		err := daemon.setWindowsCredentialSpec(containerFactory("credentialspe=config://whatever"), spec)
		assert.ErrorContains(t, err, "security option not supported: credentialspe")

		assert.Check(t, spec.Windows == nil)
	})

	t.Run("it rejects unsupported credentialspec options", func(t *testing.T) {
		spec := &specs.Spec{}

		err := daemon.setWindowsCredentialSpec(containerFactory("idontexist://whatever"), spec)
		assert.Equal(t, errInvalidCredentialSpecSecOpt, err)

		assert.Check(t, spec.Windows == nil)
	})

	for _, option := range []string{"file", "registry", "config", "raw"} {
		t.Run(fmt.Sprintf("it rejects empty values for %s", option), func(t *testing.T) {
			spec := &specs.Spec{}

			err := daemon.setWindowsCredentialSpec(containerFactory(option+"://"), spec)
			assert.Equal(t, errInvalidCredentialSpecSecOpt, err)

			assert.Check(t, spec.Windows == nil)
		})
	}
}

/* Helpers below */

type dummyRegistryKey struct {
	getStringValueFunc func(name string) (val string, valtype uint32, err error)
	closed             bool
}

func (k *dummyRegistryKey) GetStringValue(name string) (val string, valtype uint32, err error) {
	return k.getStringValueFunc(name)
}

func (k *dummyRegistryKey) Close() error {
	k.closed = true
	return nil
}

// setRegistryOpenKeyFunc replaces the registryOpenKeyFunc package variable, and returns a function
// to be called to revert the change when done with testing.
func setRegistryOpenKeyFunc(t *testing.T, key *dummyRegistryKey, err ...error) func() {
	previousRegistryOpenKeyFunc := registryOpenKeyFunc

	registryOpenKeyFunc = func(baseKey registry.Key, path string, access uint32) (registryKey, error) {
		// this should always be called with exactly the same arguments
		assert.Equal(t, registry.LOCAL_MACHINE, baseKey)
		assert.Equal(t, credentialSpecRegistryLocation, path)
		assert.Equal(t, uint32(registry.QUERY_VALUE), access)

		if len(err) > 0 {
			return nil, err[0]
		}
		return key, nil
	}

	return func() {
		registryOpenKeyFunc = previousRegistryOpenKeyFunc
	}
}

func TestSetupWindowsDevices(t *testing.T) {
	t.Run("it does nothing if there are no devices", func(t *testing.T) {
		devices, err := setupWindowsDevices(nil)
		assert.NilError(t, err)
		assert.Equal(t, len(devices), 0)
	})

	t.Run("it fails if any devices are blank", func(t *testing.T) {
		devices, err := setupWindowsDevices([]containertypes.DeviceMapping{{PathOnHost: "class/anything"}, {PathOnHost: ""}})
		assert.ErrorContains(t, err, "invalid device assignment path")
		assert.ErrorContains(t, err, "''")
		assert.Equal(t, len(devices), 0)
	})

	t.Run("it fails if all devices do not contain '/' or '://'", func(t *testing.T) {
		devices, err := setupWindowsDevices([]containertypes.DeviceMapping{{PathOnHost: "anything"}, {PathOnHost: "goes"}})
		assert.ErrorContains(t, err, "invalid device assignment path")
		assert.ErrorContains(t, err, "'anything'")
		assert.Equal(t, len(devices), 0)
	})

	t.Run("it fails if any devices do not contain '/' or '://'", func(t *testing.T) {
		devices, err := setupWindowsDevices([]containertypes.DeviceMapping{{PathOnHost: "class/anything"}, {PathOnHost: "goes"}})
		assert.ErrorContains(t, err, "invalid device assignment path")
		assert.ErrorContains(t, err, "'goes'")
		assert.Equal(t, len(devices), 0)
	})

	t.Run("it fails if all '/'-separated devices do not have IDType 'class'", func(t *testing.T) {
		devices, err := setupWindowsDevices([]containertypes.DeviceMapping{{PathOnHost: "klass/anything"}, {PathOnHost: "klass/goes"}})
		assert.ErrorContains(t, err, "invalid device assignment path")
		assert.ErrorContains(t, err, "'klass/anything'")
		assert.Equal(t, len(devices), 0)
	})

	t.Run("it fails if any '/'-separated devices do not have IDType 'class'", func(t *testing.T) {
		devices, err := setupWindowsDevices([]containertypes.DeviceMapping{{PathOnHost: "class/anything"}, {PathOnHost: "klass/goes"}})
		assert.ErrorContains(t, err, "invalid device assignment path")
		assert.ErrorContains(t, err, "'klass/goes'")
		assert.Equal(t, len(devices), 0)
	})

	t.Run("it fails if any '://'-separated devices have IDType ''", func(t *testing.T) {
		devices, err := setupWindowsDevices([]containertypes.DeviceMapping{{PathOnHost: "class/anything"}, {PathOnHost: "://goes"}})
		assert.ErrorContains(t, err, "invalid device assignment path")
		assert.ErrorContains(t, err, "'://goes'")
		assert.Equal(t, len(devices), 0)
	})

	t.Run("it creates devices if all '/'-separated devices have IDType 'class'", func(t *testing.T) {
		devices, err := setupWindowsDevices([]containertypes.DeviceMapping{{PathOnHost: "class/anything"}, {PathOnHost: "class/goes"}})
		expectedDevices := []specs.WindowsDevice{{IDType: "class", ID: "anything"}, {IDType: "class", ID: "goes"}}
		assert.NilError(t, err)
		assert.Equal(t, len(devices), len(expectedDevices))
		for i := range expectedDevices {
			assert.Equal(t, devices[i], expectedDevices[i])
		}
	})

	t.Run("it creates devices if all '://'-separated devices have non-blank IDType", func(t *testing.T) {
		devices, err := setupWindowsDevices([]containertypes.DeviceMapping{{PathOnHost: "class://anything"}, {PathOnHost: "klass://goes"}})
		expectedDevices := []specs.WindowsDevice{{IDType: "class", ID: "anything"}, {IDType: "klass", ID: "goes"}}
		assert.NilError(t, err)
		assert.Equal(t, len(devices), len(expectedDevices))
		for i := range expectedDevices {
			assert.Equal(t, devices[i], expectedDevices[i])
		}
	})

	t.Run("it creates devices when given a mix of '/'-separated and '://'-separated devices", func(t *testing.T) {
		devices, err := setupWindowsDevices([]containertypes.DeviceMapping{{PathOnHost: "class/anything"}, {PathOnHost: "klass://goes"}})
		expectedDevices := []specs.WindowsDevice{{IDType: "class", ID: "anything"}, {IDType: "klass", ID: "goes"}}
		assert.NilError(t, err)
		assert.Equal(t, len(devices), len(expectedDevices))
		for i := range expectedDevices {
			assert.Equal(t, devices[i], expectedDevices[i])
		}
	})
}
