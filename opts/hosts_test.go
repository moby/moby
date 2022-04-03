package opts // import "github.com/docker/docker/opts"

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseHost(t *testing.T) {
	invalid := map[string]string{
		"something with spaces": `parse "tcp://something with spaces": invalid character " " in host name`,
		"://":                   `Invalid bind address format: ://`,
		"unknown://":            `Invalid bind address format: unknown://`,
		"tcp://:port":           `parse "tcp://:port": invalid port ":port" after host`,
		"tcp://invalid:port":    `parse "tcp://invalid:port": invalid port ":port" after host`,
		"tcp://:5555/":          `invalid bind address (:5555/): should not contain a path element`,
		"tcp://:5555/p":         `invalid bind address (:5555/p): should not contain a path element`,
		"tcp://0.0.0.0:5555/":   `invalid bind address (0.0.0.0:5555/): should not contain a path element`,
		"tcp://0.0.0.0:5555/p":  `invalid bind address (0.0.0.0:5555/p): should not contain a path element`,
		"tcp://[::1]:/":         `invalid bind address ([::1]:/): should not contain a path element`,
		"tcp://[::1]:5555/":     `invalid bind address ([::1]:5555/): should not contain a path element`,
		"tcp://[::1]:5555/p":    `invalid bind address ([::1]:5555/p): should not contain a path element`,
		" tcp://:5555/path ":    `invalid bind address (:5555/path): should not contain a path element`,
	}

	valid := map[string]string{
		"":                         DefaultHost,
		" ":                        DefaultHost,
		"  ":                       DefaultHost,
		"fd://":                    "fd://",
		"fd://something":           "fd://something",
		"tcp://host:":              fmt.Sprintf("tcp://host:%d", DefaultHTTPPort),
		"tcp://":                   DefaultTCPHost,
		"tcp://:":                  DefaultTCPHost,
		"tcp://:5555":              fmt.Sprintf("tcp://%s:5555", DefaultHTTPHost),
		"tcp://[::1]:":             fmt.Sprintf(`tcp://[::1]:%d`, DefaultHTTPPort),
		"tcp://[::1]:5555":         `tcp://[::1]:5555`,
		"tcp://0.0.0.0:5555":       "tcp://0.0.0.0:5555",
		"tcp://192.168:5555":       "tcp://192.168:5555",
		"tcp://192.168.0.1:5555":   "tcp://192.168.0.1:5555",
		"tcp://0.0.0.0:1234567890": "tcp://0.0.0.0:1234567890", // yeah it's valid :P
		"tcp://docker.com:5555":    "tcp://docker.com:5555",
		"unix://":                  "unix://" + DefaultUnixSocket,
		"unix://path/to/socket":    "unix://path/to/socket",
		"npipe://":                 "npipe://" + DefaultNamedPipe,
		"npipe:////./pipe/foo":     "npipe:////./pipe/foo",
	}

	for value, expectedError := range invalid {
		t.Run(value, func(t *testing.T) {
			_, err := ParseHost(false, false, value)
			if err == nil || err.Error() != expectedError {
				t.Errorf(`expected error "%s", got "%v"`, expectedError, err)
			}
		})
	}

	for value, expected := range valid {
		t.Run(value, func(t *testing.T) {
			actual, err := ParseHost(false, false, value)
			if err != nil {
				t.Errorf(`unexpected error: "%v"`, err)
			}
			if actual != expected {
				t.Errorf(`expected "%s", got "%s""`, expected, actual)
			}
		})
	}
}

func TestParseDockerDaemonHost(t *testing.T) {
	invalids := map[string]string{
		"tcp:a.b.c.d":                   `parse "tcp://tcp:a.b.c.d": invalid port ":a.b.c.d" after host`,
		"tcp:a.b.c.d/path":              `parse "tcp://tcp:a.b.c.d/path": invalid port ":a.b.c.d" after host`,
		"udp://127.0.0.1":               "Invalid bind address format: udp://127.0.0.1",
		"udp://127.0.0.1:5555":          "Invalid bind address format: udp://127.0.0.1:5555",
		"tcp://unix:///run/docker.sock": "Invalid proto, expected tcp: unix:///run/docker.sock",
		" tcp://:5555/path ":            "Invalid bind address format:  tcp://:5555/path ",
		"":                              "Invalid bind address format: ",
		":5555/path":                    "invalid bind address (:5555/path): should not contain a path element",
		"0.0.0.1:5555/path":             "invalid bind address (0.0.0.1:5555/path): should not contain a path element",
		"[::1]:5555/path":               "invalid bind address ([::1]:5555/path): should not contain a path element",
		"[0:0:0:0:0:0:0:1]:5555/path":   "invalid bind address ([0:0:0:0:0:0:0:1]:5555/path): should not contain a path element",
		"tcp://:5555/path":              "invalid bind address (:5555/path): should not contain a path element",
		"localhost:5555/path":           "invalid bind address (localhost:5555/path): should not contain a path element",
	}
	valids := map[string]string{
		":":                       DefaultTCPHost,
		":5555":                   fmt.Sprintf("tcp://%s:5555", DefaultHTTPHost),
		"0.0.0.1:":                fmt.Sprintf("tcp://0.0.0.1:%d", DefaultHTTPPort),
		"0.0.0.1:5555":            "tcp://0.0.0.1:5555",
		"[::1]:":                  fmt.Sprintf("tcp://[::1]:%d", DefaultHTTPPort),
		"[::1]:5555":              "tcp://[::1]:5555",
		"[0:0:0:0:0:0:0:1]:":      fmt.Sprintf("tcp://[0:0:0:0:0:0:0:1]:%d", DefaultHTTPPort),
		"[0:0:0:0:0:0:0:1]:5555":  "tcp://[0:0:0:0:0:0:0:1]:5555",
		"localhost":               fmt.Sprintf("tcp://localhost:%d", DefaultHTTPPort),
		"localhost:":              fmt.Sprintf("tcp://localhost:%d", DefaultHTTPPort),
		"localhost:5555":          "tcp://localhost:5555",
		"fd://":                   "fd://",
		"fd://something":          "fd://something",
		"npipe://":                "npipe://" + DefaultNamedPipe,
		"npipe:////./pipe/foo":    "npipe:////./pipe/foo",
		"tcp://":                  DefaultTCPHost,
		"tcp://:5555":             fmt.Sprintf("tcp://%s:5555", DefaultHTTPHost),
		"tcp://[::1]:":            fmt.Sprintf("tcp://[::1]:%d", DefaultHTTPPort),
		"tcp://[::1]:5555":        "tcp://[::1]:5555",
		"unix://":                 "unix://" + DefaultUnixSocket,
		"unix:///run/docker.sock": "unix:///run/docker.sock",
	}
	for invalidAddr, expectedError := range invalids {
		t.Run(invalidAddr, func(t *testing.T) {
			addr, err := parseDaemonHost(invalidAddr)
			if err == nil || err.Error() != expectedError {
				t.Errorf(`expected error "%s", got "%v"`, expectedError, err)
			}
			if addr != "" {
				t.Errorf(`expected addr to be empty, got "%s""`, addr)
			}
		})
	}
	for validAddr, expectedAddr := range valids {
		t.Run(validAddr, func(t *testing.T) {
			addr, err := parseDaemonHost(validAddr)
			if err != nil {
				t.Errorf(`unexpected error: "%v"`, err)
			}
			if addr != expectedAddr {
				t.Errorf(`expected "%s", got "%s""`, expectedAddr, addr)
			}
		})
	}
}

func TestParseTCP(t *testing.T) {
	var (
		defaultHTTPHost = "tcp://127.0.0.1:8888"
	)
	invalids := map[string]string{
		"tcp:a.b.c.d":                 `parse "tcp://tcp:a.b.c.d": invalid port ":a.b.c.d" after host`,
		"tcp:a.b.c.d/path":            `parse "tcp://tcp:a.b.c.d/path": invalid port ":a.b.c.d" after host`,
		"udp://127.0.0.1":             "Invalid proto, expected tcp: udp://127.0.0.1",
		"udp://127.0.0.1:5555":        "Invalid proto, expected tcp: udp://127.0.0.1:5555",
		":5555/path":                  "invalid bind address (:5555/path): should not contain a path element",
		"0.0.0.1:5555/path":           "invalid bind address (0.0.0.1:5555/path): should not contain a path element",
		"[::1]:5555/path":             "invalid bind address ([::1]:5555/path): should not contain a path element",
		"[0:0:0:0:0:0:0:1]:5555/path": "invalid bind address ([0:0:0:0:0:0:0:1]:5555/path): should not contain a path element",
		"tcp://:5555/path":            "invalid bind address (tcp://:5555/path): should not contain a path element",
		"localhost:5555/path":         "invalid bind address (localhost:5555/path): should not contain a path element",
	}
	valids := map[string]string{
		"":                       defaultHTTPHost,
		"0.0.0.1":                "tcp://0.0.0.1:8888",
		"0.0.0.1:":               "tcp://0.0.0.1:8888",
		"0.0.0.1:5555":           "tcp://0.0.0.1:5555",
		":":                      "tcp://127.0.0.1:8888",
		":5555":                  "tcp://127.0.0.1:5555",
		"::1":                    "tcp://[::1]:8888",
		"[::1]:":                 "tcp://[::1]:8888",
		"[::1]:5555":             "tcp://[::1]:5555",
		"[0:0:0:0:0:0:0:1]:":     "tcp://[0:0:0:0:0:0:0:1]:8888",
		"[0:0:0:0:0:0:0:1]:5555": "tcp://[0:0:0:0:0:0:0:1]:5555",
		"localhost":              "tcp://localhost:8888",
		"localhost:":             "tcp://localhost:8888",
		"localhost:5555":         "tcp://localhost:5555",
		"tcp://":                 defaultHTTPHost,
		"tcp://:":                defaultHTTPHost,
		"tcp://:5555":            "tcp://127.0.0.1:5555",
	}
	for invalidAddr, expectedError := range invalids {
		t.Run(invalidAddr, func(t *testing.T) {
			addr, err := ParseTCPAddr(invalidAddr, defaultHTTPHost)
			if err == nil || err.Error() != expectedError {
				t.Errorf(`expected error "%s", got "%v"`, expectedError, err)
			}
			if addr != "" {
				t.Errorf(`expected addr to be empty, got "%s""`, addr)
			}
		})
	}
	for validAddr, expectedAddr := range valids {
		t.Run(validAddr, func(t *testing.T) {
			addr, err := ParseTCPAddr(validAddr, defaultHTTPHost)
			if err != nil {
				t.Errorf(`unexpected error: "%v"`, err)
			}
			if addr != expectedAddr {
				t.Errorf(`expected "%s", got "%s""`, expectedAddr, addr)
			}
		})
	}
}

func TestParseInvalidUnixAddrInvalid(t *testing.T) {
	if _, err := parseSimpleProtoAddr("unix", "tcp://127.0.0.1", "unix:///var/run/docker.sock"); err == nil || err.Error() != "Invalid proto, expected unix: tcp://127.0.0.1" {
		t.Fatalf("Expected an error, got %v", err)
	}
	if _, err := parseSimpleProtoAddr("unix", "unix://tcp://127.0.0.1", "/var/run/docker.sock"); err == nil || err.Error() != "Invalid proto, expected unix: tcp://127.0.0.1" {
		t.Fatalf("Expected an error, got %v", err)
	}
	if v, err := parseSimpleProtoAddr("unix", "", "/var/run/docker.sock"); err != nil || v != "unix:///var/run/docker.sock" {
		t.Fatalf("Expected an %v, got %v", v, "unix:///var/run/docker.sock")
	}
}

func TestValidateExtraHosts(t *testing.T) {
	valid := []string{
		`myhost:192.168.0.1`,
		`thathost:10.0.2.1`,
		`anipv6host:2003:ab34:e::1`,
		`ipv6local:::1`,
	}

	invalid := map[string]string{
		`myhost:192.notanipaddress.1`:  `invalid IP`,
		`thathost-nosemicolon10.0.0.1`: `bad format`,
		`anipv6host:::::1`:             `invalid IP`,
		`ipv6local:::0::`:              `invalid IP`,
	}

	for _, extrahost := range valid {
		if _, err := ValidateExtraHost(extrahost); err != nil {
			t.Fatalf("ValidateExtraHost(`"+extrahost+"`) should succeed: error %v", err)
		}
	}

	for extraHost, expectedError := range invalid {
		if _, err := ValidateExtraHost(extraHost); err == nil {
			t.Fatalf("ValidateExtraHost(`%q`) should have failed validation", extraHost)
		} else {
			if !strings.Contains(err.Error(), expectedError) {
				t.Fatalf("ValidateExtraHost(`%q`) error should contain %q", extraHost, expectedError)
			}
		}
	}
}
