package graph

import (
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/utils"
)

// Lookup looks up an image by name in a TagStore and returns it as an
// ImageInspect structure.
func (s *TagStore) Lookup(name string) (*types.ImageInspect, error) {
	image, err := s.LookupImage(name)
	if err != nil || image == nil {
		return nil, fmt.Errorf("No such image: %s", name)
	}

	var tags = make([]string, 0)

	s.Lock()
	for repoName, repository := range s.Repositories {
		for ref, id := range repository {
			if id == image.ID {
				imgRef := utils.ImageReference(repoName, ref)
				tags = append(tags, imgRef)
			}
		}
	}
	s.Unlock()

	imageInspect := &types.ImageInspect{
		ID:        image.ID,
		Tags:      tags,
		Parent:    image.Parent,
		Comment:   image.Comment,
		Created:   image.Created.Format(time.RFC3339Nano),
		Container: image.Container,
		ContainerConfig: types.RunConfig{
			Hostname:        image.ContainerConfig.Hostname,
			Domainname:      image.ContainerConfig.Domainname,
			User:            image.ContainerConfig.User,
			AttachStdin:     image.ContainerConfig.AttachStdin,
			AttachStdout:    image.ContainerConfig.AttachStdout,
			AttachStderr:    image.ContainerConfig.AttachStderr,
			ExposedPorts:    map[string]struct{}{},
			PublishService:  image.ContainerConfig.PublishService,
			Tty:             image.ContainerConfig.Tty,
			OpenStdin:       image.ContainerConfig.OpenStdin,
			StdinOnce:       image.ContainerConfig.StdinOnce,
			Env:             image.ContainerConfig.Env,
			Cmd:             image.ContainerConfig.Cmd.Slice(),
			Image:           image.ContainerConfig.Image,
			Volumes:         image.ContainerConfig.Volumes,
			WorkingDir:      image.ContainerConfig.WorkingDir,
			Entrypoint:      image.ContainerConfig.Entrypoint.Slice(),
			NetworkDisabled: image.ContainerConfig.NetworkDisabled,
			MacAddress:      image.ContainerConfig.MacAddress,
			OnBuild:         image.ContainerConfig.OnBuild,
			Labels:          image.ContainerConfig.Labels,
			StopSignal:      image.ContainerConfig.StopSignal,
		},
		DockerVersion: image.DockerVersion,
		Author:        image.Author,
		Config: types.RunConfig{
			Hostname:        image.Config.Hostname,
			Domainname:      image.Config.Domainname,
			User:            image.Config.User,
			AttachStdin:     image.Config.AttachStdin,
			AttachStdout:    image.Config.AttachStdout,
			AttachStderr:    image.Config.AttachStderr,
			ExposedPorts:    map[string]struct{}{},
			PublishService:  image.Config.PublishService,
			Tty:             image.Config.Tty,
			OpenStdin:       image.Config.OpenStdin,
			StdinOnce:       image.Config.StdinOnce,
			Env:             image.Config.Env,
			Cmd:             image.Config.Cmd.Slice(),
			Image:           image.Config.Image,
			Volumes:         image.Config.Volumes,
			WorkingDir:      image.Config.WorkingDir,
			Entrypoint:      image.Config.Entrypoint.Slice(),
			NetworkDisabled: image.Config.NetworkDisabled,
			MacAddress:      image.Config.MacAddress,
			OnBuild:         image.Config.OnBuild,
			Labels:          image.Config.Labels,
			StopSignal:      image.Config.StopSignal,
		},
		Architecture: image.Architecture,
		Os:           image.OS,
		Size:         image.Size,
		VirtualSize:  s.graph.GetParentsSize(image) + image.Size,
	}

	for k, v := range image.ContainerConfig.ExposedPorts {
		imageInspect.ContainerConfig.ExposedPorts[string(k)] = v
	}

	for k, v := range image.Config.ExposedPorts {
		imageInspect.Config.ExposedPorts[string(k)] = v
	}

	imageInspect.GraphDriver.Name = s.graph.driver.String()

	graphDriverData, err := s.graph.driver.GetMetadata(image.ID)
	if err != nil {
		return nil, err
	}
	imageInspect.GraphDriver.Data = graphDriverData
	return imageInspect, nil
}

// ImageTarLayer return the tarLayer of the image
func (s *TagStore) ImageTarLayer(name string, dest io.Writer) error {
	if image, err := s.LookupImage(name); err == nil && image != nil {
		// On Windows, the base layer cannot be exported
		if runtime.GOOS != "windows" || image.Parent != "" {

			fs, err := s.graph.TarLayer(image)
			if err != nil {
				return err
			}
			defer fs.Close()

			written, err := io.Copy(dest, fs)
			if err != nil {
				return err
			}
			logrus.Debugf("rendered layer for %s of [%d] size", image.ID, written)
		}
		return nil
	}
	return fmt.Errorf("No such image: %s", name)
}
