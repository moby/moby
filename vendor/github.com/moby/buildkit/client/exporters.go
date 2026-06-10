package client

import (
	"strings"

	"github.com/pkg/errors"
)

const (
	ExporterImage  = "image"
	ExporterLocal  = "local"
	ExporterTar    = "tar"
	ExporterOCI    = "oci"
	ExporterDocker = "docker"
)

type LocalExporterMode string

const (
	LocalExporterModeCopy   LocalExporterMode = "copy"
	LocalExporterModeDelete LocalExporterMode = "delete"
)

func ParseLocalExporterMode(v string) (LocalExporterMode, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", string(LocalExporterModeCopy):
		return LocalExporterModeCopy, nil
	case string(LocalExporterModeDelete):
		return LocalExporterModeDelete, nil
	default:
		return "", errors.Errorf("invalid local exporter mode %q", v)
	}
}
