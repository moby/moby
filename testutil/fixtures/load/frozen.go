package load

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
	"github.com/moby/term"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const frozenImgDir = "/docker-frozen-images"

// FrozenImagesLinux loads the frozen image set for the integration suite
// If the images are not available locally it will download them
// TODO: This loads whatever is in the frozen image dir, regardless of what
// images were passed in. If the images need to be downloaded, then it will respect
// the passed in images
func FrozenImagesLinux(ctx context.Context, client client.APIClient, images ...string) error {
	ctx, span := otel.Tracer("").Start(ctx, "LoadFrozenImages")
	defer span.End()

	var loadImages []struct{ srcName, destName string }
	for _, img := range images {
		if !imageExists(ctx, client, img) {
			srcName := img
			// hello-world:latest gets re-tagged as hello-world:frozen
			// there are some tests that use hello-world:latest specifically so it pulls
			// the image and hello-world:frozen is used for when we just want a super
			// small image
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
		// everything is loaded, we're done
		return nil
	}

	fi, err := os.Stat(frozenImgDir)
	if err != nil || !fi.IsDir() {
		srcImages := make([]string, 0, len(loadImages))
		for _, img := range loadImages {
			srcImages = append(srcImages, img.srcName)
		}
		if err := pullImages(ctx, client, srcImages); err != nil {
			return errors.Wrap(err, "error pulling image list")
		}
	} else {
		if err := loadFrozenImages(ctx, client); err != nil {
			return err
		}
	}

	for _, img := range loadImages {
		if img.srcName != img.destName {
			if err := client.ImageTag(ctx, img.srcName, img.destName); err != nil {
				return errors.Wrapf(err, "failed to tag %s as %s", img.srcName, img.destName)
			}
			if _, err := client.ImageRemove(ctx, img.srcName, image.RemoveOptions{}); err != nil {
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
			reader, err := os.OpenFile(filepath.Join(frozenImgDir, tarfile.Name()), os.O_RDONLY, 0644)
			if err != nil {
				return err
			}
			defer reader.Close()

			resp, err := apiClient.ImageLoad(ctx, reader, client.ImageLoadWithQuiet(true))
			if err != nil {
				return errors.Wrap(err, "failed to load frozen images")
			}
			defer resp.Body.Close()

			fd, isTerminal := term.GetFdInfo(os.Stdout)
			return jsonmessage.DisplayJSONMessagesStream(resp.Body, os.Stdout, fd, isTerminal, nil)
		}(fi)
		if err != nil {
			return err
		}
	}
	return nil
}

func pullImages(ctx context.Context, client client.APIClient, images []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "error getting path to dockerfile")
	}
	dockerfile := os.Getenv("DOCKERFILE")
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	dockerfilePath := filepath.Join(filepath.Dir(filepath.Clean(cwd)), dockerfile)
	pullRefs, err := readFrozenImageList(ctx, dockerfilePath, images)
	if err != nil {
		return errors.Wrap(err, "error reading frozen image list")
	}

	var wg sync.WaitGroup
	chErr := make(chan error, len(images))
	for tag, ref := range pullRefs {
		wg.Add(1)
		go func(tag, ref string) {
			defer wg.Done()
			if err := pullTagAndRemove(ctx, client, ref, tag); err != nil {
				chErr <- err
				return
			}
		}(tag, ref)
	}
	wg.Wait()
	close(chErr)
	return <-chErr
}

func pullTagAndRemove(ctx context.Context, client client.APIClient, ref string, tag string) (retErr error) {
	ctx, span := otel.Tracer("").Start(ctx, "pull image: "+ref+" with tag: "+tag)
	defer func() {
		if retErr != nil {
			// An error here is a real error for the span, so set the span status
			span.SetStatus(codes.Error, retErr.Error())
		}
		span.End()
	}()

	resp, err := client.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to pull %s", ref)
	}
	defer resp.Close()
	fd, isTerminal := term.GetFdInfo(os.Stdout)
	if err := jsonmessage.DisplayJSONMessagesStream(resp, os.Stdout, fd, isTerminal, nil); err != nil {
		return err
	}

	if err := client.ImageTag(ctx, ref, tag); err != nil {
		return errors.Wrapf(err, "failed to tag %s as %s", ref, tag)
	}
	_, err = client.ImageRemove(ctx, ref, image.RemoveOptions{})
	return errors.Wrapf(err, "failed to remove %s", ref)
}

func readFrozenImageList(ctx context.Context, dockerfilePath string, images []string) (map[string]string, error) {
	f, err := os.Open(dockerfilePath)
	if err != nil {
		return nil, errors.Wrap(err, "error reading dockerfile")
	}
	defer f.Close()
	ls := make(map[string]string)

	span := trace.SpanFromContext(ctx)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.Fields(scanner.Text())
		if len(line) < 3 {
			continue
		}
		if line[0] != "RUN" || line[1] != "./contrib/download-frozen-image-v2.sh" {
			continue
		}

		for scanner.Scan() {
			img := strings.TrimSpace(scanner.Text())
			img = strings.TrimSuffix(img, "\\")
			img = strings.TrimSpace(img)
			split := strings.Split(img, "@")
			if len(split) < 2 {
				break
			}

			for _, i := range images {
				if split[0] == i {
					ls[i] = img
					if span.IsRecording() {
						span.AddEvent("found frozen image", trace.WithAttributes(attribute.String("image", i)))
					}
					break
				}
			}
		}
	}
	return ls, nil
}
