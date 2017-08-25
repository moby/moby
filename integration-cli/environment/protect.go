package environment

import (
	"strings"

	"github.com/docker/docker/integration-cli/fixtures/load"
	"github.com/gotestyourself/gotestyourself/icmd"
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

// ProtectImages protects existing images and on linux frozen images from being
// cleaned up at the end of test runs
func ProtectImages(t testingT, testEnv *Execution) {
	images := getExistingImages(t, testEnv)

	if testEnv.DaemonPlatform() == "linux" {
		images = append(images, ensureFrozenImagesLinux(t, testEnv)...)
	}
	testEnv.ProtectImage(t, images...)
}

func getExistingImages(t testingT, testEnv *Execution) []string {
	// TODO: use API instead of cli
	result := icmd.RunCommand(testEnv.dockerBinary, "images", "-f", "dangling=false", "--format", "{{.Repository}}:{{.Tag}}")
	result.Assert(t, icmd.Success)
	return strings.Split(strings.TrimSpace(result.Stdout()), "\n")
}

func ensureFrozenImagesLinux(t testingT, testEnv *Execution) []string {
	images := []string{"busybox:latest", "hello-world:frozen", "debian:jessie"}
	err := load.FrozenImagesLinux(testEnv.DockerBinary(), images...)
	if err != nil {
		result := icmd.RunCommand(testEnv.DockerBinary(), "image", "ls")
		t.Logf(result.String())
		t.Fatalf("%+v", err)
	}
	return images
}
