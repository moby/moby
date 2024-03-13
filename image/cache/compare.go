package cache // import "github.com/docker/docker/image/cache"

import (
	"strings"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/docker/api/types/container"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func comparePlatform(builderPlatform, imagePlatform ocispec.Platform) bool {
	// On Windows, only check the Major and Minor versions.
	// The Build and Revision compatibility depends on whether `process` or
	// `hyperv` isolation used.
	//
	// Fixes https://github.com/moby/moby/issues/47307
	if builderPlatform.OS == "windows" && imagePlatform.OS == builderPlatform.OS {
		// OSVersion format is:
		// Major.Minor.Build.Revision
		builderParts := strings.Split(builderPlatform.OSVersion, ".")
		imageParts := strings.Split(imagePlatform.OSVersion, ".")

		if len(builderParts) >= 3 && len(imageParts) >= 3 {
			// Keep only Major & Minor.
			builderParts[0] = imageParts[0]
			builderParts[1] = imageParts[1]
			imagePlatform.OSVersion = strings.Join(builderParts, ".")
		}
	}

	return platforms.Only(builderPlatform).Match(imagePlatform)
}

// compare two Config struct. Do not container-specific fields:
// - Image
// - Hostname
// - Domainname
// - MacAddress
func compare(a, b *container.Config) bool {
	if a == nil || b == nil {
		return false
	}

	if len(a.Env) != len(b.Env) {
		return false
	}
	if len(a.Cmd) != len(b.Cmd) {
		return false
	}
	if len(a.Entrypoint) != len(b.Entrypoint) {
		return false
	}
	if len(a.Shell) != len(b.Shell) {
		return false
	}
	if len(a.ExposedPorts) != len(b.ExposedPorts) {
		return false
	}
	if len(a.Volumes) != len(b.Volumes) {
		return false
	}
	if len(a.Labels) != len(b.Labels) {
		return false
	}
	if len(a.OnBuild) != len(b.OnBuild) {
		return false
	}

	for i := 0; i < len(a.Env); i++ {
		if a.Env[i] != b.Env[i] {
			return false
		}
	}
	for i := 0; i < len(a.OnBuild); i++ {
		if a.OnBuild[i] != b.OnBuild[i] {
			return false
		}
	}
	for i := 0; i < len(a.Cmd); i++ {
		if a.Cmd[i] != b.Cmd[i] {
			return false
		}
	}
	for i := 0; i < len(a.Entrypoint); i++ {
		if a.Entrypoint[i] != b.Entrypoint[i] {
			return false
		}
	}
	for i := 0; i < len(a.Shell); i++ {
		if a.Shell[i] != b.Shell[i] {
			return false
		}
	}
	for k := range a.ExposedPorts {
		if _, exists := b.ExposedPorts[k]; !exists {
			return false
		}
	}
	for key := range a.Volumes {
		if _, exists := b.Volumes[key]; !exists {
			return false
		}
	}
	for k, v := range a.Labels {
		if v != b.Labels[k] {
			return false
		}
	}

	if a.AttachStdin != b.AttachStdin {
		return false
	}
	if a.AttachStdout != b.AttachStdout {
		return false
	}
	if a.AttachStderr != b.AttachStderr {
		return false
	}
	if a.NetworkDisabled != b.NetworkDisabled {
		return false
	}
	if a.Tty != b.Tty {
		return false
	}
	if a.OpenStdin != b.OpenStdin {
		return false
	}
	if a.StdinOnce != b.StdinOnce {
		return false
	}
	if a.ArgsEscaped != b.ArgsEscaped {
		return false
	}
	if a.User != b.User {
		return false
	}
	if a.WorkingDir != b.WorkingDir {
		return false
	}
	if a.StopSignal != b.StopSignal {
		return false
	}

	if (a.StopTimeout == nil) != (b.StopTimeout == nil) {
		return false
	}
	if a.StopTimeout != nil && b.StopTimeout != nil {
		if *a.StopTimeout != *b.StopTimeout {
			return false
		}
	}
	if (a.Healthcheck == nil) != (b.Healthcheck == nil) {
		return false
	}
	if a.Healthcheck != nil && b.Healthcheck != nil {
		if a.Healthcheck.Interval != b.Healthcheck.Interval {
			return false
		}
		if a.Healthcheck.StartInterval != b.Healthcheck.StartInterval {
			return false
		}
		if a.Healthcheck.StartPeriod != b.Healthcheck.StartPeriod {
			return false
		}
		if a.Healthcheck.Timeout != b.Healthcheck.Timeout {
			return false
		}
		if a.Healthcheck.Retries != b.Healthcheck.Retries {
			return false
		}
		if len(a.Healthcheck.Test) != len(b.Healthcheck.Test) {
			return false
		}
		for i := 0; i < len(a.Healthcheck.Test); i++ {
			if a.Healthcheck.Test[i] != b.Healthcheck.Test[i] {
				return false
			}
		}
	}

	return true
}
