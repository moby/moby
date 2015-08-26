package graphdriver

import (
	"io"

	"github.com/docker/docker/image"
)

// NOTE: These interfaces are used for implementing specific features of the Windows
// graphdriver implementation.  The current versions are a short-term solution and
// likely to change or possibly be eliminated, so avoid using them outside of the Windows
// graphdriver code.

// ImageRestorer interface allows the implementer to add a custom image to
// the graph and tagstore.
type ImageRestorer interface {
	RestoreCustomImages(tagger Tagger, recorder Recorder) ([]string, error)
}

// Tagger is an interface that exposes the TagStore.Tag function without needing
// to import graph.
type Tagger interface {
	Tag(repoName, tag, imageName string, force bool) error
}

// Recorder is an interface that exposes the Graph.Register and Graph.Exists
// functions without needing to import graph.
type Recorder interface {
	Exists(id string) bool
	Register(img image.Descriptor, layerData io.Reader) error
}
