package graph

import (
	"github.com/docker/docker/engine"
	"github.com/docker/docker/utils"
)

func (s *TagStore) CmdFlatten(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 3 && n != 4 {
		return job.Errorf("Usage: %s PARENT NAME [TAG]", job.Name)
	}
	var (
		parent = job.Args[0]
		name   = job.Args[1]
		repo   = job.Args[2]
		tag    string
		sf     = utils.NewStreamFormatter(job.GetenvBool("json"))
	)
	if len(job.Args) > 3 {
		tag = job.Args[3]
	}

	if par, err := s.LookupImage(parent); err == nil && par != nil {
		if img, err := s.LookupImage(name); err == nil && img != nil {

			driver := s.graph.Driver()

			fs, err := driver.Diff(img.ID, par.ID)
			if err != nil {
				return job.Error(err)
			}
			defer fs.Close()

			img, err := s.graph.Create(fs, img.Container, par.ID, "Flattened from parent "+par.ID+" to "+img.ID, img.Author, &img.ContainerConfig, img.Config)
			if err != nil {
				return job.Error(err)
			}
			// Optionally register the image at REPO/TAG
			if repo != "" {
				if err := s.Set(repo, tag, img.ID, true); err != nil {
					return job.Error(err)
				}
			}
			job.Stdout.Write(sf.FormatStatus("", img.ID))

			return engine.StatusOK
		}
	}
	return job.Errorf("No such image: %s", name)
}
