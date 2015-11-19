package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/docker/docker/pkg/reexec"
)

var (
	// the docker binary to use
	dockerBinary = "docker"

	// the private registry image to use for tests involving the registry
	registryImageName = "registry"

	// the private registry to use for tests
	privateRegistryURL = "127.0.0.1:5000"

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

	// windowsDaemonKV is used on Windows to distinguish between different
	// versions. This is necessary to enable certain tests based on whether
	// the platform supports it. For example, Windows Server 2016 TP3 does
	// not support volumes, but TP4 does.
	windowsDaemonKV int

	// daemonDefaultImage is the name of the default image to use when running
	// tests. This is platform dependent.
	daemonDefaultImage string

	// For a local daemon on Linux, these values will be used for testing
	// user namespace support as the standard graph path(s) will be
	// appended with the root remapped uid.gid prefix
	dockerBasePath       string
	volumesConfigPath    string
	containerStoragePath string
)

const (
	// WindowsBaseImage is the name of the base image for Windows testing
	WindowsBaseImage = "windowsservercore"

	// DefaultImage is the name of the base image for the majority of tests that
	// are run across suites
	DefaultImage = "busybox"
)

func init() {
	reexec.Init()
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
		isLocalDaemon = false
	} else {
		isLocalDaemon = true
	}

	// This is only used for a tests with local daemon true (Linux-only today)
	// default is "/var/lib/docker", but we'll try and ask the
	// /info endpoint for the specific root dir
	dockerBasePath = "/var/lib/docker"
	type Info struct {
		DockerRootDir string
	}
	var i Info
	status, b, err := sockRequest("GET", "/info", nil)
	if err == nil && status == 200 {
		if err = json.Unmarshal(b, &i); err == nil {
			dockerBasePath = i.DockerRootDir
		}
	}
	volumesConfigPath = dockerBasePath + "/volumes"
	containerStoragePath = dockerBasePath + "/containers"
}
