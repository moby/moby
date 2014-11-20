package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
)

func binarySearchCommand() *exec.Cmd {
	if runtime.GOOS == "windows" {
		// Windows where.exe is included since Windows Server 2003. It accepts
		// wildcards, which we use here to match the development builds binary
		// names (such as docker-$VERSION.exe).
		return exec.Command("where.exe", "docker*.exe")
	}
	return exec.Command("which", "docker")
}

func init() {
	if dockerBin := os.Getenv("DOCKER_BINARY"); dockerBin != "" {
		dockerBinary = dockerBin
	} else {
		whichCmd := binarySearchCommand()
		out, _, err := runCommandWithOutput(whichCmd)
		if err == nil {
			dockerBinary = stripTrailingCharacters(out)
		} else {
			fmt.Printf("ERROR: couldn't resolve full path to the Docker binary (%v)", err)
			os.Exit(1)
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
