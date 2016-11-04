package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-check/check"
)

type testCondition func() bool

type testRequirement struct {
	Condition   testCondition
	SkipMessage string
}

// List test requirements
var (
	DaemonIsWindows = testRequirement{
		func() bool { return daemonPlatform == "windows" },
		"Test requires a Windows daemon",
	}
	DaemonIsLinux = testRequirement{
		func() bool { return daemonPlatform == "linux" },
		"Test requires a Linux daemon",
	}
	ExperimentalDaemon = testRequirement{
		func() bool { return experimentalDaemon },
		"Test requires an experimental daemon",
	}
	NotExperimentalDaemon = testRequirement{
		func() bool { return !experimentalDaemon },
		"Test requires a non experimental daemon",
	}
	IsAmd64 = testRequirement{
		func() bool { return os.Getenv("DOCKER_ENGINE_GOARCH") == "amd64" },
		"Test requires a daemon running on amd64",
	}
	NotArm = testRequirement{
		func() bool { return os.Getenv("DOCKER_ENGINE_GOARCH") != "arm" },
		"Test requires a daemon not running on ARM",
	}
	NotArm64 = testRequirement{
		func() bool { return os.Getenv("DOCKER_ENGINE_GOARCH") != "arm64" },
		"Test requires a daemon not running on arm64",
	}
	NotPpc64le = testRequirement{
		func() bool { return os.Getenv("DOCKER_ENGINE_GOARCH") != "ppc64le" },
		"Test requires a daemon not running on ppc64le",
	}
	NotS390X = testRequirement{
		func() bool { return os.Getenv("DOCKER_ENGINE_GOARCH") != "s390x" },
		"Test requires a daemon not running on s390x",
	}
	SameHostDaemon = testRequirement{
		func() bool { return isLocalDaemon },
		"Test requires docker daemon to run on the same machine as CLI",
	}
	UnixCli = testRequirement{
		func() bool { return isUnixCli },
		"Test requires posix utilities or functionality to run.",
	}
	ExecSupport = testRequirement{
		func() bool { return supportsExec },
		"Test requires 'docker exec' capabilities on the tested daemon.",
	}
	Network = testRequirement{
		func() bool {
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
		},
		"Test requires network availability, environment variable set to none to run in a non-network enabled mode.",
	}
	Apparmor = testRequirement{
		func() bool {
			buf, err := ioutil.ReadFile("/sys/module/apparmor/parameters/enabled")
			return err == nil && len(buf) > 1 && buf[0] == 'Y'
		},
		"Test requires apparmor is enabled.",
	}
	RegistryHosting = testRequirement{
		func() bool {
			// for now registry binary is built only if we're running inside
			// container through `make test`. Figure that out by testing if
			// registry binary is in PATH.
			_, err := exec.LookPath(v2binary)
			return err == nil
		},
		fmt.Sprintf("Test requires an environment that can host %s in the same host", v2binary),
	}
	NotaryHosting = testRequirement{
		func() bool {
			// for now notary binary is built only if we're running inside
			// container through `make test`. Figure that out by testing if
			// notary-server binary is in PATH.
			_, err := exec.LookPath(notaryServerBinary)
			return err == nil
		},
		fmt.Sprintf("Test requires an environment that can host %s in the same host", notaryServerBinary),
	}
	NotaryServerHosting = testRequirement{
		func() bool {
			// for now notary-server binary is built only if we're running inside
			// container through `make test`. Figure that out by testing if
			// notary-server binary is in PATH.
			_, err := exec.LookPath(notaryServerBinary)
			return err == nil
		},
		fmt.Sprintf("Test requires an environment that can host %s in the same host", notaryServerBinary),
	}
	NotOverlay = testRequirement{
		func() bool {
			return !strings.HasPrefix(daemonStorageDriver, "overlay")
		},
		"Test requires underlying root filesystem not be backed by overlay.",
	}

	Devicemapper = testRequirement{
		func() bool {
			return strings.HasPrefix(daemonStorageDriver, "devicemapper")
		},
		"Test requires underlying root filesystem to be backed by devicemapper.",
	}

	IPv6 = testRequirement{
		func() bool {
			cmd := exec.Command("test", "-f", "/proc/net/if_inet6")

			if err := cmd.Run(); err != nil {
				return true
			}
			return false
		},
		"Test requires support for IPv6",
	}
	UserNamespaceROMount = testRequirement{
		func() bool {
			// quick case--userns not enabled in this test run
			if os.Getenv("DOCKER_REMAP_ROOT") == "" {
				return true
			}
			if _, _, err := dockerCmdWithError("run", "--rm", "--read-only", "busybox", "date"); err != nil {
				return false
			}
			return true
		},
		"Test cannot be run if user namespaces enabled but readonly mounts fail on this kernel.",
	}
	UserNamespaceInKernel = testRequirement{
		func() bool {
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
				if string(b) == "N" {
					return false
				}
				return true
			}

			return true
		},
		"Kernel must have user namespaces configured and enabled.",
	}
	NotUserNamespace = testRequirement{
		func() bool {
			root := os.Getenv("DOCKER_REMAP_ROOT")
			if root != "" {
				return false
			}
			return true
		},
		"Test cannot be run when remapping root",
	}
	IsPausable = testRequirement{
		func() bool {
			if daemonPlatform == "windows" {
				return isolation == "hyperv"
			}
			return true
		},
		"Test requires containers are pausable.",
	}
	NotPausable = testRequirement{
		func() bool {
			if daemonPlatform == "windows" {
				return isolation == "process"
			}
			return false
		},
		"Test requires containers are not pausable.",
	}
	IsolationIsHyperv = testRequirement{
		func() bool {
			return daemonPlatform == "windows" && isolation == "hyperv"
		},
		"Test requires a Windows daemon running default isolation mode of hyperv.",
	}
	IsolationIsProcess = testRequirement{
		func() bool {
			return daemonPlatform == "windows" && isolation == "process"
		},
		"Test requires a Windows daemon running default isolation mode of process.",
	}
)

// testRequires checks if the environment satisfies the requirements
// for the test to run or skips the tests.
func testRequires(c *check.C, requirements ...testRequirement) {
	for _, r := range requirements {
		if !r.Condition() {
			c.Skip(r.SkipMessage)
		}
	}
}
