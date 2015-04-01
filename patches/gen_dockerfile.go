package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func genDockerfile() (string, error) {
	file, err := os.Open("/etc/redhat-release")
	if err != nil {
		return "Dockerfile", nil
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	scanner := bufio.NewScanner(reader)
	scanner.Scan()
	line := strings.Split(scanner.Text(), " ")
	os_str := line[0]
	switch os_str {
	case "Fedora", "Centos":
		break
	case "red":
		os_str = "rhel"
		break

	default:
		return "Dockerfile", nil
	}
	if err := patchDockerfile(os_str); err != nil {
		return "", err
	}
	return "patches/Dockerfile" + os_str, nil
}

func patchDockerfile(os_name string) error {
	patchStr := "patch -p1 -o patches/Dockerfile" + os_name + " < patches/" + os_name + ".patch"
	patcher := []string{"-c", patchStr}
	c := exec.Command("/bin/sh", patcher...)
	out, err := c.CombinedOutput()
	if err != nil {
		fmt.Println(string(out), err)
	}
	return err
}

func main() {

	dockerfile, err := genDockerfile()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(dockerfile)

}
