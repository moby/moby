package cache

import (
	"runtime"
	"testing"

	"github.com/moby/moby/api/types/container"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCompare(t *testing.T) {
	ports1 := container.PortSet{
		"1111/tcp": struct{}{},
		"2222/tcp": struct{}{},
	}
	ports2 := container.PortSet{
		"3333/tcp": struct{}{},
		"4444/tcp": struct{}{},
	}
	ports3 := container.PortSet{
		"1111/tcp": struct{}{},
		"2222/tcp": struct{}{},
		"5555/tcp": struct{}{},
	}
	volumes1 := map[string]struct{}{
		"/test1": {},
	}
	volumes2 := map[string]struct{}{
		"/test2": {},
	}
	volumes3 := map[string]struct{}{
		"/test1": {},
		"/test3": {},
	}
	envs1 := []string{"ENV1=value1", "ENV2=value2"}
	envs2 := []string{"ENV1=value1", "ENV3=value3"}
	entrypoint1 := []string{"/bin/sh", "-c"}
	entrypoint2 := []string{"/bin/sh", "-d"}
	entrypoint3 := []string{"/bin/sh", "-c", "echo"}
	cmd1 := []string{"/bin/sh", "-c"}
	cmd2 := []string{"/bin/sh", "-d"}
	cmd3 := []string{"/bin/sh", "-c", "echo"}
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
