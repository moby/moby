package cliconfig

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/cliconfig/configfile"
	"github.com/docker/docker/pkg/homedir"
	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestEmptyConfigDir(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	SetConfigDir(tmpHome)

	config, err := Load("")
	if err != nil {
		c.Fatalf("Failed loading on empty config dir: %q", err)
	}

	expectedConfigFilename := filepath.Join(tmpHome, ConfigFileName)
	if config.Filename != expectedConfigFilename {
		c.Fatalf("Expected config filename %s, got %s", expectedConfigFilename, config.Filename)
	}

	// Now save it and make sure it shows up in new form
	saveConfigAndValidateNewFormat(c, config, tmpHome)
}

func (s *DockerSuite) TestMissingFile(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	config, err := Load(tmpHome)
	if err != nil {
		c.Fatalf("Failed loading on missing file: %q", err)
	}

	// Now save it and make sure it shows up in new form
	saveConfigAndValidateNewFormat(c, config, tmpHome)
}

func (s *DockerSuite) TestSaveFileToDirs(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	tmpHome += "/.docker"

	config, err := Load(tmpHome)
	if err != nil {
		c.Fatalf("Failed loading on missing file: %q", err)
	}

	// Now save it and make sure it shows up in new form
	saveConfigAndValidateNewFormat(c, config, tmpHome)
}

func (s *DockerSuite) TestEmptyFile(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	fn := filepath.Join(tmpHome, ConfigFileName)
	if err := ioutil.WriteFile(fn, []byte(""), 0600); err != nil {
		c.Fatal(err)
	}

	_, err = Load(tmpHome)
	if err == nil {
		c.Fatalf("Was supposed to fail")
	}
}

func (s *DockerSuite) TestEmptyJson(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	fn := filepath.Join(tmpHome, ConfigFileName)
	if err := ioutil.WriteFile(fn, []byte("{}"), 0600); err != nil {
		c.Fatal(err)
	}

	config, err := Load(tmpHome)
	if err != nil {
		c.Fatalf("Failed loading on empty json file: %q", err)
	}

	// Now save it and make sure it shows up in new form
	saveConfigAndValidateNewFormat(c, config, tmpHome)
}

func (s *DockerSuite) TestOldInvalidsAuth(c *check.C) {
	invalids := map[string]string{
		`username = test`: "The Auth config file is empty",
		`username
password`: "Invalid Auth config file",
		`username = test
email`: "Invalid auth configuration file",
	}

	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	homeKey := homedir.Key()
	homeVal := homedir.Get()

	defer func() { os.Setenv(homeKey, homeVal) }()
	os.Setenv(homeKey, tmpHome)

	for content, expectedError := range invalids {
		fn := filepath.Join(tmpHome, oldConfigfile)
		if err := ioutil.WriteFile(fn, []byte(content), 0600); err != nil {
			c.Fatal(err)
		}

		config, err := Load(tmpHome)
		// Use Contains instead of == since the file name will change each time
		if err == nil || !strings.Contains(err.Error(), expectedError) {
			c.Fatalf("Should have failed\nConfig: %v\nGot: %v\nExpected: %v", config, err, expectedError)
		}

	}
}

func (s *DockerSuite) TestOldValidAuth(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	homeKey := homedir.Key()
	homeVal := homedir.Get()

	defer func() { os.Setenv(homeKey, homeVal) }()
	os.Setenv(homeKey, tmpHome)

	fn := filepath.Join(tmpHome, oldConfigfile)
	js := `username = am9lam9lOmhlbGxv
	email = user@example.com`
	if err := ioutil.WriteFile(fn, []byte(js), 0600); err != nil {
		c.Fatal(err)
	}

	config, err := Load(tmpHome)
	if err != nil {
		c.Fatal(err)
	}

	// defaultIndexserver is https://index.docker.io/v1/
	ac := config.AuthConfigs["https://index.docker.io/v1/"]
	if ac.Username != "joejoe" || ac.Password != "hello" {
		c.Fatalf("Missing data from parsing:\n%q", config)
	}

	// Now save it and make sure it shows up in new form
	configStr := saveConfigAndValidateNewFormat(c, config, tmpHome)

	expConfStr := `{
	"auths": {
		"https://index.docker.io/v1/": {
			"auth": "am9lam9lOmhlbGxv"
		}
	}
}`

	if configStr != expConfStr {
		c.Fatalf("Should have save in new form: \n%s\n not \n%s", configStr, expConfStr)
	}
}

func (s *DockerSuite) TestOldJsonInvalid(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	homeKey := homedir.Key()
	homeVal := homedir.Get()

	defer func() { os.Setenv(homeKey, homeVal) }()
	os.Setenv(homeKey, tmpHome)

	fn := filepath.Join(tmpHome, oldConfigfile)
	js := `{"https://index.docker.io/v1/":{"auth":"test","email":"user@example.com"}}`
	if err := ioutil.WriteFile(fn, []byte(js), 0600); err != nil {
		c.Fatal(err)
	}

	config, err := Load(tmpHome)
	// Use Contains instead of == since the file name will change each time
	if err == nil || !strings.Contains(err.Error(), "Invalid auth configuration file") {
		c.Fatalf("Expected an error got : %v, %v", config, err)
	}
}

func (s *DockerSuite) TestOldJson(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	homeKey := homedir.Key()
	homeVal := homedir.Get()

	defer func() { os.Setenv(homeKey, homeVal) }()
	os.Setenv(homeKey, tmpHome)

	fn := filepath.Join(tmpHome, oldConfigfile)
	js := `{"https://index.docker.io/v1/":{"auth":"am9lam9lOmhlbGxv","email":"user@example.com"}}`
	if err := ioutil.WriteFile(fn, []byte(js), 0600); err != nil {
		c.Fatal(err)
	}

	config, err := Load(tmpHome)
	if err != nil {
		c.Fatalf("Failed loading on empty json file: %q", err)
	}

	ac := config.AuthConfigs["https://index.docker.io/v1/"]
	if ac.Username != "joejoe" || ac.Password != "hello" {
		c.Fatalf("Missing data from parsing:\n%q", config)
	}

	// Now save it and make sure it shows up in new form
	configStr := saveConfigAndValidateNewFormat(c, config, tmpHome)

	expConfStr := `{
	"auths": {
		"https://index.docker.io/v1/": {
			"auth": "am9lam9lOmhlbGxv",
			"email": "user@example.com"
		}
	}
}`

	if configStr != expConfStr {
		c.Fatalf("Should have save in new form: \n'%s'\n not \n'%s'\n", configStr, expConfStr)
	}
}

func (s *DockerSuite) TestNewJson(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	fn := filepath.Join(tmpHome, ConfigFileName)
	js := ` { "auths": { "https://index.docker.io/v1/": { "auth": "am9lam9lOmhlbGxv" } } }`
	if err := ioutil.WriteFile(fn, []byte(js), 0600); err != nil {
		c.Fatal(err)
	}

	config, err := Load(tmpHome)
	if err != nil {
		c.Fatalf("Failed loading on empty json file: %q", err)
	}

	ac := config.AuthConfigs["https://index.docker.io/v1/"]
	if ac.Username != "joejoe" || ac.Password != "hello" {
		c.Fatalf("Missing data from parsing:\n%q", config)
	}

	// Now save it and make sure it shows up in new form
	configStr := saveConfigAndValidateNewFormat(c, config, tmpHome)

	expConfStr := `{
	"auths": {
		"https://index.docker.io/v1/": {
			"auth": "am9lam9lOmhlbGxv"
		}
	}
}`

	if configStr != expConfStr {
		c.Fatalf("Should have save in new form: \n%s\n not \n%s", configStr, expConfStr)
	}
}

func (s *DockerSuite) TestNewJsonNoEmail(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	fn := filepath.Join(tmpHome, ConfigFileName)
	js := ` { "auths": { "https://index.docker.io/v1/": { "auth": "am9lam9lOmhlbGxv" } } }`
	if err := ioutil.WriteFile(fn, []byte(js), 0600); err != nil {
		c.Fatal(err)
	}

	config, err := Load(tmpHome)
	if err != nil {
		c.Fatalf("Failed loading on empty json file: %q", err)
	}

	ac := config.AuthConfigs["https://index.docker.io/v1/"]
	if ac.Username != "joejoe" || ac.Password != "hello" {
		c.Fatalf("Missing data from parsing:\n%q", config)
	}

	// Now save it and make sure it shows up in new form
	configStr := saveConfigAndValidateNewFormat(c, config, tmpHome)

	expConfStr := `{
	"auths": {
		"https://index.docker.io/v1/": {
			"auth": "am9lam9lOmhlbGxv"
		}
	}
}`

	if configStr != expConfStr {
		c.Fatalf("Should have save in new form: \n%s\n not \n%s", configStr, expConfStr)
	}
}

func (s *DockerSuite) TestJsonWithPsFormat(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	fn := filepath.Join(tmpHome, ConfigFileName)
	js := `{
		"auths": { "https://index.docker.io/v1/": { "auth": "am9lam9lOmhlbGxv", "email": "user@example.com" } },
		"psFormat": "table {{.ID}}\\t{{.Label \"com.docker.label.cpu\"}}"
}`
	if err := ioutil.WriteFile(fn, []byte(js), 0600); err != nil {
		c.Fatal(err)
	}

	config, err := Load(tmpHome)
	if err != nil {
		c.Fatalf("Failed loading on empty json file: %q", err)
	}

	if config.PsFormat != `table {{.ID}}\t{{.Label "com.docker.label.cpu"}}` {
		c.Fatalf("Unknown ps format: %s\n", config.PsFormat)
	}

	// Now save it and make sure it shows up in new form
	configStr := saveConfigAndValidateNewFormat(c, config, tmpHome)
	if !strings.Contains(configStr, `"psFormat":`) ||
		!strings.Contains(configStr, "{{.ID}}") {
		c.Fatalf("Should have save in new form: %s", configStr)
	}
}

// Save it and make sure it shows up in new form
func saveConfigAndValidateNewFormat(c *check.C, config *configfile.ConfigFile, homeFolder string) string {
	if err := config.Save(); err != nil {
		c.Fatalf("Failed to save: %q", err)
	}

	buf, err := ioutil.ReadFile(filepath.Join(homeFolder, ConfigFileName))
	if err != nil {
		c.Fatal(err)
	}
	if !strings.Contains(string(buf), `"auths":`) {
		c.Fatalf("Should have save in new form: %s", string(buf))
	}
	return string(buf)
}

func (s *DockerSuite) TestConfigDir(c *check.C) {
	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	if ConfigDir() == tmpHome {
		c.Fatalf("Expected ConfigDir to be different than %s by default, but was the same", tmpHome)
	}

	// Update configDir
	SetConfigDir(tmpHome)

	if ConfigDir() != tmpHome {
		c.Fatalf("Expected ConfigDir to %s, but was %s", tmpHome, ConfigDir())
	}
}

func (s *DockerSuite) TestConfigFile(c *check.C) {
	configFilename := "configFilename"
	configFile := NewConfigFile(configFilename)

	if configFile.Filename != configFilename {
		c.Fatalf("Expected %s, got %s", configFilename, configFile.Filename)
	}
}

func (s *DockerSuite) TestJsonReaderNoFile(c *check.C) {
	js := ` { "auths": { "https://index.docker.io/v1/": { "auth": "am9lam9lOmhlbGxv", "email": "user@example.com" } } }`

	config, err := LoadFromReader(strings.NewReader(js))
	if err != nil {
		c.Fatalf("Failed loading on empty json file: %q", err)
	}

	ac := config.AuthConfigs["https://index.docker.io/v1/"]
	if ac.Username != "joejoe" || ac.Password != "hello" {
		c.Fatalf("Missing data from parsing:\n%q", config)
	}

}

func (s *DockerSuite) TestOldJsonReaderNoFile(c *check.C) {
	js := `{"https://index.docker.io/v1/":{"auth":"am9lam9lOmhlbGxv","email":"user@example.com"}}`

	config, err := LegacyLoadFromReader(strings.NewReader(js))
	if err != nil {
		c.Fatalf("Failed loading on empty json file: %q", err)
	}

	ac := config.AuthConfigs["https://index.docker.io/v1/"]
	if ac.Username != "joejoe" || ac.Password != "hello" {
		c.Fatalf("Missing data from parsing:\n%q", config)
	}
}

func (s *DockerSuite) TestJsonWithPsFormatNoFile(c *check.C) {
	js := `{
		"auths": { "https://index.docker.io/v1/": { "auth": "am9lam9lOmhlbGxv", "email": "user@example.com" } },
		"psFormat": "table {{.ID}}\\t{{.Label \"com.docker.label.cpu\"}}"
}`
	config, err := LoadFromReader(strings.NewReader(js))
	if err != nil {
		c.Fatalf("Failed loading on empty json file: %q", err)
	}

	if config.PsFormat != `table {{.ID}}\t{{.Label "com.docker.label.cpu"}}` {
		c.Fatalf("Unknown ps format: %s\n", config.PsFormat)
	}

}

func (s *DockerSuite) TestJsonSaveWithNoFile(c *check.C) {
	js := `{
		"auths": { "https://index.docker.io/v1/": { "auth": "am9lam9lOmhlbGxv" } },
		"psFormat": "table {{.ID}}\\t{{.Label \"com.docker.label.cpu\"}}"
}`
	config, err := LoadFromReader(strings.NewReader(js))
	err = config.Save()
	if err == nil {
		c.Fatalf("Expected error. File should not have been able to save with no file name.")
	}

	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatalf("Failed to create a temp dir: %q", err)
	}
	defer os.RemoveAll(tmpHome)

	fn := filepath.Join(tmpHome, ConfigFileName)
	f, _ := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()

	err = config.SaveToWriter(f)
	if err != nil {
		c.Fatalf("Failed saving to file: %q", err)
	}
	buf, err := ioutil.ReadFile(filepath.Join(tmpHome, ConfigFileName))
	if err != nil {
		c.Fatal(err)
	}
	expConfStr := `{
	"auths": {
		"https://index.docker.io/v1/": {
			"auth": "am9lam9lOmhlbGxv"
		}
	},
	"psFormat": "table {{.ID}}\\t{{.Label \"com.docker.label.cpu\"}}"
}`
	if string(buf) != expConfStr {
		c.Fatalf("Should have save in new form: \n%s\nnot \n%s", string(buf), expConfStr)
	}
}

func (s *DockerSuite) TestLegacyJsonSaveWithNoFile(c *check.C) {

	js := `{"https://index.docker.io/v1/":{"auth":"am9lam9lOmhlbGxv","email":"user@example.com"}}`
	config, err := LegacyLoadFromReader(strings.NewReader(js))
	err = config.Save()
	if err == nil {
		c.Fatalf("Expected error. File should not have been able to save with no file name.")
	}

	tmpHome, err := ioutil.TempDir("", "config-test")
	if err != nil {
		c.Fatalf("Failed to create a temp dir: %q", err)
	}
	defer os.RemoveAll(tmpHome)

	fn := filepath.Join(tmpHome, ConfigFileName)
	f, _ := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()

	if err = config.SaveToWriter(f); err != nil {
		c.Fatalf("Failed saving to file: %q", err)
	}
	buf, err := ioutil.ReadFile(filepath.Join(tmpHome, ConfigFileName))
	if err != nil {
		c.Fatal(err)
	}

	expConfStr := `{
	"auths": {
		"https://index.docker.io/v1/": {
			"auth": "am9lam9lOmhlbGxv",
			"email": "user@example.com"
		}
	}
}`

	if string(buf) != expConfStr {
		c.Fatalf("Should have save in new form: \n%s\n not \n%s", string(buf), expConfStr)
	}
}
