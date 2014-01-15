package docker

import (
	"github.com/dotcloud/docker"
	"testing"
)

func TestReadCgroup(t *testing.T) {
	eng := NewTestEngine(t)
	srv := mkServerFromEngine(eng, t)
	defer mkRuntimeFromEngine(eng, t).Nuke()

	config, hostConfig, _, err := docker.ParseRun([]string{"-i", "-m", "100m", "-c", "1000", "-lxc-conf", "lxc.cgroup.cpuset.cpus=1", unitTestImageID, "/bin/cat"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	id := createTestContainer(eng, config, t)

	job := eng.Job("start", id)
	if err := job.ImportEnv(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	raw := map[string]string{
		"memory.limit_in_bytes": "104857600",
		"cpu.shares":            "1000",
		"cpuset.cpus":           "1",
	}
	cgroupData := &docker.APICgroup{}
	cgroupData.ReadSubsystem = []string{"memory.limit_in_bytes", "cpu.shares", "cpuset.cpus"}

	cgroupResponses, err := srv.ContainerCgroup(id, cgroupData, false)

	if err != nil {
		t.Fatal(err)
	}

	if len(cgroupResponses) != 3 {
		t.Fatalf("Except length is 3, actual is %d", len(cgroupResponses))
	}

	for _, cgroupResponse := range cgroupResponses {
		if cgroupResponse.Status != 0 {
			t.Fatalf("Unexcepted status %d for subsystem %s", cgroupResponse.Status, cgroupResponse.Subsystem)
		}
		value, exist := raw[cgroupResponse.Subsystem]
		if exist {
			if value != cgroupResponse.Out {
				t.Fatalf("Unexcepted output %s for subsystem %s", cgroupResponse.Out, cgroupResponse.Subsystem)
			}
		} else {
			t.Fatalf("Unexcepted subsystem %s", cgroupResponse.Subsystem)
		}
	}
}

func TestWriteCgroup(t *testing.T) {
	eng := NewTestEngine(t)
	srv := mkServerFromEngine(eng, t)
	defer mkRuntimeFromEngine(eng, t).Nuke()

	config, hostConfig, _, err := docker.ParseRun([]string{"-i", "-m", "100m", unitTestImageID, "/bin/cat"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	id := createTestContainer(eng, config, t)

	job := eng.Job("start", id)
	if err := job.ImportEnv(hostConfig); err != nil {
		t.Fatal(err)
	}
	if err := job.Run(); err != nil {
		t.Fatal(err)
	}

	raw := map[string]string{
		"memory.memsw.limit_in_bytes": "524288000",
		"memory.limit_in_bytes":       "209715200",
		"cpu.shares":                  "500",
		"cpuset.cpus":                 "0-1",
	}
	cgroupData := &docker.APICgroup{}
	for key, value := range raw {
		cgroupData.WriteSubsystem = append(cgroupData.WriteSubsystem, docker.KeyValuePair{Key: key, Value: value})
	}

	cgroupResponses, err := srv.ContainerCgroup(id, cgroupData, true)

	if err != nil {
		t.Fatal(err)
	}

	if len(cgroupResponses) != 4 {
		t.Fatalf("Except length is 4, actual is %d", len(cgroupResponses))
	}

	for _, cgroupResponse := range cgroupResponses {
		if cgroupResponse.Status != 0 {
			t.Fatalf("Unexcepted status %d for subsystem %s", cgroupResponse.Status, cgroupResponse.Subsystem)
		}
		_, exist := raw[cgroupResponse.Subsystem]
		if exist {
			if cgroupResponse.Out != "" || cgroupResponse.Err != "" {
				t.Fatalf("Unexcepted stdout %s, stderr %s for subsystem %s", cgroupResponse.Out, cgroupResponse.Err, cgroupResponse.Subsystem)
			}
		} else {
			t.Fatalf("Unexcepted subsystem %s", cgroupResponse.Subsystem)
		}
	}
}
