package main

import (
	"os"
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
	}
	if registryImage := os.Getenv("REGISTRY_IMAGE"); registryImage != "" {
		registryImageName = registryImage
	}
	if registry := os.Getenv("REGISTRY_URL"); registry != "" {
		privateRegistryURL = registry
	}
	workingDirectory, _ = os.Getwd()
}
