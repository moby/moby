package registry

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func pass() {
	pc := make([]uintptr, 10)
	runtime.Callers(0, pc)
	fc := runtime.FuncForPC(pc[2])

	fn, _ := fc.FileLine(2)
	fn = regexp.MustCompilePOSIX(".*/([^/]*).go").FindStringSubmatch(fn)[1]

	name := fc.Name()
	name = regexp.MustCompilePOSIX(".*\\.(.*)").FindStringSubmatch(name)[1]

	fmt.Printf(" PASS - %s: %s\n", fn, name)
}

func TestMissingFile(t *testing.T) {
	tmpHome, _ := ioutil.TempDir("", "config-test")

	config, err := LoadConfig(tmpHome)
	if err != nil {
		t.Fatalf("Failed loading on missing file: %q", err)
	}

	// Now save it and make sure it shows up in new form
	err = SaveConfig(config)
	if err != nil {
		t.Fatalf("Failed to save: %q", err)
	}

	buf, err := ioutil.ReadFile(filepath.Join(tmpHome, CONFIGFILE))
	if !strings.Contains(string(buf), `"auths":`) {
		t.Fatalf("Should have save in new form: %s", string(buf))
	}

	pass()
}

func TestEmptyFile(t *testing.T) {
	tmpHome, _ := ioutil.TempDir("", "config-test")
	fn := filepath.Join(tmpHome, CONFIGFILE)
	ioutil.WriteFile(fn, []byte(""), 0600)

	_, err := LoadConfig(tmpHome)
	if err == nil {
		t.Fatalf("Was supposed to fail")
	}

	pass()
}

func TestEmptyJson(t *testing.T) {
	tmpHome, _ := ioutil.TempDir("", "config-test")
	fn := filepath.Join(tmpHome, CONFIGFILE)
	ioutil.WriteFile(fn, []byte("{}"), 0600)

	config, err := LoadConfig(tmpHome)
	if err != nil {
		t.Fatalf("Failed loading on empty json file: %q", err)
	}

	// Now save it and make sure it shows up in new form
	err = SaveConfig(config)
	if err != nil {
		t.Fatalf("Failed to save: %q", err)
	}

	buf, err := ioutil.ReadFile(filepath.Join(tmpHome, CONFIGFILE))
	if !strings.Contains(string(buf), `"auths":`) {
		t.Fatalf("Should have save in new form: %s", string(buf))
	}

	pass()
}

func TestOldJson(t *testing.T) {
	tmpHome, _ := ioutil.TempDir("", "config-test")
	fn := filepath.Join(tmpHome, CONFIGFILE)
	js := `{"https://index.docker.io/v1/":{"auth":"am9lam9lOmhlbGxv","email":"joe@gmail.com"}}`
	ioutil.WriteFile(fn, []byte(js), 0600)

	config, err := LoadConfig(tmpHome)
	if err != nil {
		t.Fatalf("Failed loading on empty json file: %q", err)
	}

	if config.AuthConfigs["https://index.docker.io/v1/"].Email != "joe@gmail.com" {
		t.Fatalf("Missing data from parsing:\n%q", config)
	}

	// Now save it and make sure it shows up in new form
	err = SaveConfig(config)
	if err != nil {
		t.Fatalf("Failed to save: %q", err)
	}

	buf, err := ioutil.ReadFile(filepath.Join(tmpHome, CONFIGFILE))
	if !strings.Contains(string(buf), `"auths":`) ||
		!strings.Contains(string(buf), "joe@gmail.com") {
		t.Fatalf("Should have save in new form: %s", string(buf))
	}

	pass()
}

func TestNewJson(t *testing.T) {
	tmpHome, _ := ioutil.TempDir("", "config-test")
	fn := filepath.Join(tmpHome, CONFIGFILE)
	js := ` { "auths": { "https://index.docker.io/v1/": { "auth": "am9lam9lOmhlbGxv", "email": "joe@gmail.com" } } }`
	ioutil.WriteFile(fn, []byte(js), 0600)

	config, err := LoadConfig(tmpHome)
	if err != nil {
		t.Fatalf("Failed loading on empty json file: %q", err)
	}

	if config.AuthConfigs["https://index.docker.io/v1/"].Email != "joe@gmail.com" {
		t.Fatalf("Missing data from parsing:\n%q", config)
	}

	// Now save it and make sure it shows up in new form
	err = SaveConfig(config)
	if err != nil {
		t.Fatalf("Failed to save: %q", err)
	}

	buf, err := ioutil.ReadFile(filepath.Join(tmpHome, CONFIGFILE))
	if !strings.Contains(string(buf), `"auths":`) ||
		!strings.Contains(string(buf), "joe@gmail.com") {
		t.Fatalf("Should have save in new form: %s", string(buf))
	}

	pass()
}
