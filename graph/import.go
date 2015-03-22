package graph

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/progressreader"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/utils"
)

func (s *TagStore) CmdImport(job *engine.Job) error {
	if n := len(job.Args); n != 2 && n != 3 {
		return fmt.Errorf("Usage: %s SRC REPO [TAG]", job.Name)
	}
	var (
		src          = job.Args[0]
		repo         = job.Args[1]
		tag          string
		sf           = streamformatter.NewStreamFormatter(job.GetenvBool("json"))
		archive      archive.ArchiveReader
		resp         *http.Response
		stdoutBuffer = bytes.NewBuffer(nil)
		newConfig    runconfig.Config
	)
	if len(job.Args) > 2 {
		tag = job.Args[2]
	}

	if src == "-" {
		if valid, err := validImageTar(job.Stdin); !valid && err != nil {
			return err
		}

		archive = job.Stdin
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
		job.Stdout.Write(sf.FormatStatus("", "Downloading from %s", u))
		resp, err = utils.Download(u.String())
		if err != nil {
			return err
		}
		progressReader := progressreader.New(progressreader.Config{
			In:        resp.Body,
			Out:       job.Stdout,
			Formatter: sf,
			Size:      int(resp.ContentLength),
			NewLines:  true,
			ID:        "",
			Action:    "Importing",
		})
		defer progressReader.Close()
		archive = progressReader
	}

	buildConfigJob := job.Eng.Job("build_config")
	buildConfigJob.Stdout.Add(stdoutBuffer)
	buildConfigJob.Setenv("changes", job.Getenv("changes"))
	// FIXME this should be remove when we remove deprecated config param
	buildConfigJob.Setenv("config", job.Getenv("config"))

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
	job.Stdout.Write(sf.FormatStatus("", img.ID))
	logID := img.ID
	if tag != "" {
		logID = utils.ImageReference(logID, tag)
	}
	if err = job.Eng.Job("log", "import", logID, "").Run(); err != nil {
		logrus.Errorf("Error logging event 'import' for %s: %s", logID, err)
	}
	return nil
}

func validImageTar(reader io.Reader) (bool, error) {
	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
		if hdr.Name == "repositories" {
			return false, fmt.Errorf("Looks like you are trying to import a previously saved image. Did you mean to use docker load instead?")
		}
	}

	return true, nil
}
