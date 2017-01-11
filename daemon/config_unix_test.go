// +build !windows

package daemon

import (
	"io/ioutil"
	"testing"
)

func TestDaemonConfigurationMerge(t *testing.T) {
	f, err := ioutil.TempFile("", "docker-config-")
	if err != nil {
		t.Fatal(err)
	}

	configFile := f.Name()

	f.Write([]byte(`
		{
			"debug": true,
			"default-ulimits": {
				"nofile": {
					"Name": "nofile",
					"Hard": 2048,
					"Soft": 1024
				}
			},
			"log-opts": {
				"tag": "test_tag"
			}
		}`))

	f.Close()

	c := &Config{
		CommonConfig: CommonConfig{
			AutoRestart: true,
			LogConfig: LogConfig{
				Type:   "syslog",
				Config: map[string]string{"tag": "test"},
			},
		},
	}

	cc, err := MergeDaemonConfigurations(c, nil, configFile)
	if err != nil {
		t.Fatal(err)
	}
	if !cc.Debug {
		t.Fatalf("expected %v, got %v\n", true, cc.Debug)
	}
	if !cc.AutoRestart {
		t.Fatalf("expected %v, got %v\n", true, cc.AutoRestart)
	}
	if cc.LogConfig.Type != "syslog" {
		t.Fatalf("expected syslog config, got %q\n", cc.LogConfig)
	}

	if configValue, OK := cc.LogConfig.Config["tag"]; !OK {
		t.Fatal("expected syslog config attributes, got nil\n")
	} else {
		if configValue != "test_tag" {
			t.Fatalf("expected syslog config attributes 'tag=test_tag', got 'tag=%s'\n", configValue)
		}
	}

	if cc.Ulimits == nil {
		t.Fatal("expected default ulimit config, got nil\n")
	} else {
		if _, OK := cc.Ulimits["nofile"]; OK {
			if cc.Ulimits["nofile"].Name != "nofile" ||
				cc.Ulimits["nofile"].Hard != 2048 ||
				cc.Ulimits["nofile"].Soft != 1024 {
				t.Fatalf("expected default ulimit name, hard and soft are nofile, 2048, 1024, got %s, %d, %d\n", cc.Ulimits["nofile"].Name, cc.Ulimits["nofile"].Hard, cc.Ulimits["nofile"].Soft)
			}
		} else {
			t.Fatal("expected default ulimit name nofile, got nil\n")
		}
	}
}
