package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	// the docker binary to use
	dockerBinary = "docker"

	// the private registry image to use for tests involving the registry
	registryImageName = "registry"

	// the private registry to use for tests
	privateRegistryURL = "127.0.0.1:5000"

	dockerBasePath       = "/var/lib/docker"
	execDriverPath       = dockerBasePath + "/execdriver/native"
	volumesConfigPath    = dockerBasePath + "/volumes"
	volumesStoragePath   = dockerBasePath + "/vfs/dir"
	containerStoragePath = dockerBasePath + "/containers"

	workingDirectory string

	daemonGoVersion string
)

func init() {
	if dockerBin := os.Getenv("DOCKER_BINARY"); dockerBin != "" {
		dockerBinary = dockerBin
	}
	var err error
	dockerBinary, err = exec.LookPath(dockerBinary)
	if err != nil {
		fmt.Printf("ERROR: couldn't resolve full path to the Docker binary (%v)", err)
		os.Exit(1)
	}
	if out, err := exec.Command(dockerBinary, "version").CombinedOutput(); err != nil {
		fmt.Printf("ERROR: couldn't execute docker binary at %q", dockerBinary)
		os.Exit(1)
	} else {
		daemonGoVersion = regexp.MustCompile(`(?m)^Go version \(server\): (.*)$`).FindStringSubmatch(string(out))[1]
		parts := strings.Split(daemonGoVersion, ".")
		m, err := strconv.Atoi(parts[1])
		if err != nil {
			fmt.Printf("ERROR: fail to parse go version: %s", daemonGoVersion)
			os.Exit(1)
		}
		if m < 2 {
			fmt.Printf("ERROR: go versions below 1.2 is not supported, daemon compiled with %s", daemonGoVersion)
		}
	}
	if registryImage := os.Getenv("REGISTRY_IMAGE"); registryImage != "" {
		registryImageName = registryImage
	}
	if registry := os.Getenv("REGISTRY_URL"); registry != "" {
		privateRegistryURL = registry
	}
	workingDirectory, _ = os.Getwd()
}
