package runconfig

import (
	df "github.com/dotcloud/docker/pkg/dockerfile"
	"strings"
	"testing"
)

func mkTest(script string, tester func(*Config) bool) func(*testing.T) {
	return func(t *testing.T) {
		var cfg Config
		err := df.ParseScript([]byte(script), &cfg)
		if err != nil {
			t.Fatal(err)
		}
		if ok := tester(&cfg); !ok {
			t.Fatalf("'%s': %#v\n", script, cfg)
		}
	}
}

func TestScriptEnv(t *testing.T) {
	mkTest("env foo bar", func(cfg *Config) bool {
		if strings.Join(cfg.Env, "\n") != "foo=bar" {
			return false
		}
		return true
	})(t)
}

func TestScriptOnBuild(t *testing.T) {
	mkTest("onbuild add . /src\nonbuild run echo hello world", func(cfg *Config) bool {
		if cfg.OnBuild[0] != "add . /src" {
			return false
		}
		if cfg.OnBuild[1] != "run echo hello world" {
			return false
		}
		return true
	})(t)
}

func TestScriptCmdJSON(t *testing.T) {
	mkTest("cmd [\"echo\", \"hello\", \"world\"]", func(cfg *Config) bool {
		if len(cfg.Cmd) != 3 {
			return false
		}
		if cfg.Cmd[0] != "echo" {
			return false
		}
		if cfg.Cmd[1] != "hello" {
			return false
		}
		if cfg.Cmd[2] != "world" {
			return false
		}
		return true
	})(t)
}

func TestScriptCmdPlain(t *testing.T) {
	mkTest("cmd command --arg", func(cfg *Config) bool {
		if len(cfg.Cmd) != 3 {
			return false
		}
		if cfg.Cmd[0] != "/bin/sh" {
			return false
		}
		if cfg.Cmd[1] != "-c" {
			return false
		}
		if cfg.Cmd[2] != "command --arg" {
			return false
		}
		return true
	})(t)
}

func TestScriptEntrypointJSON(t *testing.T) {
	mkTest("entrypoint [\"echo\", \"hello\", \"world\"]", func(cfg *Config) bool {
		if len(cfg.Entrypoint) != 3 {
			return false
		}
		if cfg.Entrypoint[0] != "echo" {
			return false
		}
		if cfg.Entrypoint[1] != "hello" {
			return false
		}
		if cfg.Entrypoint[2] != "world" {
			return false
		}
		return true
	})(t)
}

func TestScriptEntrypointPlain(t *testing.T) {
	mkTest("entrypoint command --arg", func(cfg *Config) bool {
		if len(cfg.Entrypoint) != 3 {
			return false
		}
		if cfg.Entrypoint[0] != "/bin/sh" {
			return false
		}
		if cfg.Entrypoint[1] != "-c" {
			return false
		}
		if cfg.Entrypoint[2] != "command --arg" {
			return false
		}
		return true
	})(t)
}

func TestScriptExpose(t *testing.T) {
	mkTest("expose 8080 80 53/udp", func(cfg *Config) bool {
		if len(cfg.ExposedPorts) != 3 {
			return false
		}
		if _, exists := cfg.ExposedPorts["8080/tcp"]; !exists {
			return false
		}
		if _, exists := cfg.ExposedPorts["80/tcp"]; !exists {
			return false
		}
		if _, exists := cfg.ExposedPorts["53/udp"]; !exists {
			return false
		}
		// Check that deprecated field is not set
		if len(cfg.PortSpecs) != 0 {
			return false
		}
		return true
	})(t)
}

func TestScriptUserNumerical(t *testing.T) {
	mkTest("user 4242", func(cfg *Config) bool {
		if cfg.User != "4242" {
			return false
		}
		return true
	})(t)
}

func TestScriptUserLetters(t *testing.T) {
	mkTest("user solomon", func(cfg *Config) bool {
		if cfg.User != "solomon" {
			return false
		}
		return true
	})(t)
}
