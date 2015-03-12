package runconfig

import (
	"reflect"
	"testing"

	"github.com/docker/docker/nat"
)

func TestMergeUnsetEnv(t *testing.T) {
	conf := &Config{UnsetEnv: []string{"DEBUG"}}
	imgConf := &Config{Env: []string{"DEBUG=true", "PATH=/bin"}}

	err := Merge(conf, imgConf)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	expected := []string{"PATH=/bin"}
	if !reflect.DeepEqual(conf.Env, expected) {
		t.Errorf("Env(%v), want %v", imgConf.Env, expected)
	}
}

func TestMergeUnsetPorts(t *testing.T) {
	portSpecs := []string{"3000/tcp", "8080/tcp"}
	ports, _, err := nat.ParsePortSpecs(portSpecs)
	if err != nil {
		t.Errorf("Failed to parse port specs %v, err %s", portSpecs, err)
	}
	rmPortSpecs := []string{"3000/tcp"}
	rmPorts, _, err := nat.ParsePortSpecs(rmPortSpecs)
	if err != nil {
		t.Errorf("Failed to parse port specs %v, err %s", rmPortSpecs, err)
	}

	conf := &Config{UnsetPorts: rmPorts}
	imgConf := &Config{ExposedPorts: ports}

	err = Merge(conf, imgConf)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}

	if _, exists := conf.ExposedPorts["3000/tcp"]; exists {
		t.Errorf("ExposedPorts(%v) should not have %s", conf.ExposedPorts, "3000/tcp")
	}
}

func TestMergeUnsetVolumes(t *testing.T) {
	conf := &Config{UnsetVolumes: map[string]struct{}{
		"/test1": {},
	}}
	imgConf := &Config{Volumes: map[string]struct{}{
		"/test1": {},
		"/test2": {},
	}}

	err := Merge(conf, imgConf)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}

	if _, exists := conf.Volumes["/test1"]; exists {
		t.Errorf("Volumes(%v) should not have %s", conf.Volumes, "/test1")
	}
}
