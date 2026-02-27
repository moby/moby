package load

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
	"github.com/moby/term"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
)

const frozenImgDir = "/docker-frozen-images"

// FrozenImagesLinux loads the frozen image set for the integration suite.
func FrozenImagesLinux(ctx context.Context, apiClient client.APIClient, images ...string) error {
	ctx, span := otel.Tracer("").Start(ctx, "LoadFrozenImages")
	defer span.End()

	var loadImages []struct{ srcName, destName string }
	for _, img := range images {
		if !imageExists(ctx, apiClient, img) {
			srcName := img
			// hello-world:latest gets re-tagged as hello-world:frozen.
			if img == "hello-world:frozen" {
				srcName = "hello-world:latest"
			}
			loadImages = append(loadImages, struct{ srcName, destName string }{
				srcName:  srcName,
				destName: img,
			})
		}
	}
	if len(loadImages) == 0 {
		return nil
	}

	if fi, err := os.Stat(frozenImgDir); err != nil || !fi.IsDir() {
		return errors.Wrapf(err, "error checking frozen images directory %s", frozenImgDir)
	}
	if err := loadFrozenImages(ctx, apiClient); err != nil {
		return err
	}

	for _, img := range loadImages {
		if img.srcName != img.destName {
			if _, err := apiClient.ImageTag(ctx, client.ImageTagOptions{Source: img.srcName, Target: img.destName}); err != nil {
				return errors.Wrapf(err, "failed to tag %s as %s", img.srcName, img.destName)
			}
			if _, err := apiClient.ImageRemove(ctx, img.srcName, client.ImageRemoveOptions{}); err != nil {
				return errors.Wrapf(err, "failed to remove %s", img.srcName)
			}
		}
	}
	return nil
}

func imageExists(ctx context.Context, client client.APIClient, name string) bool {
	ctx, span := otel.Tracer("").Start(ctx, "check image exists: "+name)
	defer span.End()
	_, err := client.ImageInspect(ctx, name)
	if err != nil {
		span.RecordError(err)
	}
	return err == nil
}

func loadFrozenImages(ctx context.Context, apiClient client.APIClient) error {
	frozenImages, _ := os.ReadDir(frozenImgDir)
	for _, frozenImage := range frozenImages {
		if frozenImage.IsDir() {
			continue
		}
		fi, err := frozenImage.Info()
		if err != nil {
			return err
		}
		err = func(tarfile fs.FileInfo) error {
			reader, err := os.OpenFile(filepath.Join(frozenImgDir, tarfile.Name()), os.O_RDONLY, 0o644)
			if err != nil {
				return err
			}
			defer reader.Close()

			resp, err := apiClient.ImageLoad(ctx, reader, client.ImageLoadWithQuiet(true))
			if err != nil {
				return errors.Wrap(err, "failed to load frozen images")
			}
			defer resp.Close()

			fd, isTerminal := term.GetFdInfo(os.Stdout)
			return jsonmessage.DisplayJSONMessagesStream(resp, os.Stdout, fd, isTerminal, nil)
		}(fi)
		if err != nil {
			return err
		}
	}
	return nil
}
