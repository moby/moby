package docker

import (
	"testing"
)

func TestStart(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	container, err := docker.Create(
		"start_test",
		"ls",
		[]string{"-al"},
		[]string{"/var/lib/docker/images/ubuntu"},
		&Config{
			Ram: 33554432,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)

	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	container.Wait()
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	// We should be able to call Wait again
	container.Wait()
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
}

func TestRun(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	container, err := docker.Create(
		"run_test",
		"ls",
		[]string{"-al"},
		[]string{"/var/lib/docker/images/ubuntu"},
		&Config{
			Ram: 33554432,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)

	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	if err := container.Run(); err != nil {
		t.Fatal(err)
	}
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
}

func TestOutput(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	container, err := docker.Create(
		"output_test",
		"echo",
		[]string{"-n", "foobar"},
		[]string{"/var/lib/docker/images/ubuntu"},
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)

	pipe, err := container.StdoutPipe()
	defer pipe.Close()
	output, err := container.Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "foobar" {
		t.Error(string(output))
	}
}

func TestKill(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}
	container, err := docker.Create(
		"stop_test",
		"cat",
		[]string{"/dev/zero"},
		[]string{"/var/lib/docker/images/ubuntu"},
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container)

	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	if err := container.Start(); err != nil {
		t.Fatal(err)
	}
	if !container.State.Running {
		t.Errorf("Container should be running")
	}
	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	container.Wait()
	if container.State.Running {
		t.Errorf("Container shouldn't be running")
	}
	// Try stopping twice
	if err := container.Kill(); err != nil {
		t.Fatal(err)
	}
}

func TestExitCode(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}

	trueContainer, err := docker.Create(
		"exit_test_1",
		"/bin/true",
		[]string{""},
		[]string{"/var/lib/docker/images/ubuntu"},
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(trueContainer)
	if err := trueContainer.Run(); err != nil {
		t.Fatal(err)
	}

	falseContainer, err := docker.Create(
		"exit_test_2",
		"/bin/false",
		[]string{""},
		[]string{"/var/lib/docker/images/ubuntu"},
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(falseContainer)
	if err := falseContainer.Run(); err != nil {
		t.Fatal(err)
	}

	if trueContainer.State.ExitCode != 0 {
		t.Errorf("Unexpected exit code %v", trueContainer.State.ExitCode)
	}

	if falseContainer.State.ExitCode != 1 {
		t.Errorf("Unexpected exit code %v", falseContainer.State.ExitCode)
	}
}

func TestMultipleContainers(t *testing.T) {
	docker, err := newTestDocker()
	if err != nil {
		t.Fatal(err)
	}

	container1, err := docker.Create(
		"container1",
		"cat",
		[]string{"/dev/zero"},
		[]string{"/var/lib/docker/images/ubuntu"},
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container1)

	container2, err := docker.Create(
		"container2",
		"cat",
		[]string{"/dev/zero"},
		[]string{"/var/lib/docker/images/ubuntu"},
		&Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Destroy(container2)

	// Start both containers
	if err := container1.Start(); err != nil {
		t.Fatal(err)
	}
	if err := container2.Start(); err != nil {
		t.Fatal(err)
	}

	// If we are here, both containers should be running
	if !container1.State.Running {
		t.Fatal("Container not running")
	}
	if !container2.State.Running {
		t.Fatal("Container not running")
	}

	// Kill them
	if err := container1.Kill(); err != nil {
		t.Fatal(err)
	}

	if err := container2.Kill(); err != nil {
		t.Fatal(err)
	}
}
