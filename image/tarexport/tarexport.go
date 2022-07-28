package tarexport // import "github.com/docker/docker/image/tarexport"

import (
	"github.com/docker/distribution"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
	refstore "github.com/docker/docker/reference"
)

const (
	manifestFileName           = "manifest.json"
	legacyLayerFileName        = "layer.tar"
	legacyConfigFileName       = "json"
	legacyVersionFileName      = "VERSION"
	legacyRepositoriesFileName = "repositories"
)

type manifestItem struct {
	Config       string
	RepoTags     []string
	Layers       []string
	Parent       image.ID                                 `json:",omitempty"`
	LayerSources map[layer.DiffID]distribution.Descriptor `json:",omitempty"`
}

type tarexporter struct {
	is             image.Store
	lss            layer.Store
	rs             refstore.Store
	loggerImgEvent LogImageEvent
}

// LogImageEvent defines function to generate events related to image tar(load and save) operations
type LogImageEvent func(imageID, refName, action string)

// NewTarExporter returns new Exporter for tar packages
func NewTarExporter(is image.Store, lss layer.Store, rs refstore.Store, loggerImgEvent LogImageEvent) image.Exporter {
	return &tarexporter{
		is:             is,
		lss:            lss,
		rs:             rs,
		loggerImgEvent: loggerImgEvent,
	}
}
