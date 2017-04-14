// +build linux

package xdg

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/pkg/homedir"
)

func TestGetConfigFileEmptyEnvironmentAndNoFile(t *testing.T) {
	defer withEnv(map[string]string{})()
	actual, err := GetConfigFile("a/path")
	if err == nil {
		t.Fatalf("expected an error, got nothing : %s", actual)
	}
	expected := filepath.Join(homedir.Get(), ".config/a/path")
	if actual != expected {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestGetConfigFileWithXDGConfigAndNoFile(t *testing.T) {
	defer withEnv(map[string]string{"XDG_CONFIG_HOME": "/a/config/path"})()
	actual, err := GetConfigFile("a/path")
	if err == nil {
		t.Fatalf("expected an error, got nothing : %s", actual)
	}
	expected := "/a/config/path/a/path"
	if actual != expected {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestGetConfigFileXdgConfigDirsAndNoFile(t *testing.T) {
	defer withEnv(map[string]string{"XDG_CONFIG_DIRS": "/a/config/path:/another/config/path"})()
	actual, err := GetConfigFile("a/path")
	if err == nil {
		t.Fatalf("expected an error, got nothing : %s", actual)
	}
	expected := filepath.Join(homedir.Get(), ".config/a/path")
	if actual != expected {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestGetConfigFileWithXDGConfigHomeAndAFile(t *testing.T) {
	folder, cleanTemporaryFn := createTemporaryFile(t, "a/path")
	defer cleanTemporaryFn()
	defer withEnv(map[string]string{"XDG_CONFIG_HOME": folder})()
	actual, err := GetConfigFile("a/path")
	if err != nil {
		t.Fatalf("expected no error, got %v : %s", err, actual)
	}
	expected := filepath.Join(folder, "a/path")
	if actual != expected {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestGetConfigFileXdgConfigDirsAndAFile(t *testing.T) {
	folder, cleanTemporaryFn := createTemporaryFile(t, "a/path")
	defer cleanTemporaryFn()
	defer withEnv(map[string]string{"XDG_CONFIG_DIRS": "/a/config/path:" + folder})()
	actual, err := GetConfigFile("a/path")
	if err != nil {
		t.Fatalf("expected no error, got %v : %s", err, actual)
	}
	expected := filepath.Join(folder, "a/path")
	if actual != expected {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func createTemporaryFile(t *testing.T, filename string) (string, func()) {
	tmpFolder, err := ioutil.TempDir("", "xdg-test")
	if err != nil {
		t.Fatal(err)
	}
	d := filepath.Join(tmpFolder, filepath.Dir(filename))
	if err := os.MkdirAll(d, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(tmpFolder, filename), []byte("content"), 0755); err != nil {
		t.Fatal(err)
	}
	return tmpFolder, func() {
		if err := os.RemoveAll(d); err != nil {
			t.Fatal(err)
		}
	}
}

func withEnv(envs map[string]string) func() {
	oldEnvs := os.Environ()
	for _, oldEnv := range oldEnvs {
		key := strings.Split(oldEnv, "=")[0]
		os.Unsetenv(key)
	}
	for key, value := range envs {
		os.Setenv(key, value)
	}
	return func() {
		for _, oldEnv := range oldEnvs {
			e := strings.SplitN(oldEnv, "=", 2)
			os.Setenv(e[0], e[1])
		}
	}
}
