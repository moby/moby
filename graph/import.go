package graph

import (
	"io"
	"net/http"
	"net/url"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

// ImageImportConfig holds configuration to import a image.
type ImageImportConfig struct {
	// Changes are the container changes written to top layer.
	Changes []string
	// InConfig is the input stream containers layered data.
	InConfig io.ReadCloser
	// OutStream is the output stream where the image is written.
	OutStream io.Writer
	// ContainerConfig is the configuration of commit container.
	ContainerConfig *runconfig.Config
}

// Import allows to download image from  a archive.
// If the src is a URL, the content is downloaded from the archive. If the source is '-' then the imageImportConfig.InConfig
// reader will be used to load the image. Once all the layers required are loaded locally, image is then tagged using the tag specified.
func (s *TagStore) Import(src string, repo string, tag string, imageImportConfig *ImageImportConfig) error {
	var (
		sf      = streamformatter.NewJSONStreamFormatter()
		archive archive.ArchiveReader
		resp    *http.Response
	)

	if src == "-" {
		archive = imageImportConfig.InConfig
	} else {
		u, err := url.Parse(src)
		if err != nil {
			return err
		}
		if u.Scheme == "" {
			u.Scheme = "http"
			u.Host = src
			u.Path = ""
		}
		imageImportConfig.OutStream.Write(sf.FormatStatus("", "Downloading from %s", u))
		resp, err = httputils.Download(u.String())
		if err != nil {
			return err
		}
		progressReader := progressreader.New(progressreader.Config{
			In:        resp.Body,
			Out:       imageImportConfig.OutStream,
			Formatter: sf,
			Size:      int(resp.ContentLength),
			NewLines:  true,
			ID:        "",
			Action:    "Importing",
		})
		defer progressReader.Close()
		archive = progressReader
	}

	img, err := s.graph.Create(archive, "", "", "Imported from "+src, "", nil, imageImportConfig.ContainerConfig)
	if err != nil {
		return err
	}
	// Optionally register the image at REPO/TAG
	if repo != "" {
		if err := s.Tag(repo, tag, img.ID, true); err != nil {
			return err
		}
	}
	imageImportConfig.OutStream.Write(sf.FormatStatus("", img.ID))
	logID := img.ID
	if tag != "" {
		logID = utils.ImageReference(logID, tag)
	}

	s.eventsService.Log("import", logID, "")
	return nil
}
