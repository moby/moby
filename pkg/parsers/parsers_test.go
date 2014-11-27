package parsers

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseHost(t *testing.T) {
	var (
		defaultHttpHost = "127.0.0.1"
		defaultUnix     = "/var/run/docker.sock"
	)
	if addr, err := ParseHost(defaultHttpHost, defaultUnix, "0.0.0.0"); err == nil {
		t.Errorf("tcp 0.0.0.0 address expected error return, but err == nil, got %s", addr)
	}
	if addr, err := ParseHost(defaultHttpHost, defaultUnix, "tcp://"); err == nil {
		t.Errorf("default tcp:// address expected error return, but err == nil, got %s", addr)
	}
	if addr, err := ParseHost(defaultHttpHost, defaultUnix, "0.0.0.1:5555"); err != nil || addr != "tcp://0.0.0.1:5555" {
		t.Errorf("0.0.0.1:5555 -> expected tcp://0.0.0.1:5555, got %s", addr)
	}
	if addr, err := ParseHost(defaultHttpHost, defaultUnix, ":6666"); err != nil || addr != "tcp://127.0.0.1:6666" {
		t.Errorf(":6666 -> expected tcp://127.0.0.1:6666, got %s", addr)
	}
	if addr, err := ParseHost(defaultHttpHost, defaultUnix, "tcp://:7777"); err != nil || addr != "tcp://127.0.0.1:7777" {
		t.Errorf("tcp://:7777 -> expected tcp://127.0.0.1:7777, got %s", addr)
	}
	if addr, err := ParseHost(defaultHttpHost, defaultUnix, ""); err != nil || addr != "unix:///var/run/docker.sock" {
		t.Errorf("empty argument -> expected unix:///var/run/docker.sock, got %s", addr)
	}
	if addr, err := ParseHost(defaultHttpHost, defaultUnix, "unix:///var/run/docker.sock"); err != nil || addr != "unix:///var/run/docker.sock" {
		t.Errorf("unix:///var/run/docker.sock -> expected unix:///var/run/docker.sock, got %s", addr)
	}
	if addr, err := ParseHost(defaultHttpHost, defaultUnix, "unix://"); err != nil || addr != "unix:///var/run/docker.sock" {
		t.Errorf("unix:///var/run/docker.sock -> expected unix:///var/run/docker.sock, got %s", addr)
	}
	if addr, err := ParseHost(defaultHttpHost, defaultUnix, "udp://127.0.0.1"); err == nil {
		t.Errorf("udp protocol address expected error return, but err == nil. Got %s", addr)
	}
	if addr, err := ParseHost(defaultHttpHost, defaultUnix, "udp://127.0.0.1:2375"); err == nil {
		t.Errorf("udp protocol address expected error return, but err == nil. Got %s", addr)
	}
}

func TestParseRepositoryTag(t *testing.T) {
	if repo, tag := ParseRepositoryTag("root"); repo != "root" || tag != "" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "root", "", repo, tag)
	}
	if repo, tag := ParseRepositoryTag("root:tag"); repo != "root" || tag != "tag" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "root", "tag", repo, tag)
	}
	if repo, tag := ParseRepositoryTag("user/repo"); repo != "user/repo" || tag != "" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "user/repo", "", repo, tag)
	}
	if repo, tag := ParseRepositoryTag("user/repo:tag"); repo != "user/repo" || tag != "tag" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "user/repo", "tag", repo, tag)
	}
	if repo, tag := ParseRepositoryTag("url:5000/repo"); repo != "url:5000/repo" || tag != "" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "url:5000/repo", "", repo, tag)
	}
	if repo, tag := ParseRepositoryTag("url:5000/repo:tag"); repo != "url:5000/repo" || tag != "tag" {
		t.Errorf("Expected repo: '%s' and tag: '%s', got '%s' and '%s'", "url:5000/repo", "tag", repo, tag)
	}
}

func TestParsePortMapping(t *testing.T) {
	data, err := PartParser("ip:public:private", "192.168.1.1:80:8080")
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != 3 {
		t.FailNow()
	}
	if data["ip"] != "192.168.1.1" {
		t.Fail()
	}
	if data["public"] != "80" {
		t.Fail()
	}
	if data["private"] != "8080" {
		t.Fail()
	}
}

func TestParseFilterDate(t *testing.T) {
	tz := strings.Split(time.Now().Local().String(), "+")[1]
	testData := []struct {
		t string
		v string
	}{
		{"2012-03-12 05:04", "2012-03-12 05:04:00 +" + tz},
		{"2012-03-12", "2012-03-12 00:00:00 +" + tz},
		{"05:04", fmt.Sprintf("%d-%02d-%02d 05:04:00 +%s", time.Now().Year(), time.Now().Month(), time.Now().Day(), tz)},
	}

	for _, d := range testData {
		date, err := ParseFilterDate(d.t)
		if err != nil {
			t.Errorf("%s: %s", d.t, err)
		}
		if d.v != date.String() {
			t.Errorf("%s: Expecting %v, got %v", d.t, d.v, date)
		}
	}
}

func TestParseFilterDateError(t *testing.T) {
	testData := []struct {
		t string
	}{
		{"05:04 2012-03-12"},
		{"2012-13-03"},
		{"2012-2-3"},
		{"25:66"},
		{"05:60"},
		{"5:6"},
	}

	for _, d := range testData {
		if date, err := ParseFilterDate(d.t); err == nil {
			t.Errorf("%s: Expecting error, got %s", d.t, date)
		}
	}
}
