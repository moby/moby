package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func deleteContainer(container string) error {
	container = strings.Replace(container, "\n", " ", -1)
	container = strings.Trim(container, " ")
	killArgs := fmt.Sprintf("kill %v", container)
	killSplitArgs := strings.Split(killArgs, " ")
	killCmd := exec.Command(dockerBinary, killSplitArgs...)
	runCommand(killCmd)
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

func imageExists(image string) error {
	inspectCmd := exec.Command(dockerBinary, "inspect", image)
	exitCode, err := runCommand(inspectCmd)
	if exitCode != 0 && err == nil {
		err = fmt.Errorf("couldn't find image '%s'", image)
	}
	return err
}

func pullImageIfNotExist(image string) (err error) {
	if err := imageExists(image); err != nil {
		pullCmd := exec.Command(dockerBinary, "pull", image)
		_, exitCode, err := runCommandWithOutput(pullCmd)

		if err != nil || exitCode != 0 {
			err = fmt.Errorf("image '%s' wasn't found locally and it couldn't be pulled: %s", image, err)
		}
	}
	return
}

// deprecated, use dockerCmd instead
func cmd(t *testing.T, args ...string) (string, int, error) {
	return dockerCmd(t, args...)
}

func dockerCmd(t *testing.T, args ...string) (string, int, error) {
	out, status, err := runCommandWithOutput(exec.Command(dockerBinary, args...))
	errorOut(err, t, fmt.Sprintf("'%s' failed with errors: %v (%v)", strings.Join(args, " "), err, out))
	return out, status, err
}

// execute a docker command in a directory
func dockerCmdInDir(t *testing.T, path string, args ...string) (string, int, error) {
	dockerCommand := exec.Command(dockerBinary, args...)
	dockerCommand.Dir = path
	out, status, err := runCommandWithOutput(dockerCommand)
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

type FakeContext struct {
	Dir string
}

func (f *FakeContext) Add(file, content string) error {
	filepath := path.Join(f.Dir, file)
	dirpath := path.Dir(filepath)
	if dirpath != "." {
		if err := os.MkdirAll(dirpath, 0755); err != nil {
			return err
		}
	}
	return ioutil.WriteFile(filepath, []byte(content), 0644)
}

func (f *FakeContext) Delete(file string) error {
	filepath := path.Join(f.Dir, file)
	return os.RemoveAll(filepath)
}

func (f *FakeContext) Close() error {
	return os.RemoveAll(f.Dir)
}

func fakeContext(dockerfile string, files map[string]string) (*FakeContext, error) {
	tmp, err := ioutil.TempDir("", "fake-context")
	if err != nil {
		return nil, err
	}
	ctx := &FakeContext{tmp}
	for file, content := range files {
		if err := ctx.Add(file, content); err != nil {
			ctx.Close()
			return nil, err
		}
	}
	if err := ctx.Add("Dockerfile", dockerfile); err != nil {
		ctx.Close()
		return nil, err
	}
	return ctx, nil
}

type FakeStorage struct {
	*FakeContext
	*httptest.Server
}

func (f *FakeStorage) Close() error {
	f.Server.Close()
	return f.FakeContext.Close()
}

func fakeStorage(files map[string]string) (*FakeStorage, error) {
	tmp, err := ioutil.TempDir("", "fake-storage")
	if err != nil {
		return nil, err
	}
	ctx := &FakeContext{tmp}
	for file, content := range files {
		if err := ctx.Add(file, content); err != nil {
			ctx.Close()
			return nil, err
		}
	}
	handler := http.FileServer(http.Dir(ctx.Dir))
	server := httptest.NewServer(handler)
	return &FakeStorage{
		FakeContext: ctx,
		Server:      server,
	}, nil
}

func inspectField(name, field string) (string, error) {
	format := fmt.Sprintf("{{.%s}}", field)
	inspectCmd := exec.Command(dockerBinary, "inspect", "-f", format, name)
	out, exitCode, err := runCommandWithOutput(inspectCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to inspect %s: %s", name, out)
	}
	return strings.TrimSpace(out), nil
}

func inspectFieldJSON(name, field string) (string, error) {
	format := fmt.Sprintf("{{json .%s}}", field)
	inspectCmd := exec.Command(dockerBinary, "inspect", "-f", format, name)
	out, exitCode, err := runCommandWithOutput(inspectCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to inspect %s: %s", name, out)
	}
	return strings.TrimSpace(out), nil
}

func getIDByName(name string) (string, error) {
	return inspectField(name, "Id")
}

func buildImageWithOut(name, dockerfile string, useCache bool) (string, string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, "-")
	buildCmd := exec.Command(dockerBinary, args...)
	buildCmd.Stdin = strings.NewReader(dockerfile)
	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		return "", out, fmt.Errorf("failed to build the image: %s", out)
	}
	id, err := getIDByName(name)
	if err != nil {
		return "", out, err
	}
	return id, out, nil
}

func buildImage(name, dockerfile string, useCache bool) (string, error) {
	id, _, err := buildImageWithOut(name, dockerfile, useCache)
	return id, err
}

func buildImageFromContext(name string, ctx *FakeContext, useCache bool) (string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, ".")
	buildCmd := exec.Command(dockerBinary, args...)
	buildCmd.Dir = ctx.Dir
	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to build the image: %s", out)
	}
	return getIDByName(name)
}

func buildImageFromPath(name, path string, useCache bool) (string, error) {
	args := []string{"build", "-t", name}
	if !useCache {
		args = append(args, "--no-cache")
	}
	args = append(args, path)
	buildCmd := exec.Command(dockerBinary, args...)
	out, exitCode, err := runCommandWithOutput(buildCmd)
	if err != nil || exitCode != 0 {
		return "", fmt.Errorf("failed to build the image: %s", out)
	}
	return getIDByName(name)
}

type FakeGIT struct {
	*httptest.Server
	Root    string
	RepoURL string
}

func (g *FakeGIT) Close() {
	g.Server.Close()
	os.RemoveAll(g.Root)
}

func fakeGIT(name string, files map[string]string) (*FakeGIT, error) {
	tmp, err := ioutil.TempDir("", "fake-git-repo")
	if err != nil {
		return nil, err
	}
	ctx := &FakeContext{tmp}
	for file, content := range files {
		if err := ctx.Add(file, content); err != nil {
			ctx.Close()
			return nil, err
		}
	}
	defer ctx.Close()
	curdir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	defer os.Chdir(curdir)

	if output, err := exec.Command("git", "init", ctx.Dir).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("Error trying to init repo: %s (%s)", err, output)
	}
	err = os.Chdir(ctx.Dir)
	if err != nil {
		return nil, err
	}
	if output, err := exec.Command("git", "add", "*").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("Error trying to add files to repo: %s (%s)", err, output)
	}
	if output, err := exec.Command("git", "commit", "-a", "-m", "Initial commit").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("Error trying to commit to repo: %s (%s)", err, output)
	}

	root, err := ioutil.TempDir("", "docker-test-git-repo")
	if err != nil {
		return nil, err
	}
	repoPath := filepath.Join(root, name+".git")
	if output, err := exec.Command("git", "clone", "--bare", ctx.Dir, repoPath).CombinedOutput(); err != nil {
		os.RemoveAll(root)
		return nil, fmt.Errorf("Error trying to clone --bare: %s (%s)", err, output)
	}
	err = os.Chdir(repoPath)
	if err != nil {
		os.RemoveAll(root)
		return nil, err
	}
	if output, err := exec.Command("git", "update-server-info").CombinedOutput(); err != nil {
		os.RemoveAll(root)
		return nil, fmt.Errorf("Error trying to git update-server-info: %s (%s)", err, output)
	}
	err = os.Chdir(curdir)
	if err != nil {
		os.RemoveAll(root)
		return nil, err
	}
	handler := http.FileServer(http.Dir(root))
	server := httptest.NewServer(handler)
	return &FakeGIT{
		Server:  server,
		Root:    root,
		RepoURL: fmt.Sprintf("%s/%s.git", server.URL, name),
	}, nil
}
