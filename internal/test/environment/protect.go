package environment

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration-cli/fixtures/load"
	"github.com/stretchr/testify/require"
)

type protectedElements struct {
	images map[string]struct{}
}

// ProtectImage adds the specified image(s) to be protected in case of clean
func (e *Execution) ProtectImage(t testingT, images ...string) {
	for _, image := range images {
		e.protectedElements.images[image] = struct{}{}
	}
}

func newProtectedElements() protectedElements {
	return protectedElements{
		images: map[string]struct{}{},
	}
}

// ProtectImages protects existing images and on linux frozen images from being
// cleaned up at the end of test runs
func ProtectImages(t testingT, testEnv *Execution) {
	images := getExistingImages(t, testEnv)

	if testEnv.DaemonInfo.OSType == "linux" {
		images = append(images, ensureFrozenImagesLinux(t, testEnv)...)
	}
	testEnv.ProtectImage(t, images...)
}

func getExistingImages(t require.TestingT, testEnv *Execution) []string {
	client := testEnv.APIClient()
	filter := filters.NewArgs()
	filter.Add("dangling", "false")
	imageList, err := client.ImageList(context.Background(), types.ImageListOptions{
		Filters: filter,
	})
	require.NoError(t, err, "failed to list images")

	images := []string{}
	for _, image := range imageList {
		images = append(images, tagsFromImageSummary(image)...)
	}
	return images
}

func tagsFromImageSummary(image types.ImageSummary) []string {
	result := []string{}
	for _, tag := range image.RepoTags {
		if tag != "<none>:<none>" {
			result = append(result, tag)
		}
	}
	for _, digest := range image.RepoDigests {
		if digest != "<none>@<none>" {
			result = append(result, digest)
		}
	}
	return result
}

func ensureFrozenImagesLinux(t testingT, testEnv *Execution) []string {
	images := []string{"busybox:latest", "hello-world:frozen", "debian:jessie"}
	err := load.FrozenImagesLinux(testEnv.APIClient(), images...)
	if err != nil {
		t.Fatalf("Failed to load frozen images: %s", err)
	}
	return images
}
