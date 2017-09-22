package image

import (
	"context"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// Save saves an image to a tarball names by path
func Save(client client.APIClient, path, image string) error {
	ctx := context.Background()
	responseReader, err := client.ImageSave(ctx, []string{image})
	if err != nil {
		return err
	}
	defer responseReader.Close()
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, responseReader)
	return err
}

// Load loads an image from a tarball named by path
func Load(client client.APIClient, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	quiet := true
	ctx := context.Background()
	response, err := client.ImageLoad(ctx, file, quiet)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return nil
}

// Import imports the contents of a tarball named by path
func Import(client client.APIClient, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	options := types.ImageImportOptions{}
	ref := ""
	source := types.ImageImportSource{
		Source:     file,
		SourceName: "-",
	}
	ctx := context.Background()
	responseReader, err := client.ImageImport(ctx, source, ref, options)
	if err != nil {
		return err
	}
	defer responseReader.Close()
	return nil
}
