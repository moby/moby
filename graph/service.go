package graph

import (
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
)

// lookupRaw looks up an image by name in a TagStore and returns the raw JSON
// describing the image.
func (s *TagStore) lookupRaw(name string) ([]byte, error) {
	image, err := s.LookupImage(name)
	if err != nil || image == nil {
		return nil, fmt.Errorf("No such image %s", name)
	}

	imageInspectRaw, err := s.graph.RawJSON(image.ID)
	if err != nil {
		return nil, err
	}

	return imageInspectRaw, nil
}

// Lookup looks up an image by name in a TagStore and returns it as an
// ImageInspect structure.
func (s *TagStore) Lookup(name string) (*types.ImageInspect, error) {
	image, err := s.LookupImage(name)
	if err != nil || image == nil {
		return nil, fmt.Errorf("No such image: %s", name)
	}

	imageInspect := &types.ImageInspect{
		ID:              image.ID,
		Parent:          image.Parent,
		Comment:         image.Comment,
		Created:         image.Created.Format(time.RFC3339Nano),
		Container:       image.Container,
		ContainerConfig: &image.ContainerConfig,
		DockerVersion:   image.DockerVersion,
		Author:          image.Author,
		Config:          image.Config,
		Architecture:    image.Architecture,
		Os:              image.OS,
		Size:            image.Size,
		VirtualSize:     s.graph.GetParentsSize(image) + image.Size,
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
