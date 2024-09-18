package cache // import "github.com/docker/docker/image/cache"

import (
	"runtime"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// Just to make life easier
func newPortNoError(proto, port string) nat.Port {
	p, _ := nat.NewPort(proto, port)
	return p
}

func TestCompare(t *testing.T) {
	ports1 := make(nat.PortSet)
	ports1[newPortNoError("tcp", "1111")] = struct{}{}
	ports1[newPortNoError("tcp", "2222")] = struct{}{}
	ports2 := make(nat.PortSet)
	ports2[newPortNoError("tcp", "3333")] = struct{}{}
	ports2[newPortNoError("tcp", "4444")] = struct{}{}
	ports3 := make(nat.PortSet)
	ports3[newPortNoError("tcp", "1111")] = struct{}{}
	ports3[newPortNoError("tcp", "2222")] = struct{}{}
	ports3[newPortNoError("tcp", "5555")] = struct{}{}
	volumes1 := make(map[string]struct{})
	volumes1["/test1"] = struct{}{}
	volumes2 := make(map[string]struct{})
	volumes2["/test2"] = struct{}{}
	volumes3 := make(map[string]struct{})
	volumes3["/test1"] = struct{}{}
	volumes3["/test3"] = struct{}{}
	envs1 := []string{"ENV1=value1", "ENV2=value2"}
	envs2 := []string{"ENV1=value1", "ENV3=value3"}
	entrypoint1 := strslice.StrSlice{"/bin/sh", "-c"}
	entrypoint2 := strslice.StrSlice{"/bin/sh", "-d"}
	entrypoint3 := strslice.StrSlice{"/bin/sh", "-c", "echo"}
	cmd1 := strslice.StrSlice{"/bin/sh", "-c"}
	cmd2 := strslice.StrSlice{"/bin/sh", "-d"}
	cmd3 := strslice.StrSlice{"/bin/sh", "-c", "echo"}
	labels1 := map[string]string{"LABEL1": "value1", "LABEL2": "value2"}
	labels2 := map[string]string{"LABEL1": "value1", "LABEL2": "value3"}
	labels3 := map[string]string{"LABEL1": "value1", "LABEL2": "value2", "LABEL3": "value3"}

	sameConfigs := map[*container.Config]*container.Config{
		// Empty config
		{}: {},
		// Does not compare hostname, domainname & image
		{
			Hostname:   "host1",
			Domainname: "domain1",
			Image:      "image1",
			User:       "user",
		}: {
			Hostname:   "host2",
			Domainname: "domain2",
			Image:      "image2",
			User:       "user",
		},
		// only OpenStdin
		{OpenStdin: false}: {OpenStdin: false},
		// only env
		{Env: envs1}: {Env: envs1},
		// only cmd
		{Cmd: cmd1}: {Cmd: cmd1},
		// only labels
		{Labels: labels1}: {Labels: labels1},
		// only exposedPorts
		{ExposedPorts: ports1}: {ExposedPorts: ports1},
		// only entrypoints
		{Entrypoint: entrypoint1}: {Entrypoint: entrypoint1},
		// only volumes
		{Volumes: volumes1}: {Volumes: volumes1},
	}
	differentConfigs := map[*container.Config]*container.Config{
		nil: nil,
		{
			Hostname:   "host1",
			Domainname: "domain1",
			Image:      "image1",
			User:       "user1",
		}: {
			Hostname:   "host1",
			Domainname: "domain1",
			Image:      "image1",
			User:       "user2",
		},
		// only OpenStdin
		{OpenStdin: false}: {OpenStdin: true},
		{OpenStdin: true}:  {OpenStdin: false},
		// only env
		{Env: envs1}: {Env: envs2},
		// only cmd
		{Cmd: cmd1}: {Cmd: cmd2},
		// not the same number of parts
		{Cmd: cmd1}: {Cmd: cmd3},
		// only labels
		{Labels: labels1}: {Labels: labels2},
		// not the same number of labels
		{Labels: labels1}: {Labels: labels3},
		// only exposedPorts
		{ExposedPorts: ports1}: {ExposedPorts: ports2},
		// not the same number of ports
		{ExposedPorts: ports1}: {ExposedPorts: ports3},
		// only entrypoints
		{Entrypoint: entrypoint1}: {Entrypoint: entrypoint2},
		// not the same number of parts
		{Entrypoint: entrypoint1}: {Entrypoint: entrypoint3},
		// only volumes
		{Volumes: volumes1}: {Volumes: volumes2},
		// not the same number of labels
		{Volumes: volumes1}: {Volumes: volumes3},
	}
	for config1, config2 := range sameConfigs {
		if !compare(config1, config2) {
			t.Fatalf("Compare should be true for [%v] and [%v]", config1, config2)
		}
	}
	for config1, config2 := range differentConfigs {
		if compare(config1, config2) {
			t.Fatalf("Compare should be false for [%v] and [%v]", config1, config2)
		}
	}
}

func TestPlatformCompare(t *testing.T) {
	for _, tc := range []struct {
		name     string
		builder  ocispec.Platform
		image    ocispec.Platform
		expected bool
	}{
		{
			name:     "same os and arch",
			builder:  ocispec.Platform{Architecture: "amd64", OS: runtime.GOOS},
			image:    ocispec.Platform{Architecture: "amd64", OS: runtime.GOOS},
			expected: true,
		},
		{
			name:     "same os different arch",
			builder:  ocispec.Platform{Architecture: "amd64", OS: runtime.GOOS},
			image:    ocispec.Platform{Architecture: "arm64", OS: runtime.GOOS},
			expected: false,
		},
		{
			name:     "same os smaller host variant",
			builder:  ocispec.Platform{Variant: "v7", Architecture: "arm", OS: runtime.GOOS},
			image:    ocispec.Platform{Variant: "v8", Architecture: "arm", OS: runtime.GOOS},
			expected: false,
		},
		{
			name:     "same os higher host variant",
			builder:  ocispec.Platform{Variant: "v8", Architecture: "arm", OS: runtime.GOOS},
			image:    ocispec.Platform{Variant: "v7", Architecture: "arm", OS: runtime.GOOS},
			expected: true,
		},
		{
			// Test for https://github.com/moby/moby/issues/47307
			name:     "different build and revision",
			builder:  ocispec.Platform{Architecture: "amd64", OS: "windows", OSVersion: "10.0.22621"},
			image:    ocispec.Platform{Architecture: "amd64", OS: "windows", OSVersion: "10.0.17763.5329"},
			expected: true,
		},
		{
			name:     "different revision",
			builder:  ocispec.Platform{Architecture: "amd64", OS: "windows", OSVersion: "10.0.17763.1234"},
			image:    ocispec.Platform{Architecture: "amd64", OS: "windows", OSVersion: "10.0.17763.5329"},
			expected: true,
		},
		{
			name:     "different major",
			builder:  ocispec.Platform{Architecture: "amd64", OS: "windows", OSVersion: "11.0.17763.5329"},
			image:    ocispec.Platform{Architecture: "amd64", OS: "windows", OSVersion: "10.0.17763.5329"},
			expected: false,
		},
		{
			name:     "different minor same osver",
			builder:  ocispec.Platform{Architecture: "amd64", OS: "windows", OSVersion: "10.0.17763.5329"},
			image:    ocispec.Platform{Architecture: "amd64", OS: "windows", OSVersion: "10.1.17763.5329"},
			expected: false,
		},
		{
			name:     "different arch same osver",
			builder:  ocispec.Platform{Architecture: "arm64", OS: "windows", OSVersion: "10.0.17763.5329"},
			image:    ocispec.Platform{Architecture: "amd64", OS: "windows", OSVersion: "10.0.17763.5329"},
			expected: false,
		},
	} {
		tc := tc
		// OSVersion comparison is only performed by containerd platform
		// matcher if built on Windows.
		if (tc.image.OSVersion != "" || tc.builder.OSVersion != "") && runtime.GOOS != "windows" {
			continue
		}

		t.Run(tc.name, func(t *testing.T) {
			assert.Check(t, is.Equal(comparePlatform(tc.builder, tc.image), tc.expected))
		})
	}
}
