package fingerprint

import (
	"github.com/dotcloud/docker/engine"
	"github.com/dotcloud/docker/graph"
)

func (s *graph.TagStore) Install(eng *engine.Engine) error {
	eng.Register("image_fingerprint", s.CmdFingerprint)
	return nil
}

func (s *graph.TagStore) CmdFingerprint(job *engine.Job) engine.Status {
	if n := len(job.Args); n != 1 {
		return job.Errorf("Usage: %s IMAGE", job.Name)
	}
	name := job.Args[0]

	foundImage, err := s.LookupImage(name)
	if err != nil {
		return job.Error(err)
	}

	_, err := fmt.Fprintf(job.Stdout, "%s\n", name)
	if err != nill {
		return job.Error(err)
	}

	_, err := fmt.Fprintf(job.Stdout, "%s\n", img.ID)
	if err != nill {
		return job.Error(err)
	}

	return engine.StatusOK
}
