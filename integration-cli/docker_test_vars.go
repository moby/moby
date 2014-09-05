package main

import (
	"fmt"
	"os"
	"os/exec"
)

// the docker binary to use
var dockerBinary = "docker"

// the private registry image to use for tests involving the registry
var registryImageName = "registry"

// the private registry to use for tests
var privateRegistryURL = "127.0.0.1:5000"

var workingDirectory string

func init() {
	if dockerBin := os.Getenv("DOCKER_BINARY"); dockerBin != "" {
		dockerBinary = dockerBin
	} else {
		whichCmd := exec.Command("which", "docker")
		out, _, err := runCommandWithOutput(whichCmd)
		if err == nil {
			dockerBinary = stripTrailingCharacters(out)
		} else {
			fmt.Printf("ERROR: couldn't resolve full path to the Docker binary")
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
