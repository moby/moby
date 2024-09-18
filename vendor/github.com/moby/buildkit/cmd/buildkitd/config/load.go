package config

import (
	"io"
	"os"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
)

// Load loads buildkitd config
func Load(r io.Reader) (Config, error) {
	var c Config
	t, err := toml.LoadReader(r)
	if err != nil {
		return c, errors.Wrap(err, "failed to parse config")
	}
	err = t.Unmarshal(&c)
	if err != nil {
		return c, errors.Wrap(err, "failed to parse config")
	}
	return c, nil
}

// LoadFile loads buildkitd config file
func LoadFile(fp string) (Config, error) {
	f, err := os.Open(fp)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, errors.Wrapf(err, "failed to load config from %s", fp)
	}
	defer f.Close()
	return Load(f)
}
