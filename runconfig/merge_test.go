package runconfig

import (
	"reflect"
	"testing"
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

