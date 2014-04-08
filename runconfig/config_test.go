package runconfig

import (
	"github.com/dotcloud/docker/nat"
	"strings"
	"testing"
)

func parse(t *testing.T, args string) (*Config, *HostConfig, error) {
	config, hostConfig, _, err := Parse(strings.Split(args+" ubuntu bash", " "), nil)
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
	if _, hostConfig := mustParse(t, "--link a:b"); len(hostConfig.Links) == 0 || hostConfig.Links[0] != "a:b" {
		t.Fatalf("Error parsing links. Expected []string{\"a:b\"}, received: %v", hostConfig.Links)
	}
	if _, hostConfig := mustParse(t, "--link a:b --link c:d"); len(hostConfig.Links) < 2 || hostConfig.Links[0] != "a:b" || hostConfig.Links[1] != "c:d" {
		t.Fatalf("Error parsing links. Expected []string{\"a:b\", \"c:d\"}, received: %v", hostConfig.Links)
	}
	if _, hostConfig := mustParse(t, ""); len(hostConfig.Links) != 0 {
		t.Fatalf("Error parsing links. No link expected, received: %v", hostConfig.Links)
	}

	if _, _, err := parse(t, "--link a"); err == nil {
		t.Fatalf("Error parsing links. `--link a` should be an error but is not")
	}
	if _, _, err := parse(t, "--link"); err == nil {
		t.Fatalf("Error parsing links. `--link` should be an error but is not")
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
	if _, _, err := parse(t, "-d --rm"); err == nil {
		t.Fatalf("Error parsing attach flags, `-d --rm` should be an error but is not")
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

	if _, _, err := parse(t, "-v /"); err == nil {
		t.Fatalf("Expected error, but got none")
	}

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

func TestCompare(t *testing.T) {
	volumes1 := make(map[string]struct{})
	volumes1["/test1"] = struct{}{}
	config1 := Config{
		PortSpecs: []string{"1111:1111", "2222:2222"},
		Env:       []string{"VAR1=1", "VAR2=2"},
		Volumes:   volumes1,
	}
	config3 := Config{
		PortSpecs: []string{"0000:0000", "2222:2222"},
		Env:       []string{"VAR1=1", "VAR2=2"},
		Volumes:   volumes1,
	}
	volumes2 := make(map[string]struct{})
	volumes2["/test2"] = struct{}{}
	config5 := Config{
		PortSpecs: []string{"0000:0000", "2222:2222"},
		Env:       []string{"VAR1=1", "VAR2=2"},
		Volumes:   volumes2,
	}
	if Compare(&config1, &config3) {
		t.Fatalf("Compare should return false, PortSpecs are different")
	}
	if Compare(&config1, &config5) {
		t.Fatalf("Compare should return false, Volumes are different")
	}
	if !Compare(&config1, &config1) {
		t.Fatalf("Compare should return true")
	}
}

func TestMerge(t *testing.T) {
	volumesImage := make(map[string]struct{})
	volumesImage["/test1"] = struct{}{}
	volumesImage["/test2"] = struct{}{}
	configImage := &Config{
		PortSpecs: []string{"1111:1111", "2222:2222"},
		Env:       []string{"VAR1=1", "VAR2=2"},
		Volumes:   volumesImage,
	}

	volumesUser := make(map[string]struct{})
	volumesUser["/test3"] = struct{}{}
	configUser := &Config{
		PortSpecs: []string{"3333:2222", "3333:3333"},
		Env:       []string{"VAR2=3", "VAR3=3"},
		Volumes:   volumesUser,
	}

	if err := Merge(configUser, configImage); err != nil {
		t.Error(err)
	}

	if len(configUser.ExposedPorts) != 3 {
		t.Fatalf("Expected 3 ExposedPorts, 1111, 2222 and 3333, found %d", len(configUser.ExposedPorts))
	}
	for portSpecs := range configUser.ExposedPorts {
		if portSpecs.Port() != "1111" && portSpecs.Port() != "2222" && portSpecs.Port() != "3333" {
			t.Fatalf("Expected 1111 or 2222 or 3333, found %s", portSpecs)
		}
	}
	if len(configUser.Env) != 3 {
		t.Fatalf("Expected 3 env var, VAR1=1, VAR2=3 and VAR3=3, found %d", len(configUser.Env))
	}
	for _, env := range configUser.Env {
		if env != "VAR1=1" && env != "VAR2=3" && env != "VAR3=3" {
			t.Fatalf("Expected VAR1=1 or VAR2=3 or VAR3=3, found %s", env)
		}
	}

	if len(configUser.Volumes) != 3 {
		t.Fatalf("Expected 3 volumes, /test1, /test2 and /test3, found %d", len(configUser.Volumes))
	}
	for v := range configUser.Volumes {
		if v != "/test1" && v != "/test2" && v != "/test3" {
			t.Fatalf("Expected /test1 or /test2 or /test3, found %s", v)
		}
	}

	ports, _, err := nat.ParsePortSpecs([]string{"0000"})
	if err != nil {
		t.Error(err)
	}
	configImage2 := &Config{
		ExposedPorts: ports,
	}

	if err := Merge(configUser, configImage2); err != nil {
		t.Error(err)
	}

	if len(configUser.ExposedPorts) != 4 {
		t.Fatalf("Expected 4 ExposedPorts, 0000, 1111, 2222 and 3333, found %d", len(configUser.ExposedPorts))
	}
	for portSpecs := range configUser.ExposedPorts {
		if portSpecs.Port() != "0000" && portSpecs.Port() != "1111" && portSpecs.Port() != "2222" && portSpecs.Port() != "3333" {
			t.Fatalf("Expected 0000 or 1111 or 2222 or 3333, found %s", portSpecs)
		}
	}

}
