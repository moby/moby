package graph

import (
	"fmt"
	"io"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
)

func (s *TagStore) Install(eng *engine.Engine) error {
	for name, handler := range map[string]engine.Handler{
		"image_tag":      s.CmdTag,
		"tag":            s.CmdTagLegacy, // FIXME merge with "image_tag"
		"image_get":      s.CmdGet,
		"image_inspect":  s.CmdLookup,
		"image_tarlayer": s.CmdTarLayer,
		"image_export":   s.CmdImageExport,
		"history":        s.CmdHistory,
		"images":         s.CmdImages,
		"viz":            s.CmdViz,
		"load":           s.CmdLoad,
		"import":         s.CmdImport,
		"pull":           s.CmdPull,
		"push":           s.CmdPush,
	} {
		if err := eng.Register(name, handler); err != nil {
			return fmt.Errorf("Could not register %q: %v", name, err)
		}
	}
	return nil
}

// CmdGet returns information about an image.
// If the image doesn't exist, an empty object is returned, to allow
// checking for an image's existence.
func (s *TagStore) CmdGet(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("usage: %s NAME", job.Name)
	}
	name := job.Args[0]
	res := &engine.Env{}
	img, err := s.LookupImage(name)
	// Note: if the image doesn't exist, LookupImage returns
	// nil, nil.
	if err != nil {
		return job.Error(err)
	}
	if img != nil {
		// We don't directly expose all fields of the Image objects,
		// to maintain a clean public API which we can maintain over
		// time even if the underlying structure changes.
		// We should have done this with the Image object to begin with...
		// but we didn't, so now we're doing it here.
		//
		// Fields that we're probably better off not including:
		//	- Config/ContainerConfig. Those structs have the same sprawl problem,
		//		so we shouldn't include them wholesale either.
		//	- Comment: initially created to fulfill the "every image is a git commit"
		//		metaphor, in practice people either ignore it or use it as a
		//		generic description field which it isn't. On deprecation shortlist.
		res.SetAuto("Created", img.Created)
		res.Set("Author", img.Author)
		res.Set("Os", img.OS)
		res.Set("Architecture", img.Architecture)
		res.Set("DockerVersion", img.DockerVersion)
		res.Set("Id", img.ID)
		res.Set("Parent", img.Parent)
	}
	res.WriteTo(job.Stdout)
	return engine.StatusOK
}

// CmdLookup return an image encoded in JSON
func (s *TagStore) CmdLookup(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("usage: %s NAME", job.Name)
	}
	name := job.Args[0]
	if image, err := s.LookupImage(name); err == nil && image != nil {
		if job.GetenvBool("raw") {
			b, err := image.RawJson()
			if err != nil {
				return job.Error(err)
			}
			job.Stdout.Write(b)
			return engine.StatusOK
		}

		out := &engine.Env{}
		out.Set("Id", image.ID)
		out.Set("Parent", image.Parent)
		out.Set("Comment", image.Comment)
		out.SetAuto("Created", image.Created)
		out.Set("Container", image.Container)
		out.SetJson("ContainerConfig", image.ContainerConfig)
		out.Set("DockerVersion", image.DockerVersion)
		out.Set("Author", image.Author)
		out.SetJson("Config", image.Config)
		out.Set("Architecture", image.Architecture)
		out.Set("Os", image.OS)
		out.SetInt64("Size", image.Size)
		out.SetInt64("VirtualSize", image.GetParentsSize(0)+image.Size)
		out.Set("Checksum", image.Checksum)
		if _, err = out.WriteTo(job.Stdout); err != nil {
			return job.Error(err)
		}
		return engine.StatusOK
	}
	return job.Errorf("No such image: %s", name)
}

// CmdTarLayer return the tarLayer of the image
func (s *TagStore) CmdTarLayer(job *engine.Job) engine.Status {
	if len(job.Args) != 1 {
		return job.Errorf("usage: %s NAME", job.Name)
	}
	name := job.Args[0]
	if image, err := s.LookupImage(name); err == nil && image != nil {
		fs, err := image.TarLayer()
		if err != nil {
			return job.Error(err)
		}
		defer fs.Close()

		written, err := io.Copy(job.Stdout, fs)
		if err != nil {
			return job.Error(err)
		}
		log.Debugf("rendered layer for %s of [%d] size", image.ID, written)
		return engine.StatusOK
	}
	return job.Errorf("No such image: %s", name)
}
