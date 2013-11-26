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
