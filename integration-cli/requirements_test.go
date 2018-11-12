package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration-cli/requirement"
	"github.com/docker/docker/internal/test/registry"
	"github.com/docker/docker/pkg/parsers/kernel"
)

func ArchitectureIsNot(arch string) bool {
	return os.Getenv("DOCKER_ENGINE_GOARCH") != arch
}

func DaemonIsWindows() bool {
	return testEnv.OSType == "windows"
}

func DaemonIsWindowsAtLeastBuild(buildNumber int) func() bool {
	return func() bool {
		if testEnv.OSType != "windows" {
			return false
		}
		version := testEnv.DaemonInfo.KernelVersion
		numVersion, _ := strconv.Atoi(strings.Split(version, " ")[1])
		return numVersion >= buildNumber
	}
}

func DaemonIsLinux() bool {
	return testEnv.OSType == "linux"
}

func MinimumAPIVersion(version string) func() bool {
	return func() bool {
		return versions.GreaterThanOrEqualTo(testEnv.DaemonAPIVersion(), version)
	}
}

func OnlyDefaultNetworks() bool {
	cli, err := client.NewEnvClient()
	if err != nil {
		return false
	}
	networks, err := cli.NetworkList(context.TODO(), types.NetworkListOptions{})
	if err != nil || len(networks) > 0 {
		return false
	}
	return true
}

// Deprecated: use skip.If(t, !testEnv.DaemonInfo.ExperimentalBuild)
func ExperimentalDaemon() bool {
	return testEnv.DaemonInfo.ExperimentalBuild
}

func IsAmd64() bool {
	return os.Getenv("DOCKER_ENGINE_GOARCH") == "amd64"
}

// Certain tests here have testRequires(c, NotWindowsRS5Plus).
//
// This is being tracked internally by VSO#19599026, and externally
// through https://github.com/moby/moby/issues/38114. @jhowardmsft.
// As of 11/12/2018, there's no workaround except a reboot.
//
// Under certain circumstances, silos are not completely exiting
// causing resources to remain locked exclusively in the kernel,
// and can't be cleaned up. This is causing the RS5 CI servers to
// fill up with disk space.
//
// Hopefully this can be removed in a future Windows Update.
func NotWindowsRS5Plus() bool {
	if runtime.GOOS == "windows" {
		v, _ := kernel.GetKernelVersion()
		build, _ := strconv.Atoi(strings.Split(strings.SplitN(v.String(), " ", 3)[2][1:], ".")[0])
		if build >= 17663 {
			return false
		}
	}
	return true
}

func NotArm() bool {
	return ArchitectureIsNot("arm")
}

func NotArm64() bool {
	return ArchitectureIsNot("arm64")
}

func NotPpc64le() bool {
	return ArchitectureIsNot("ppc64le")
}

func NotS390X() bool {
	return ArchitectureIsNot("s390x")
}

func SameHostDaemon() bool {
	return testEnv.IsLocalDaemon()
}

func UnixCli() bool {
	return isUnixCli
}

func ExecSupport() bool {
	return supportsExec
}

func Network() bool {
	// Set a timeout on the GET at 15s
	var timeout = time.Duration(15 * time.Second)
	var url = "https://hub.docker.com"

	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil && strings.Contains(err.Error(), "use of closed network connection") {
		panic(fmt.Sprintf("Timeout for GET request on %s", url))
	}
	if resp != nil {
		resp.Body.Close()
	}
	return err == nil
}

func Apparmor() bool {
	if strings.HasPrefix(testEnv.DaemonInfo.OperatingSystem, "SUSE Linux Enterprise Server ") {
		return false
	}
	buf, err := ioutil.ReadFile("/sys/module/apparmor/parameters/enabled")
	return err == nil && len(buf) > 1 && buf[0] == 'Y'
}

func Devicemapper() bool {
	return strings.HasPrefix(testEnv.DaemonInfo.Driver, "devicemapper")
}

func IPv6() bool {
	cmd := exec.Command("test", "-f", "/proc/net/if_inet6")
	return cmd.Run() != nil
}

func UserNamespaceROMount() bool {
	// quick case--userns not enabled in this test run
	if os.Getenv("DOCKER_REMAP_ROOT") == "" {
		return true
	}
	if _, _, err := dockerCmdWithError("run", "--rm", "--read-only", "busybox", "date"); err != nil {
		return false
	}
	return true
}

func NotUserNamespace() bool {
	root := os.Getenv("DOCKER_REMAP_ROOT")
	return root == ""
}

func UserNamespaceInKernel() bool {
	if _, err := os.Stat("/proc/self/uid_map"); os.IsNotExist(err) {
		/*
		 * This kernel-provided file only exists if user namespaces are
		 * supported
		 */
		return false
	}

	// We need extra check on redhat based distributions
	if f, err := os.Open("/sys/module/user_namespace/parameters/enable"); err == nil {
		defer f.Close()
		b := make([]byte, 1)
		_, _ = f.Read(b)
		return string(b) != "N"
	}

	return true
}

func IsPausable() bool {
	if testEnv.OSType == "windows" {
		return testEnv.DaemonInfo.Isolation == "hyperv"
	}
	return true
}

func NotPausable() bool {
	if testEnv.OSType == "windows" {
		return testEnv.DaemonInfo.Isolation == "process"
	}
	return false
}

func IsolationIs(expectedIsolation string) bool {
	return testEnv.OSType == "windows" && string(testEnv.DaemonInfo.Isolation) == expectedIsolation
}

func IsolationIsHyperv() bool {
	return IsolationIs("hyperv")
}

func IsolationIsProcess() bool {
	return IsolationIs("process")
}

// RegistryHosting returns whether the host can host a registry (v2) or not
func RegistryHosting() bool {
	// for now registry binary is built only if we're running inside
	// container through `make test`. Figure that out by testing if
	// registry binary is in PATH.
	_, err := exec.LookPath(registry.V2binary)
	return err == nil
}

func SwarmInactive() bool {
	return testEnv.DaemonInfo.Swarm.LocalNodeState == swarm.LocalNodeStateInactive
}

func TODOBuildkit() bool {
	return os.Getenv("DOCKER_BUILDKIT") == ""
}

// testRequires checks if the environment satisfies the requirements
// for the test to run or skips the tests.
func testRequires(c requirement.SkipT, requirements ...requirement.Test) {
	requirement.Is(c, requirements...)
}
