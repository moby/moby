package docker

import (
	"testing"
)

func TestCompareConfig(t *testing.T) {
	volumes1 := make(map[string]struct{})
	volumes1["/test1"] = struct{}{}
	config1 := Config{
		Dns:         []string{"1.1.1.1", "2.2.2.2"},
		PortSpecs:   []string{"1111:1111", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "11111111",
		Volumes:     volumes1,
	}
	config2 := Config{
		Dns:         []string{"0.0.0.0", "2.2.2.2"},
		PortSpecs:   []string{"1111:1111", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "11111111",
		Volumes:     volumes1,
	}
	config3 := Config{
		Dns:         []string{"1.1.1.1", "2.2.2.2"},
		PortSpecs:   []string{"0000:0000", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "11111111",
		Volumes:     volumes1,
	}
	config4 := Config{
		Dns:         []string{"1.1.1.1", "2.2.2.2"},
		PortSpecs:   []string{"0000:0000", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "22222222",
		Volumes:     volumes1,
	}
	volumes2 := make(map[string]struct{})
	volumes2["/test2"] = struct{}{}
	config5 := Config{
		Dns:         []string{"1.1.1.1", "2.2.2.2"},
		PortSpecs:   []string{"0000:0000", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "11111111",
		Volumes:     volumes2,
	}
	if CompareConfig(&config1, &config2) {
		t.Fatalf("CompareConfig should return false, Dns are different")
	}
	if CompareConfig(&config1, &config3) {
		t.Fatalf("CompareConfig should return false, PortSpecs are different")
	}
	if CompareConfig(&config1, &config4) {
		t.Fatalf("CompareConfig should return false, VolumesFrom are different")
	}
	if CompareConfig(&config1, &config5) {
		t.Fatalf("CompareConfig should return false, Volumes are different")
	}
	if !CompareConfig(&config1, &config1) {
		t.Fatalf("CompareConfig should return true")
	}
}

func TestMergeConfig(t *testing.T) {
	volumesImage := make(map[string]struct{})
	volumesImage["/test1"] = struct{}{}
	volumesImage["/test2"] = struct{}{}
	configImage := &Config{
		Dns:         []string{"1.1.1.1", "2.2.2.2"},
		PortSpecs:   []string{"1111:1111", "2222:2222"},
		Env:         []string{"VAR1=1", "VAR2=2"},
		VolumesFrom: "1111",
		Volumes:     volumesImage,
	}

	volumesUser := make(map[string]struct{})
	volumesUser["/test3"] = struct{}{}
	configUser := &Config{
		Dns:       []string{"3.3.3.3"},
		PortSpecs: []string{"3333:2222", "3333:3333"},
		Env:       []string{"VAR2=3", "VAR3=3"},
		Volumes:   volumesUser,
	}

	if err := MergeConfig(configUser, configImage); err != nil {
		t.Error(err)
	}

	if len(configUser.Dns) != 3 {
		t.Fatalf("Expected 3 dns, 1.1.1.1, 2.2.2.2 and 3.3.3.3, found %d", len(configUser.Dns))
	}
	for _, dns := range configUser.Dns {
		if dns != "1.1.1.1" && dns != "2.2.2.2" && dns != "3.3.3.3" {
			t.Fatalf("Expected 1.1.1.1 or 2.2.2.2 or 3.3.3.3, found %s", dns)
		}
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

	if configUser.VolumesFrom != "1111" {
		t.Fatalf("Expected VolumesFrom to be 1111, found %s", configUser.VolumesFrom)
	}

	ports, _, err := parsePortSpecs([]string{"0000"})
	if err != nil {
		t.Error(err)
	}
	configImage2 := &Config{
		ExposedPorts: ports,
	}

	if err := MergeConfig(configUser, configImage2); err != nil {
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
