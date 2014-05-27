package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func deleteContainer(container string) error {
	container = strings.Replace(container, "\n", " ", -1)
	container = strings.Trim(container, " ")
	rmArgs := fmt.Sprintf("rm %v", container)
	rmSplitArgs := strings.Split(rmArgs, " ")
	rmCmd := exec.Command(dockerBinary, rmSplitArgs...)
	exitCode, err := runCommand(rmCmd)
	// set error manually if not set
	if exitCode != 0 && err == nil {
		err = fmt.Errorf("failed to remove container: `docker rm` exit is non-zero")
	}

	return err
}

func getAllContainers() (string, error) {
	getContainersCmd := exec.Command(dockerBinary, "ps", "-q", "-a")
	out, exitCode, err := runCommandWithOutput(getContainersCmd)
	if exitCode != 0 && err == nil {
		err = fmt.Errorf("failed to get a list of containers: %v\n", out)
	}

	return out, err
}

func deleteAllContainers() error {
	containers, err := getAllContainers()
	if err != nil {
		fmt.Println(containers)
		return err
	}

	if err = deleteContainer(containers); err != nil {
		return err
	}
	return nil
}

func deleteImages(images string) error {
	rmiCmd := exec.Command(dockerBinary, "rmi", images)
	exitCode, err := runCommand(rmiCmd)
	// set error manually if not set
	if exitCode != 0 && err == nil {
		err = fmt.Errorf("failed to remove image: `docker rmi` exit is non-zero")
	}

	return err
}

func cmd(t *testing.T, args ...string) (string, int, error) {
	out, status, err := runCommandWithOutput(exec.Command(dockerBinary, args...))
	errorOut(err, t, fmt.Sprintf("'%s' failed with errors: %v (%v)", strings.Join(args, " "), err, out))
	return out, status, err
}

func findContainerIp(t *testing.T, id string) string {
	cmd := exec.Command(dockerBinary, "inspect", "--format='{{ .NetworkSettings.IPAddress }}'", id)
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		t.Fatal(err, out)
	}

	return strings.Trim(out, " \r\n'")
}

func getContainerCount() (int, error) {
	const containers = "Containers:"

	cmd := exec.Command(dockerBinary, "info")
	out, _, err := runCommandWithOutput(cmd)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, containers) {
			output := stripTrailingCharacters(line)
			output = strings.TrimLeft(output, containers)
			output = strings.Trim(output, " ")
			containerCount, err := strconv.Atoi(output)
			if err != nil {
				return 0, err
			}
			return containerCount, nil
		}
	}
	return 0, fmt.Errorf("couldn't find the Container count in the output")
}
