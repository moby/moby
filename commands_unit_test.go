package docker

import (
	"strings"
	"testing"
)

func parse(t *testing.T, args string) (*Config, *HostConfig, error) {
	config, hostConfig, _, err := ParseRun(strings.Split(args+" ubuntu bash", " "), nil)
	return config, hostConfig, err
}

func mustParse(t *testing.T, args string) (*Config, *HostConfig) {
	config, hostConfig, err := parse(t, args)
	if err != nil {
		t.Fatal(err)
	}
	return config, hostConfig
}

func TestParseRunLinks(t *testing.T) {
	if _, hostConfig := mustParse(t, "-link a:b"); len(hostConfig.Links) == 0 || hostConfig.Links[0] != "a:b" {
		t.Fatalf("Error parsing links. Expected []string{\"a:b\"}, received: %v", hostConfig.Links)
	}
	if _, hostConfig := mustParse(t, "-link a:b -link c:d"); len(hostConfig.Links) < 2 || hostConfig.Links[0] != "a:b" || hostConfig.Links[1] != "c:d" {
		t.Fatalf("Error parsing links. Expected []string{\"a:b\", \"c:d\"}, received: %v", hostConfig.Links)
	}
	if _, hostConfig := mustParse(t, ""); len(hostConfig.Links) != 0 {
		t.Fatalf("Error parsing links. No link expected, received: %v", hostConfig.Links)
	}

	if _, _, err := parse(t, "-link a"); err == nil {
		t.Fatalf("Error parsing links. `-link a` should be an error but is not")
	}
	if _, _, err := parse(t, "-link"); err == nil {
		t.Fatalf("Error parsing links. `-link` should be an error but is not")
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

	if _, _, err := parse(t, "-a"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a` should be an error but is not")
	}
	if _, _, err := parse(t, "-a invalid"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a invalid` should be an error but is not")
	}
	if _, _, err := parse(t, "-a invalid -a stdout"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a stdout -a invalid` should be an error but is not")
	}
	if _, _, err := parse(t, "-a stdout -a stderr -d"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a stdout -a stderr -d` should be an error but is not")
	}
	if _, _, err := parse(t, "-a stdin -d"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a stdin -d` should be an error but is not")
	}
	if _, _, err := parse(t, "-a stdout -d"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a stdout -d` should be an error but is not")
	}
	if _, _, err := parse(t, "-a stderr -d"); err == nil {
		t.Fatalf("Error parsing attach flags, `-a stderr -d` should be an error but is not")
	}
	if _, _, err := parse(t, "-d -rm"); err == nil {
		t.Fatalf("Error parsing attach flags, `-d -rm` should be an error but is not")
	}
}

func TestParseRunVolumes(t *testing.T) {
	if config, hostConfig := mustParse(t, "-v /tmp"); hostConfig.Binds != nil {
		t.Fatalf("Error parsing volume flags, `-v /tmp` should not mount-bind anything. Received %v", hostConfig.Binds)
	} else if _, exists := config.Volumes["/tmp"]; !exists {
		t.Fatalf("Error parsing volume flags, `-v /tmp` is missing from volumes. Received %v", config.Volumes)
	}

	if config, hostConfig := mustParse(t, "-v /tmp -v /var"); hostConfig.Binds != nil {
		t.Fatalf("Error parsing volume flags, `-v /tmp -v /var` should not mount-bind anything. Received %v", hostConfig.Binds)
	} else if _, exists := config.Volumes["/tmp"]; !exists {
		t.Fatalf("Error parsing volume flags, `-v /tmp` is missing from volumes. Recevied %v", config.Volumes)
	} else if _, exists := config.Volumes["/var"]; !exists {
		t.Fatalf("Error parsing volume flags, `-v /var` is missing from volumes. Received %v", config.Volumes)
	}

	if config, hostConfig := mustParse(t, "-v /hostTmp:/containerTmp"); hostConfig.Binds == nil || hostConfig.Binds[0] != "/hostTmp:/containerTmp" {
		t.Fatalf("Error parsing volume flags, `-v /hostTmp:/containerTmp` should mount-bind /hostTmp into /containeTmp. Received %v", hostConfig.Binds)
	} else if _, exists := config.Volumes["/containerTmp"]; !exists {
		t.Fatalf("Error parsing volume flags, `-v /tmp` is missing from volumes. Received %v", config.Volumes)
	}

	if config, hostConfig := mustParse(t, "-v /hostTmp:/containerTmp -v /hostVar:/containerVar"); hostConfig.Binds == nil || hostConfig.Binds[0] != "/hostTmp:/containerTmp" || hostConfig.Binds[1] != "/hostVar:/containerVar" {
		t.Fatalf("Error parsing volume flags, `-v /hostTmp:/containerTmp -v /hostVar:/containerVar` should mount-bind /hostTmp into /containeTmp and /hostVar into /hostContainer. Received %v", hostConfig.Binds)
	} else if _, exists := config.Volumes["/containerTmp"]; !exists {
		t.Fatalf("Error parsing volume flags, `-v /containerTmp` is missing from volumes. Received %v", config.Volumes)
	} else if _, exists := config.Volumes["/containerVar"]; !exists {
		t.Fatalf("Error parsing volume flags, `-v /containerVar` is missing from volumes. Received %v", config.Volumes)
	}

	if config, hostConfig := mustParse(t, "-v /hostTmp:/containerTmp:ro -v /hostVar:/containerVar:rw"); hostConfig.Binds == nil || hostConfig.Binds[0] != "/hostTmp:/containerTmp:ro" || hostConfig.Binds[1] != "/hostVar:/containerVar:rw" {
		t.Fatalf("Error parsing volume flags, `-v /hostTmp:/containerTmp:ro -v /hostVar:/containerVar:rw` should mount-bind /hostTmp into /containeTmp and /hostVar into /hostContainer. Received %v", hostConfig.Binds)
	} else if _, exists := config.Volumes["/containerTmp"]; !exists {
		t.Fatalf("Error parsing volume flags, `-v /containerTmp` is missing from volumes. Received %v", config.Volumes)
	} else if _, exists := config.Volumes["/containerVar"]; !exists {
		t.Fatalf("Error parsing volume flags, `-v /containerVar` is missing from volumes. Received %v", config.Volumes)
	}

	if config, hostConfig := mustParse(t, "-v /hostTmp:/containerTmp -v /containerVar"); hostConfig.Binds == nil || len(hostConfig.Binds) > 1 || hostConfig.Binds[0] != "/hostTmp:/containerTmp" {
		t.Fatalf("Error parsing volume flags, `-v /hostTmp:/containerTmp -v /containerVar` should mount-bind only /hostTmp into /containeTmp. Received %v", hostConfig.Binds)
	} else if _, exists := config.Volumes["/containerTmp"]; !exists {
		t.Fatalf("Error parsing volume flags, `-v /containerTmp` is missing from volumes. Received %v", config.Volumes)
	} else if _, exists := config.Volumes["/containerVar"]; !exists {
		t.Fatalf("Error parsing volume flags, `-v /containerVar` is missing from volumes. Received %v", config.Volumes)
	}

	if config, hostConfig := mustParse(t, ""); hostConfig.Binds != nil {
		t.Fatalf("Error parsing volume flags, without volume, nothing should be mount-binded. Received %v", hostConfig.Binds)
	} else if len(config.Volumes) != 0 {
		t.Fatalf("Error parsing volume flags, without volume, no volume should be present. Received %v", config.Volumes)
	}

	mustParse(t, "-v /")

	if _, _, err := parse(t, "-v /:/"); err == nil {
		t.Fatalf("Error parsing volume flags, `-v /:/` should fail but didn't")
	}
	if _, _, err := parse(t, "-v"); err == nil {
		t.Fatalf("Error parsing volume flags, `-v` should fail but didn't")
	}
	if _, _, err := parse(t, "-v /tmp:"); err == nil {
		t.Fatalf("Error parsing volume flags, `-v /tmp:` should fail but didn't")
	}
	if _, _, err := parse(t, "-v /tmp:ro"); err == nil {
		t.Fatalf("Error parsing volume flags, `-v /tmp:ro` should fail but didn't")
	}
	if _, _, err := parse(t, "-v /tmp::"); err == nil {
		t.Fatalf("Error parsing volume flags, `-v /tmp::` should fail but didn't")
	}
	if _, _, err := parse(t, "-v :"); err == nil {
		t.Fatalf("Error parsing volume flags, `-v :` should fail but didn't")
	}
	if _, _, err := parse(t, "-v ::"); err == nil {
		t.Fatalf("Error parsing volume flags, `-v ::` should fail but didn't")
	}
	if _, _, err := parse(t, "-v /tmp:/tmp:/tmp:/tmp"); err == nil {
		t.Fatalf("Error parsing volume flags, `-v /tmp:/tmp:/tmp:/tmp` should fail but didn't")
	}
}
