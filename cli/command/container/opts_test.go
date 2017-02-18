package container

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/runconfig"
	"github.com/docker/go-connections/nat"
	"github.com/spf13/pflag"
)

func TestValidateAttach(t *testing.T) {
	valid := []string{
		"stdin",
		"stdout",
		"stderr",
		"STDIN",
		"STDOUT",
		"STDERR",
	}
	if _, err := validateAttach("invalid"); err == nil {
		t.Fatalf("Expected error with [valid streams are STDIN, STDOUT and STDERR], got nothing")
	}

	for _, attach := range valid {
		value, err := validateAttach(attach)
		if err != nil {
			t.Fatal(err)
		}
		if value != strings.ToLower(attach) {
			t.Fatalf("Expected [%v], got [%v]", attach, value)
		}
	}
}

func parseRun(args []string) (*container.Config, *container.HostConfig, *networktypes.NetworkingConfig, error) {
	flags := pflag.NewFlagSet("run", pflag.ContinueOnError)
	flags.SetOutput(ioutil.Discard)
	flags.Usage = nil
	copts := addFlags(flags)
	if err := flags.Parse(args); err != nil {
		return nil, nil, nil, err
	}
	return parse(flags, copts)
}

func parsetest(t *testing.T, args string) (*container.Config, *container.HostConfig, error) {
	config, hostConfig, _, err := parseRun(strings.Split(args+" ubuntu bash", " "))
	return config, hostConfig, err
}

func mustParse(t *testing.T, args string) (*container.Config, *container.HostConfig) {
	config, hostConfig, err := parsetest(t, args)
	if err != nil {
		t.Fatal(err)
	}
	return config, hostConfig
}

func TestParseRunLinks(t *testing.T) {
	if _, hostConfig := mustParse(t, "--link a:b"); len(hostConfig.Links) == 0 || hostConfig.Links[0] != "a:b" {
		t.Fatalf("Error parsing links. Expected []string{\"a:b\"}, received: %v", hostConfig.Links)
	}
	if _, hostConfig := mustParse(t, "--link a:b --link c:d"); len(hostConfig.Links) < 2 || hostConfig.Links[0] != "a:b" || hostConfig.Links[1] != "c:d" {
		t.Fatalf("Error parsing links. Expected []string{\"a:b\", \"c:d\"}, received: %v", hostConfig.Links)
	}
	if _, hostConfig := mustParse(t, ""); len(hostConfig.Links) != 0 {
		t.Fatalf("Error parsing links. No link expected, received: %v", hostConfig.Links)
	}
}

func TestParseRunAttach(t *testing.T) {
	if config, _ := mustParse(t, "-a stdin"); !config.AttachStdin || config.AttachStdout || config.AttachStderr {
		t.Fatalf("Error parsing attach flags. Expect only Stdin enabled. Received: in: %v, out: %v, err: %v", config.AttachStdin, config.AttachStdout, config.AttachStderr)
	}
	if config, _ := mustParse(t, "-a stdin -a stdout"); !config.AttachStdin || !config.AttachStdout || config.AttachStderr {
		t.Fatalf("Error parsing attach flags. Expect only Stdin and Stdout enabled. Received: in: %v, out: %v, err: %v", config.AttachStdin, config.AttachStdout, config.AttachStderr)
	}
	if config, _ := mustParse(t, "-a stdin -a stdout -a stderr"); !config.AttachStdin || !config.AttachStdout || !config.AttachStderr {
		t.Fatalf("Error parsing attach flags. Expect all attach enabled. Received: in: %v, out: %v, err: %v", config.AttachStdin, config.AttachStdout, config.AttachStderr)
	}
	if config, _ := mustParse(t, ""); config.AttachStdin || !config.AttachStdout || !config.AttachStderr {
		t.Fatalf("Error parsing attach flags. Expect Stdin disabled. Received: in: %v, out: %v, err: %v", config.AttachStdin, config.AttachStdout, config.AttachStderr)
	}
	if config, _ := mustParse(t, "-i"); !config.AttachStdin || !config.AttachStdout || !config.AttachStderr {
		t.Fatalf("Error parsing attach flags. Expect Stdin enabled. Received: in: %v, out: %v, err: %v", config.AttachStdin, config.AttachStdout, config.AttachStderr)
	}

	if _, _, err := parsetest(t, "-a"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a` should be an error but is not")
	}
	if _, _, err := parsetest(t, "-a invalid"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a invalid` should be an error but is not")
	}
	if _, _, err := parsetest(t, "-a invalid -a stdout"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a stdout -a invalid` should be an error but is not")
	}
	if _, _, err := parsetest(t, "-a stdout -a stderr -d"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a stdout -a stderr -d` should be an error but is not")
	}
	if _, _, err := parsetest(t, "-a stdin -d"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a stdin -d` should be an error but is not")
	}
	if _, _, err := parsetest(t, "-a stdout -d"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a stdout -d` should be an error but is not")
	}
	if _, _, err := parsetest(t, "-a stderr -d"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a stderr -d` should be an error but is not")
	}
	if _, _, err := parsetest(t, "-d --rm"); err == nil {
		t.Fatalf("Error parsing attach flags, `-d --rm` should be an error but is not")
	}
}

func TestParseRunVolumes(t *testing.T) {

	// A single volume
	arr, tryit := setupPlatformVolume([]string{`/tmp`}, []string{`c:\tmp`})
	if config, hostConfig := mustParse(t, tryit); hostConfig.Binds != nil {
		t.Fatalf("Error parsing volume flags, %q should not mount-bind anything. Received %v", tryit, hostConfig.Binds)
	} else if _, exists := config.Volumes[arr[0]]; !exists {
		t.Fatalf("Error parsing volume flags, %q is missing from volumes. Received %v", tryit, config.Volumes)
	}

	// Two volumes
	arr, tryit = setupPlatformVolume([]string{`/tmp`, `/var`}, []string{`c:\tmp`, `c:\var`})
	if config, hostConfig := mustParse(t, tryit); hostConfig.Binds != nil {
		t.Fatalf("Error parsing volume flags, %q should not mount-bind anything. Received %v", tryit, hostConfig.Binds)
	} else if _, exists := config.Volumes[arr[0]]; !exists {
		t.Fatalf("Error parsing volume flags, %s is missing from volumes. Received %v", arr[0], config.Volumes)
	} else if _, exists := config.Volumes[arr[1]]; !exists {
		t.Fatalf("Error parsing volume flags, %s is missing from volumes. Received %v", arr[1], config.Volumes)
	}

	// A single bind-mount
	arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp`}, []string{os.Getenv("TEMP") + `:c:\containerTmp`})
	if config, hostConfig := mustParse(t, tryit); hostConfig.Binds == nil || hostConfig.Binds[0] != arr[0] {
		t.Fatalf("Error parsing volume flags, %q should mount-bind the path before the colon into the path after the colon. Received %v %v", arr[0], hostConfig.Binds, config.Volumes)
	}

	// Two bind-mounts.
	arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp`, `/hostVar:/containerVar`}, []string{os.Getenv("ProgramData") + `:c:\ContainerPD`, os.Getenv("TEMP") + `:c:\containerTmp`})
	if _, hostConfig := mustParse(t, tryit); hostConfig.Binds == nil || compareRandomizedStrings(hostConfig.Binds[0], hostConfig.Binds[1], arr[0], arr[1]) != nil {
		t.Fatalf("Error parsing volume flags, `%s and %s` did not mount-bind correctly. Received %v", arr[0], arr[1], hostConfig.Binds)
	}

	// Two bind-mounts, first read-only, second read-write.
	// TODO Windows: The Windows version uses read-write as that's the only mode it supports. Can change this post TP4
	arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp:ro`, `/hostVar:/containerVar:rw`}, []string{os.Getenv("TEMP") + `:c:\containerTmp:rw`, os.Getenv("ProgramData") + `:c:\ContainerPD:rw`})
	if _, hostConfig := mustParse(t, tryit); hostConfig.Binds == nil || compareRandomizedStrings(hostConfig.Binds[0], hostConfig.Binds[1], arr[0], arr[1]) != nil {
		t.Fatalf("Error parsing volume flags, `%s and %s` did not mount-bind correctly. Received %v", arr[0], arr[1], hostConfig.Binds)
	}

	// Similar to previous test but with alternate modes which are only supported by Linux
	if runtime.GOOS != "windows" {
		arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp:ro,Z`, `/hostVar:/containerVar:rw,Z`}, []string{})
		if _, hostConfig := mustParse(t, tryit); hostConfig.Binds == nil || compareRandomizedStrings(hostConfig.Binds[0], hostConfig.Binds[1], arr[0], arr[1]) != nil {
			t.Fatalf("Error parsing volume flags, `%s and %s` did not mount-bind correctly. Received %v", arr[0], arr[1], hostConfig.Binds)
		}

		arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp:Z`, `/hostVar:/containerVar:z`}, []string{})
		if _, hostConfig := mustParse(t, tryit); hostConfig.Binds == nil || compareRandomizedStrings(hostConfig.Binds[0], hostConfig.Binds[1], arr[0], arr[1]) != nil {
			t.Fatalf("Error parsing volume flags, `%s and %s` did not mount-bind correctly. Received %v", arr[0], arr[1], hostConfig.Binds)
		}
	}

	// One bind mount and one volume
	arr, tryit = setupPlatformVolume([]string{`/hostTmp:/containerTmp`, `/containerVar`}, []string{os.Getenv("TEMP") + `:c:\containerTmp`, `c:\containerTmp`})
	if config, hostConfig := mustParse(t, tryit); hostConfig.Binds == nil || len(hostConfig.Binds) > 1 || hostConfig.Binds[0] != arr[0] {
		t.Fatalf("Error parsing volume flags, %s and %s should only one and only one bind mount %s. Received %s", arr[0], arr[1], arr[0], hostConfig.Binds)
	} else if _, exists := config.Volumes[arr[1]]; !exists {
		t.Fatalf("Error parsing volume flags %s and %s. %s is missing from volumes. Received %v", arr[0], arr[1], arr[1], config.Volumes)
	}

	// Root to non-c: drive letter (Windows specific)
	if runtime.GOOS == "windows" {
		arr, tryit = setupPlatformVolume([]string{}, []string{os.Getenv("SystemDrive") + `\:d:`})
		if config, hostConfig := mustParse(t, tryit); hostConfig.Binds == nil || len(hostConfig.Binds) > 1 || hostConfig.Binds[0] != arr[0] || len(config.Volumes) != 0 {
			t.Fatalf("Error parsing %s. Should have a single bind mount and no volumes", arr[0])
		}
	}

}

// setupPlatformVolume takes two arrays of volume specs - a Unix style
// spec and a Windows style spec. Depending on the platform being unit tested,
// it returns one of them, along with a volume string that would be passed
// on the docker CLI (e.g. -v /bar -v /foo).
func setupPlatformVolume(u []string, w []string) ([]string, string) {
	var a []string
	if runtime.GOOS == "windows" {
		a = w
	} else {
		a = u
	}
	s := ""
	for _, v := range a {
		s = s + "-v " + v + " "
	}
	return a, s
}

// check if (a == c && b == d) || (a == d && b == c)
// because maps are randomized
func compareRandomizedStrings(a, b, c, d string) error {
	if a == c && b == d {
		return nil
	}
	if a == d && b == c {
		return nil
	}
	return fmt.Errorf("strings don't match")
}

// Simple parse with MacAddress validation
func TestParseWithMacAddress(t *testing.T) {
	invalidMacAddress := "--mac-address=invalidMacAddress"
	validMacAddress := "--mac-address=92:d0:c6:0a:29:33"
	if _, _, _, err := parseRun([]string{invalidMacAddress, "img", "cmd"}); err != nil && err.Error() != "invalidMacAddress is not a valid mac address" {
		t.Fatalf("Expected an error with %v mac-address, got %v", invalidMacAddress, err)
	}
	if config, _ := mustParse(t, validMacAddress); config.MacAddress != "92:d0:c6:0a:29:33" {
		t.Fatalf("Expected the config to have '92:d0:c6:0a:29:33' as MacAddress, got '%v'", config.MacAddress)
	}
}

func TestParseWithMemory(t *testing.T) {
	invalidMemory := "--memory=invalid"
	validMemory := "--memory=1G"
	if _, _, _, err := parseRun([]string{invalidMemory, "img", "cmd"}); err != nil && err.Error() != "invalid size: 'invalid'" {
		t.Fatalf("Expected an error with '%v' Memory, got '%v'", invalidMemory, err)
	}
	if _, hostconfig := mustParse(t, validMemory); hostconfig.Memory != 1073741824 {
		t.Fatalf("Expected the config to have '1G' as Memory, got '%v'", hostconfig.Memory)
	}
}

func TestParseWithMemorySwap(t *testing.T) {
	invalidMemory := "--memory-swap=invalid"
	validMemory := "--memory-swap=1G"
	anotherValidMemory := "--memory-swap=-1"
	if _, _, _, err := parseRun([]string{invalidMemory, "img", "cmd"}); err == nil || err.Error() != "invalid size: 'invalid'" {
		t.Fatalf("Expected an error with '%v' MemorySwap, got '%v'", invalidMemory, err)
	}
	if _, hostconfig := mustParse(t, validMemory); hostconfig.MemorySwap != 1073741824 {
		t.Fatalf("Expected the config to have '1073741824' as MemorySwap, got '%v'", hostconfig.MemorySwap)
	}
	if _, hostconfig := mustParse(t, anotherValidMemory); hostconfig.MemorySwap != -1 {
		t.Fatalf("Expected the config to have '-1' as MemorySwap, got '%v'", hostconfig.MemorySwap)
	}
}

func TestParseHostname(t *testing.T) {
	validHostnames := map[string]string{
		"hostname":    "hostname",
		"host-name":   "host-name",
		"hostname123": "hostname123",
		"123hostname": "123hostname",
		"hostname-of-63-bytes-long-should-be-valid-and-without-any-error": "hostname-of-63-bytes-long-should-be-valid-and-without-any-error",
	}
	hostnameWithDomain := "--hostname=hostname.domainname"
	hostnameWithDomainTld := "--hostname=hostname.domainname.tld"
	for hostname, expectedHostname := range validHostnames {
		if config, _ := mustParse(t, fmt.Sprintf("--hostname=%s", hostname)); config.Hostname != expectedHostname {
			t.Fatalf("Expected the config to have 'hostname' as hostname, got '%v'", config.Hostname)
		}
	}
	if config, _ := mustParse(t, hostnameWithDomain); config.Hostname != "hostname.domainname" && config.Domainname != "" {
		t.Fatalf("Expected the config to have 'hostname' as hostname.domainname, got '%v'", config.Hostname)
	}
	if config, _ := mustParse(t, hostnameWithDomainTld); config.Hostname != "hostname.domainname.tld" && config.Domainname != "" {
		t.Fatalf("Expected the config to have 'hostname' as hostname.domainname.tld, got '%v'", config.Hostname)
	}
}

func TestParseWithExpose(t *testing.T) {
	invalids := map[string]string{
		":":                   "invalid port format for --expose: :",
		"8080:9090":           "invalid port format for --expose: 8080:9090",
		"/tcp":                "invalid range format for --expose: /tcp, error: Empty string specified for ports.",
		"/udp":                "invalid range format for --expose: /udp, error: Empty string specified for ports.",
		"NaN/tcp":             `invalid range format for --expose: NaN/tcp, error: strconv.ParseUint: parsing "NaN": invalid syntax`,
		"NaN-NaN/tcp":         `invalid range format for --expose: NaN-NaN/tcp, error: strconv.ParseUint: parsing "NaN": invalid syntax`,
		"8080-NaN/tcp":        `invalid range format for --expose: 8080-NaN/tcp, error: strconv.ParseUint: parsing "NaN": invalid syntax`,
		"1234567890-8080/tcp": `invalid range format for --expose: 1234567890-8080/tcp, error: strconv.ParseUint: parsing "1234567890": value out of range`,
	}
	valids := map[string][]nat.Port{
		"8080/tcp":      {"8080/tcp"},
		"8080/udp":      {"8080/udp"},
		"8080/ncp":      {"8080/ncp"},
		"8080-8080/udp": {"8080/udp"},
		"8080-8082/tcp": {"8080/tcp", "8081/tcp", "8082/tcp"},
	}
	for expose, expectedError := range invalids {
		if _, _, _, err := parseRun([]string{fmt.Sprintf("--expose=%v", expose), "img", "cmd"}); err == nil || err.Error() != expectedError {
			t.Fatalf("Expected error '%v' with '--expose=%v', got '%v'", expectedError, expose, err)
		}
	}
	for expose, exposedPorts := range valids {
		config, _, _, err := parseRun([]string{fmt.Sprintf("--expose=%v", expose), "img", "cmd"})
		if err != nil {
			t.Fatal(err)
		}
		if len(config.ExposedPorts) != len(exposedPorts) {
			t.Fatalf("Expected %v exposed port, got %v", len(exposedPorts), len(config.ExposedPorts))
		}
		for _, port := range exposedPorts {
			if _, ok := config.ExposedPorts[port]; !ok {
				t.Fatalf("Expected %v, got %v", exposedPorts, config.ExposedPorts)
			}
		}
	}
	// Merge with actual published port
	config, _, _, err := parseRun([]string{"--publish=80", "--expose=80-81/tcp", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.ExposedPorts) != 2 {
		t.Fatalf("Expected 2 exposed ports, got %v", config.ExposedPorts)
	}
	ports := []nat.Port{"80/tcp", "81/tcp"}
	for _, port := range ports {
		if _, ok := config.ExposedPorts[port]; !ok {
			t.Fatalf("Expected %v, got %v", ports, config.ExposedPorts)
		}
	}
}

func TestParseDevice(t *testing.T) {
	valids := map[string]container.DeviceMapping{
		"/dev/snd": {
			PathOnHost:        "/dev/snd",
			PathInContainer:   "/dev/snd",
			CgroupPermissions: "rwm",
		},
		"/dev/snd:rw": {
			PathOnHost:        "/dev/snd",
			PathInContainer:   "/dev/snd",
			CgroupPermissions: "rw",
		},
		"/dev/snd:/something": {
			PathOnHost:        "/dev/snd",
			PathInContainer:   "/something",
			CgroupPermissions: "rwm",
		},
		"/dev/snd:/something:rw": {
			PathOnHost:        "/dev/snd",
			PathInContainer:   "/something",
			CgroupPermissions: "rw",
		},
	}
	for device, deviceMapping := range valids {
		_, hostconfig, _, err := parseRun([]string{fmt.Sprintf("--device=%v", device), "img", "cmd"})
		if err != nil {
			t.Fatal(err)
		}
		if len(hostconfig.Devices) != 1 {
			t.Fatalf("Expected 1 devices, got %v", hostconfig.Devices)
		}
		if hostconfig.Devices[0] != deviceMapping {
			t.Fatalf("Expected %v, got %v", deviceMapping, hostconfig.Devices)
		}
	}

}

func TestParseModes(t *testing.T) {
	// ipc ko
	if _, _, _, err := parseRun([]string{"--ipc=container:", "img", "cmd"}); err == nil || err.Error() != "--ipc: invalid IPC mode" {
		t.Fatalf("Expected an error with message '--ipc: invalid IPC mode', got %v", err)
	}
	// ipc ok
	_, hostconfig, _, err := parseRun([]string{"--ipc=host", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if !hostconfig.IpcMode.Valid() {
		t.Fatalf("Expected a valid IpcMode, got %v", hostconfig.IpcMode)
	}
	// pid ko
	if _, _, _, err := parseRun([]string{"--pid=container:", "img", "cmd"}); err == nil || err.Error() != "--pid: invalid PID mode" {
		t.Fatalf("Expected an error with message '--pid: invalid PID mode', got %v", err)
	}
	// pid ok
	_, hostconfig, _, err = parseRun([]string{"--pid=host", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if !hostconfig.PidMode.Valid() {
		t.Fatalf("Expected a valid PidMode, got %v", hostconfig.PidMode)
	}
	// uts ko
	if _, _, _, err := parseRun([]string{"--uts=container:", "img", "cmd"}); err == nil || err.Error() != "--uts: invalid UTS mode" {
		t.Fatalf("Expected an error with message '--uts: invalid UTS mode', got %v", err)
	}
	// uts ok
	_, hostconfig, _, err = parseRun([]string{"--uts=host", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if !hostconfig.UTSMode.Valid() {
		t.Fatalf("Expected a valid UTSMode, got %v", hostconfig.UTSMode)
	}
	// shm-size ko
	expectedErr := `invalid argument "a128m" for --shm-size=a128m: invalid size: 'a128m'`
	if _, _, _, err = parseRun([]string{"--shm-size=a128m", "img", "cmd"}); err == nil || err.Error() != expectedErr {
		t.Fatalf("Expected an error with message '%v', got %v", expectedErr, err)
	}
	// shm-size ok
	_, hostconfig, _, err = parseRun([]string{"--shm-size=128m", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if hostconfig.ShmSize != 134217728 {
		t.Fatalf("Expected a valid ShmSize, got %d", hostconfig.ShmSize)
	}
}

func TestParseRestartPolicy(t *testing.T) {
	invalids := map[string]string{
		"always:2:3":         "invalid restart policy format",
		"on-failure:invalid": "maximum retry count must be an integer",
	}
	valids := map[string]container.RestartPolicy{
		"": {},
		"always": {
			Name:              "always",
			MaximumRetryCount: 0,
		},
		"on-failure:1": {
			Name:              "on-failure",
			MaximumRetryCount: 1,
		},
	}
	for restart, expectedError := range invalids {
		if _, _, _, err := parseRun([]string{fmt.Sprintf("--restart=%s", restart), "img", "cmd"}); err == nil || err.Error() != expectedError {
			t.Fatalf("Expected an error with message '%v' for %v, got %v", expectedError, restart, err)
		}
	}
	for restart, expected := range valids {
		_, hostconfig, _, err := parseRun([]string{fmt.Sprintf("--restart=%v", restart), "img", "cmd"})
		if err != nil {
			t.Fatal(err)
		}
		if hostconfig.RestartPolicy != expected {
			t.Fatalf("Expected %v, got %v", expected, hostconfig.RestartPolicy)
		}
	}
}

func TestParseRestartPolicyAutoRemove(t *testing.T) {
	expected := "Conflicting options: --restart and --rm"
	_, _, _, err := parseRun([]string{"--rm", "--restart=always", "img", "cmd"})
	if err == nil || err.Error() != expected {
		t.Fatalf("Expected error %v, but got none", expected)
	}
}

func TestParseHealth(t *testing.T) {
	checkOk := func(args ...string) *container.HealthConfig {
		config, _, _, err := parseRun(args)
		if err != nil {
			t.Fatalf("%#v: %v", args, err)
		}
		return config.Healthcheck
	}
	checkError := func(expected string, args ...string) {
		config, _, _, err := parseRun(args)
		if err == nil {
			t.Fatalf("Expected error, but got %#v", config)
		}
		if err.Error() != expected {
			t.Fatalf("Expected %#v, got %#v", expected, err)
		}
	}
	health := checkOk("--no-healthcheck", "img", "cmd")
	if health == nil || len(health.Test) != 1 || health.Test[0] != "NONE" {
		t.Fatalf("--no-healthcheck failed: %#v", health)
	}

	health = checkOk("--health-cmd=/check.sh -q", "img", "cmd")
	if len(health.Test) != 2 || health.Test[0] != "CMD-SHELL" || health.Test[1] != "/check.sh -q" {
		t.Fatalf("--health-cmd: got %#v", health.Test)
	}
	if health.Timeout != 0 {
		t.Fatalf("--health-cmd: timeout = %f", health.Timeout)
	}

	checkError("--no-healthcheck conflicts with --health-* options",
		"--no-healthcheck", "--health-cmd=/check.sh -q", "img", "cmd")

	health = checkOk("--health-timeout=2s", "--health-retries=3", "--health-interval=4.5s", "img", "cmd")
	if health.Timeout != 2*time.Second || health.Retries != 3 || health.Interval != 4500*time.Millisecond {
		t.Fatalf("--health-*: got %#v", health)
	}
}

func TestParseLoggingOpts(t *testing.T) {
	// logging opts ko
	if _, _, _, err := parseRun([]string{"--log-driver=none", "--log-opt=anything", "img", "cmd"}); err == nil || err.Error() != "invalid logging opts for driver none" {
		t.Fatalf("Expected an error with message 'invalid logging opts for driver none', got %v", err)
	}
	// logging opts ok
	_, hostconfig, _, err := parseRun([]string{"--log-driver=syslog", "--log-opt=something", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if hostconfig.LogConfig.Type != "syslog" || len(hostconfig.LogConfig.Config) != 1 {
		t.Fatalf("Expected a 'syslog' LogConfig with one config, got %v", hostconfig.RestartPolicy)
	}
}

func TestParseEnvfileVariables(t *testing.T) {
	e := "open nonexistent: no such file or directory"
	if runtime.GOOS == "windows" {
		e = "open nonexistent: The system cannot find the file specified."
	}
	// env ko
	if _, _, _, err := parseRun([]string{"--env-file=nonexistent", "img", "cmd"}); err == nil || err.Error() != e {
		t.Fatalf("Expected an error with message '%s', got %v", e, err)
	}
	// env ok
	config, _, _, err := parseRun([]string{"--env-file=testdata/valid.env", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Env) != 1 || config.Env[0] != "ENV1=value1" {
		t.Fatalf("Expected a config with [ENV1=value1], got %v", config.Env)
	}
	config, _, _, err = parseRun([]string{"--env-file=testdata/valid.env", "--env=ENV2=value2", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Env) != 2 || config.Env[0] != "ENV1=value1" || config.Env[1] != "ENV2=value2" {
		t.Fatalf("Expected a config with [ENV1=value1 ENV2=value2], got %v", config.Env)
	}
}

func TestParseEnvfileVariablesWithBOMUnicode(t *testing.T) {
	// UTF8 with BOM
	config, _, _, err := parseRun([]string{"--env-file=testdata/utf8.env", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	env := []string{"FOO=BAR", "HELLO=" + string([]byte{0xe6, 0x82, 0xa8, 0xe5, 0xa5, 0xbd}), "BAR=FOO"}
	if len(config.Env) != len(env) {
		t.Fatalf("Expected a config with %d env variables, got %v: %v", len(env), len(config.Env), config.Env)
	}
	for i, v := range env {
		if config.Env[i] != v {
			t.Fatalf("Expected a config with [%s], got %v", v, []byte(config.Env[i]))
		}
	}

	// UTF16 with BOM
	e := "contains invalid utf8 bytes at line"
	if _, _, _, err := parseRun([]string{"--env-file=testdata/utf16.env", "img", "cmd"}); err == nil || !strings.Contains(err.Error(), e) {
		t.Fatalf("Expected an error with message '%s', got %v", e, err)
	}
	// UTF16BE with BOM
	if _, _, _, err := parseRun([]string{"--env-file=testdata/utf16be.env", "img", "cmd"}); err == nil || !strings.Contains(err.Error(), e) {
		t.Fatalf("Expected an error with message '%s', got %v", e, err)
	}
}

func TestParseLabelfileVariables(t *testing.T) {
	e := "open nonexistent: no such file or directory"
	if runtime.GOOS == "windows" {
		e = "open nonexistent: The system cannot find the file specified."
	}
	// label ko
	if _, _, _, err := parseRun([]string{"--label-file=nonexistent", "img", "cmd"}); err == nil || err.Error() != e {
		t.Fatalf("Expected an error with message '%s', got %v", e, err)
	}
	// label ok
	config, _, _, err := parseRun([]string{"--label-file=testdata/valid.label", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Labels) != 1 || config.Labels["LABEL1"] != "value1" {
		t.Fatalf("Expected a config with [LABEL1:value1], got %v", config.Labels)
	}
	config, _, _, err = parseRun([]string{"--label-file=testdata/valid.label", "--label=LABEL2=value2", "img", "cmd"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Labels) != 2 || config.Labels["LABEL1"] != "value1" || config.Labels["LABEL2"] != "value2" {
		t.Fatalf("Expected a config with [LABEL1:value1 LABEL2:value2], got %v", config.Labels)
	}
}

func TestParseEntryPoint(t *testing.T) {
	config, _, _, err := parseRun([]string{"--entrypoint=anything", "cmd", "img"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Entrypoint) != 1 && config.Entrypoint[0] != "anything" {
		t.Fatalf("Expected entrypoint 'anything', got %v", config.Entrypoint)
	}
}

// This tests the cases for binds which are generated through
// DecodeContainerConfig rather than Parse()
func TestDecodeContainerConfigVolumes(t *testing.T) {

	// Root to root
	bindsOrVols, _ := setupPlatformVolume([]string{`/:/`}, []string{os.Getenv("SystemDrive") + `\:c:\`})
	if _, _, err := callDecodeContainerConfig(nil, bindsOrVols); err == nil {
		t.Fatalf("binds %v should have failed", bindsOrVols)
	}
	if _, _, err := callDecodeContainerConfig(bindsOrVols, nil); err == nil {
		t.Fatalf("volume %v should have failed", bindsOrVols)
	}

	// No destination path
	bindsOrVols, _ = setupPlatformVolume([]string{`/tmp:`}, []string{os.Getenv("TEMP") + `\:`})
	if _, _, err := callDecodeContainerConfig(nil, bindsOrVols); err == nil {
		t.Fatalf("binds %v should have failed", bindsOrVols)
	}
	if _, _, err := callDecodeContainerConfig(bindsOrVols, nil); err == nil {
		t.Fatalf("volume %v should have failed", bindsOrVols)
	}

	//	// No destination path or mode
	bindsOrVols, _ = setupPlatformVolume([]string{`/tmp::`}, []string{os.Getenv("TEMP") + `\::`})
	if _, _, err := callDecodeContainerConfig(nil, bindsOrVols); err == nil {
		t.Fatalf("binds %v should have failed", bindsOrVols)
	}
	if _, _, err := callDecodeContainerConfig(bindsOrVols, nil); err == nil {
		t.Fatalf("volume %v should have failed", bindsOrVols)
	}

	// A whole lot of nothing
	bindsOrVols = []string{`:`}
	if _, _, err := callDecodeContainerConfig(nil, bindsOrVols); err == nil {
		t.Fatalf("binds %v should have failed", bindsOrVols)
	}
	if _, _, err := callDecodeContainerConfig(bindsOrVols, nil); err == nil {
		t.Fatalf("volume %v should have failed", bindsOrVols)
	}

	// A whole lot of nothing with no mode
	bindsOrVols = []string{`::`}
	if _, _, err := callDecodeContainerConfig(nil, bindsOrVols); err == nil {
		t.Fatalf("binds %v should have failed", bindsOrVols)
	}
	if _, _, err := callDecodeContainerConfig(bindsOrVols, nil); err == nil {
		t.Fatalf("volume %v should have failed", bindsOrVols)
	}

	// Too much including an invalid mode
	wTmp := os.Getenv("TEMP")
	bindsOrVols, _ = setupPlatformVolume([]string{`/tmp:/tmp:/tmp:/tmp`}, []string{wTmp + ":" + wTmp + ":" + wTmp + ":" + wTmp})
	if _, _, err := callDecodeContainerConfig(nil, bindsOrVols); err == nil {
		t.Fatalf("binds %v should have failed", bindsOrVols)
	}
	if _, _, err := callDecodeContainerConfig(bindsOrVols, nil); err == nil {
		t.Fatalf("volume %v should have failed", bindsOrVols)
	}

	// Windows specific error tests
	if runtime.GOOS == "windows" {
		// Volume which does not include a drive letter
		bindsOrVols = []string{`\tmp`}
		if _, _, err := callDecodeContainerConfig(nil, bindsOrVols); err == nil {
			t.Fatalf("binds %v should have failed", bindsOrVols)
		}
		if _, _, err := callDecodeContainerConfig(bindsOrVols, nil); err == nil {
			t.Fatalf("volume %v should have failed", bindsOrVols)
		}

		// Root to C-Drive
		bindsOrVols = []string{os.Getenv("SystemDrive") + `\:c:`}
		if _, _, err := callDecodeContainerConfig(nil, bindsOrVols); err == nil {
			t.Fatalf("binds %v should have failed", bindsOrVols)
		}
		if _, _, err := callDecodeContainerConfig(bindsOrVols, nil); err == nil {
			t.Fatalf("volume %v should have failed", bindsOrVols)
		}

		// Container path that does not include a drive letter
		bindsOrVols = []string{`c:\windows:\somewhere`}
		if _, _, err := callDecodeContainerConfig(nil, bindsOrVols); err == nil {
			t.Fatalf("binds %v should have failed", bindsOrVols)
		}
		if _, _, err := callDecodeContainerConfig(bindsOrVols, nil); err == nil {
			t.Fatalf("volume %v should have failed", bindsOrVols)
		}
	}

	// Linux-specific error tests
	if runtime.GOOS != "windows" {
		// Just root
		bindsOrVols = []string{`/`}
		if _, _, err := callDecodeContainerConfig(nil, bindsOrVols); err == nil {
			t.Fatalf("binds %v should have failed", bindsOrVols)
		}
		if _, _, err := callDecodeContainerConfig(bindsOrVols, nil); err == nil {
			t.Fatalf("volume %v should have failed", bindsOrVols)
		}

		// A single volume that looks like a bind mount passed in Volumes.
		// This should be handled as a bind mount, not a volume.
		vols := []string{`/foo:/bar`}
		if config, hostConfig, err := callDecodeContainerConfig(vols, nil); err != nil {
			t.Fatal("Volume /foo:/bar should have succeeded as a volume name")
		} else if hostConfig.Binds != nil {
			t.Fatalf("Error parsing volume flags, /foo:/bar should not mount-bind anything. Received %v", hostConfig.Binds)
		} else if _, exists := config.Volumes[vols[0]]; !exists {
			t.Fatalf("Error parsing volume flags, /foo:/bar is missing from volumes. Received %v", config.Volumes)
		}

	}
}

// callDecodeContainerConfig is a utility function used by TestDecodeContainerConfigVolumes
// to call DecodeContainerConfig. It effectively does what a client would
// do when calling the daemon by constructing a JSON stream of a
// ContainerConfigWrapper which is populated by the set of volume specs
// passed into it. It returns a config and a hostconfig which can be
// validated to ensure DecodeContainerConfig has manipulated the structures
// correctly.
func callDecodeContainerConfig(volumes []string, binds []string) (*container.Config, *container.HostConfig, error) {
	var (
		b   []byte
		err error
		c   *container.Config
		h   *container.HostConfig
	)
	w := runconfig.ContainerConfigWrapper{
		Config: &container.Config{
			Volumes: map[string]struct{}{},
		},
		HostConfig: &container.HostConfig{
			NetworkMode: "none",
			Binds:       binds,
		},
	}
	for _, v := range volumes {
		w.Config.Volumes[v] = struct{}{}
	}
	if b, err = json.Marshal(w); err != nil {
		return nil, nil, fmt.Errorf("Error on marshal %s", err.Error())
	}
	c, h, _, err = runconfig.DecodeContainerConfig(bytes.NewReader(b))
	if err != nil {
		return nil, nil, fmt.Errorf("Error parsing %s: %v", string(b), err)
	}
	if c == nil || h == nil {
		return nil, nil, fmt.Errorf("Empty config or hostconfig")
	}

	return c, h, err
}

func TestVolumeSplitN(t *testing.T) {
	for _, x := range []struct {
		input    string
		n        int
		expected []string
	}{
		{`C:\foo:d:`, -1, []string{`C:\foo`, `d:`}},
		{`:C:\foo:d:`, -1, nil},
		{`/foo:/bar:ro`, 3, []string{`/foo`, `/bar`, `ro`}},
		{`/foo:/bar:ro`, 2, []string{`/foo`, `/bar:ro`}},
		{`C:\foo\:/foo`, -1, []string{`C:\foo\`, `/foo`}},

		{`d:\`, -1, []string{`d:\`}},
		{`d:`, -1, []string{`d:`}},
		{`d:\path`, -1, []string{`d:\path`}},
		{`d:\path with space`, -1, []string{`d:\path with space`}},
		{`d:\pathandmode:rw`, -1, []string{`d:\pathandmode`, `rw`}},
		{`c:\:d:\`, -1, []string{`c:\`, `d:\`}},
		{`c:\windows\:d:`, -1, []string{`c:\windows\`, `d:`}},
		{`c:\windows:d:\s p a c e`, -1, []string{`c:\windows`, `d:\s p a c e`}},
		{`c:\windows:d:\s p a c e:RW`, -1, []string{`c:\windows`, `d:\s p a c e`, `RW`}},
		{`c:\program files:d:\s p a c e i n h o s t d i r`, -1, []string{`c:\program files`, `d:\s p a c e i n h o s t d i r`}},
		{`0123456789name:d:`, -1, []string{`0123456789name`, `d:`}},
		{`MiXeDcAsEnAmE:d:`, -1, []string{`MiXeDcAsEnAmE`, `d:`}},
		{`name:D:`, -1, []string{`name`, `D:`}},
		{`name:D::rW`, -1, []string{`name`, `D:`, `rW`}},
		{`name:D::RW`, -1, []string{`name`, `D:`, `RW`}},
		{`c:/:d:/forward/slashes/are/good/too`, -1, []string{`c:/`, `d:/forward/slashes/are/good/too`}},
		{`c:\Windows`, -1, []string{`c:\Windows`}},
		{`c:\Program Files (x86)`, -1, []string{`c:\Program Files (x86)`}},

		{``, -1, nil},
		{`.`, -1, []string{`.`}},
		{`..\`, -1, []string{`..\`}},
		{`c:\:..\`, -1, []string{`c:\`, `..\`}},
		{`c:\:d:\:xyzzy`, -1, []string{`c:\`, `d:\`, `xyzzy`}},

		// Cover directories with one-character name
		{`/tmp/x/y:/foo/x/y`, -1, []string{`/tmp/x/y`, `/foo/x/y`}},
	} {
		res := volumeSplitN(x.input, x.n)
		if len(res) < len(x.expected) {
			t.Fatalf("input: %v, expected: %v, got: %v", x.input, x.expected, res)
		}
		for i, e := range res {
			if e != x.expected[i] {
				t.Fatalf("input: %v, expected: %v, got: %v", x.input, x.expected, res)
			}
		}
	}
}

func TestValidateDevice(t *testing.T) {
	valid := []string{
		"/home",
		"/home:/home",
		"/home:/something/else",
		"/with space",
		"/home:/with space",
		"relative:/absolute-path",
		"hostPath:/containerPath:r",
		"/hostPath:/containerPath:rw",
		"/hostPath:/containerPath:mrw",
	}
	invalid := map[string]string{
		"":        "bad format for path: ",
		"./":      "./ is not an absolute path",
		"../":     "../ is not an absolute path",
		"/:../":   "../ is not an absolute path",
		"/:path":  "path is not an absolute path",
		":":       "bad format for path: :",
		"/tmp:":   " is not an absolute path",
		":test":   "bad format for path: :test",
		":/test":  "bad format for path: :/test",
		"tmp:":    " is not an absolute path",
		":test:":  "bad format for path: :test:",
		"::":      "bad format for path: ::",
		":::":     "bad format for path: :::",
		"/tmp:::": "bad format for path: /tmp:::",
		":/tmp::": "bad format for path: :/tmp::",
		"path:ro": "ro is not an absolute path",
		"path:rr": "rr is not an absolute path",
		"a:/b:ro": "bad mode specified: ro",
		"a:/b:rr": "bad mode specified: rr",
	}

	for _, path := range valid {
		if _, err := validateDevice(path); err != nil {
			t.Fatalf("ValidateDevice(`%q`) should succeed: error %q", path, err)
		}
	}

	for path, expectedError := range invalid {
		if _, err := validateDevice(path); err == nil {
			t.Fatalf("ValidateDevice(`%q`) should have failed validation", path)
		} else {
			if err.Error() != expectedError {
				t.Fatalf("ValidateDevice(`%q`) error should contain %q, got %q", path, expectedError, err.Error())
			}
		}
	}
}
