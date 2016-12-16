package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/integration-cli/requirement"
	"github.com/go-check/check"
)

func PlatformIs(platform string) bool {
	return daemonPlatform == platform
}

func ArchitectureIs(arch string) bool {
	return os.Getenv("DOCKER_ENGINE_GOARCH") == arch
}

func ArchitectureIsNot(arch string) bool {
	return os.Getenv("DOCKER_ENGINE_GOARCH") != arch
}

func StorageDriverIs(storageDriver string) bool {
	return strings.HasPrefix(daemonStorageDriver, storageDriver)
}

func StorageDriverIsNot(storageDriver string) bool {
	return !strings.HasPrefix(daemonStorageDriver, storageDriver)
}

func DaemonIsWindows() bool {
	return PlatformIs("windows")
}

func DaemonIsLinux() bool {
	return PlatformIs("linux")
}

func ExperimentalDaemon() bool {
	return experimentalDaemon
}

func NotExperimentalDaemon() bool {
	return !experimentalDaemon
}

func IsAmd64() bool {
	return ArchitectureIs("amd64")
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
	return isLocalDaemon
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
	buf, err := ioutil.ReadFile("/sys/module/apparmor/parameters/enabled")
	return err == nil && len(buf) > 1 && buf[0] == 'Y'
}

func RegistryHosting() bool {
	// for now registry binary is built only if we're running inside
	// container through `make test`. Figure that out by testing if
	// registry binary is in PATH.
	_, err := exec.LookPath(v2binary)
	return err == nil
}

func NotaryHosting() bool {
	// for now notary binary is built only if we're running inside
	// container through `make test`. Figure that out by testing if
	// notary-server binary is in PATH.
	_, err := exec.LookPath(notaryServerBinary)
	return err == nil
}

func NotaryServerHosting() bool {
	// for now notary-server binary is built only if we're running inside
	// container through `make test`. Figure that out by testing if
	// notary-server binary is in PATH.
	_, err := exec.LookPath(notaryServerBinary)
	return err == nil
}

func NotOverlay() bool {
	return StorageDriverIsNot("overlay")
}

func Devicemapper() bool {
	return StorageDriverIs("devicemapper")
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
	if daemonPlatform == "windows" {
		return isolation == "hyperv"
	}
	return true
}

func NotPausable() bool {
	if daemonPlatform == "windows" {
		return isolation == "process"
	}
	return false
}

func IsolationIs(expectedIsolation string) bool {
	return daemonPlatform == "windows" && string(isolation) == expectedIsolation
}

func IsolationIsHyperv() bool {
	return IsolationIs("hyperv")
}

func IsolationIsProcess() bool {
	return IsolationIs("process")
}

// testRequires checks if the environment satisfies the requirements
// for the test to run or skips the tests.
func testRequires(c *check.C, requirements ...requirement.Test) {
	requirement.Is(c, requirements...)
}
