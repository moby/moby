package main

import (
	"fmt"
	"os"
	"os/exec"
)

var (
	// the docker binary to use
	dockerBinary = "docker"

	// the private registry image to use for tests involving the registry
	registryImageName = "registry"

	// the private registry to use for tests
	privateRegistryURL = "127.0.0.1:5000"

	dockerBasePath       = "/var/lib/docker"
	volumesConfigPath    = dockerBasePath + "/volumes"
	containerStoragePath = dockerBasePath + "/containers"

	runtimePath    = "/var/run/docker"
	execDriverPath = runtimePath + "/execdriver/native"

	workingDirectory string

	// isLocalDaemon is true if the daemon under test is on the same
	// host as the CLI.
	isLocalDaemon bool

	// daemonPlatform is held globally so that tests can make intelligent
	// decisions on how to configure themselves according to the platform
	// of the daemon. This is initialised in docker_utils by sending
	// a version call to the daemon and examining the response header.
	daemonPlatform string
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
	if registryImage := os.Getenv("REGISTRY_IMAGE"); registryImage != "" {
		registryImageName = registryImage
	}
	if registry := os.Getenv("REGISTRY_URL"); registry != "" {
		privateRegistryURL = registry
	}
	workingDirectory, _ = os.Getwd()

	// Deterministically working out the environment in which CI is running
	// to evaluate whether the daemon is local or remote is not possible through
	// a build tag.
	//
	// For example Windows CI under Jenkins test the 64-bit
	// Windows binary build with the daemon build tag, but calls a remote
	// Linux daemon.
	//
	// We can't just say if Windows then assume the daemon is local as at
	// some point, we will be testing the Windows CLI against a Windows daemon.
	//
	// Similarly, it will be perfectly valid to also run CLI tests from
	// a Linux CLI (built with the daemon tag) against a Windows daemon.
	if len(os.Getenv("DOCKER_REMOTE_DAEMON")) > 0 {
		fmt.Println("INFO: Testing against a remote daemon")
		isLocalDaemon = false
	} else {
		fmt.Println("INFO: Testing against a local daemon")
		isLocalDaemon = true
	}

}
