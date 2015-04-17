package graph

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

type ImageImportConfig struct {
	Changes   []string
	InConfig  io.ReadCloser
	Json      bool
	OutStream io.Writer
	//OutStream WriteFlusher
}

func (s *TagStore) Import(src string, repo string, tag string, imageImportConfig *ImageImportConfig, eng *engine.Engine) error {
	var (
		sf           = streamformatter.NewStreamFormatter(imageImportConfig.Json)
		archive      archive.ArchiveReader
		resp         *http.Response
		stdoutBuffer = bytes.NewBuffer(nil)
		newConfig    runconfig.Config
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

	buildConfigJob := eng.Job("build_config")
	buildConfigJob.Stdout.Add(stdoutBuffer)
	buildConfigJob.SetenvList("changes", imageImportConfig.Changes)
	// FIXME this should be remove when we remove deprecated config param
	//buildConfigJob.Setenv("config", job.Getenv("config"))

	if err := buildConfigJob.Run(); err != nil {
		return err
	}
	if err := json.NewDecoder(stdoutBuffer).Decode(&newConfig); err != nil {
		return err
	}

	img, err := s.graph.Create(archive, "", "", "Imported from "+src, "", nil, &newConfig)
	if err != nil {
		return err
	}
	// Optionally register the image at REPO/TAG
	if repo != "" {
		if err := s.Set(repo, tag, img.ID, true); err != nil {
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
