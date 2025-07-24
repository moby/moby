package local

import (
	"github.com/pkg/errors"
)

// CreateConfig is used to configure new instances of driver
type CreateConfig struct {
	DisableCompression bool
	MaxFileSize        int64
	MaxFileCount       int
}

func newDefaultConfig() *CreateConfig {
	return &CreateConfig{
		MaxFileSize:        defaultMaxFileSize,
		MaxFileCount:       defaultMaxFileCount,
		DisableCompression: !defaultCompressLogs,
	}
}

func validateConfig(cfg *CreateConfig) error {
	if cfg.MaxFileSize < 0 {
		return errors.New("max size should be a positive number")
	}
	if cfg.MaxFileCount < 0 {
		return errors.New("max file count cannot be less than 0")
	}

	if !cfg.DisableCompression {
		if cfg.MaxFileCount <= 1 {
			return errors.New("compression cannot be enabled when max file count is 1")
		}
	}
	return nil
}
