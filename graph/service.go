package graph

import (
	"fmt"
	"io"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/engine"
)

func (s *TagStore) Install(eng *engine.Engine) error {
	for name, handler := range map[string]engine.Handler{
		"image_inspect": s.CmdLookup,
		"image_export":  s.CmdImageExport,
		"viz":           s.CmdViz,
		"load":          s.CmdLoad,
		"push":          s.CmdPush,
	} {
		if err := eng.Register(name, handler); err != nil {
			return fmt.Errorf("Could not register %q: %v", name, err)
		}
	}
	return nil
}

// CmdLookup return an image encoded in JSON
func (s *TagStore) CmdLookup(job *engine.Job) error {
	if len(job.Args) != 1 {
		return fmt.Errorf("usage: %s NAME", job.Name)
	}
	name := job.Args[0]
	if image, err := s.LookupImage(name); err == nil && image != nil {
		if job.GetenvBool("raw") {
			b, err := image.RawJson()
			if err != nil {
				return err
			}
			job.Stdout.Write(b)
			return nil
		}

		out := &engine.Env{}
		out.SetJson("Id", image.ID)
		out.SetJson("Parent", image.Parent)
		out.SetJson("Comment", image.Comment)
		out.SetAuto("Created", image.Created)
		out.SetJson("Container", image.Container)
		out.SetJson("ContainerConfig", image.ContainerConfig)
		out.Set("DockerVersion", image.DockerVersion)
		out.SetJson("Author", image.Author)
		out.SetJson("Config", image.Config)
		out.Set("Architecture", image.Architecture)
		out.Set("Os", image.OS)
		out.SetInt64("Size", image.Size)
		out.SetInt64("VirtualSize", image.GetParentsSize(0)+image.Size)
		if _, err = out.WriteTo(job.Stdout); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("No such image: %s", name)
}

// ImageTarLayer return the tarLayer of the image
func (s *TagStore) ImageTarLayer(name string, dest io.Writer) error {
	if image, err := s.LookupImage(name); err == nil && image != nil {
		fs, err := image.TarLayer()
		if err != nil {
			return err
		}
		defer fs.Close()

		written, err := io.Copy(dest, fs)
		if err != nil {
			return err
		}
		logrus.Debugf("rendered layer for %s of [%d] size", image.ID, written)
		return nil
	}
	return fmt.Errorf("No such image: %s", name)
}
