package image

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration-cli/cli/build/fakecontext"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/opencontainers/image-tools/image"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveOCIReferencesConflicts(t *testing.T) {
	skip.IfCondition(t, !testEnv.DaemonInfo.ExperimentalBuild)
	client := testEnv.APIClient()
	ctx := context.Background()

	imgs := []struct {
		tag        string
		dockerfile string
	}{
		{
			tag: "busybox0",
			dockerfile: `FROM busybox
			ENV oci=true`,
		},
		{
			tag: "busybox1",
			dockerfile: `FROM busybox
			ENV oci=true
			ENV test=true`,
		},
	}
	for _, i := range imgs {
		source := fakecontext.New(t, "", fakecontext.WithDockerfile(i.dockerfile))
		defer source.Close()

		resp, err := client.ImageBuild(ctx,
			source.AsTarReader(t),
			types.ImageBuildOptions{
				Tags: []string{i.tag}},
		)
		require.NoError(t, err)
		_, err = io.Copy(ioutil.Discard, resp.Body)
		require.NoError(t, err)
		resp.Body.Close()
	}

	cases := []struct {
		name     string
		toSave   []string
		refs     map[string]string
		format   string
		contains string
		toTag    map[string]string
	}{
		{
			name:     "test that the same tag conflicts",
			toSave:   []string{"busybox0", "busybox1"},
			format:   "oci.v1",
			contains: `unable to include unique references "latest" in OCI image`,
		},
		{
			name:     `test that --ref on just a subset of the images raise a conflict (because busybox1 and busybox are still "latest")`,
			toSave:   []string{"busybox", "busybox0", "busybox1"},
			refs:     map[string]string{"busybox0": "busybox0-latest"},
			format:   "oci.v1",
			contains: `unable to include unique references "latest" in OCI image`,
		},
		{
			name:     "test that saving an non existing image fails in oci.v1",
			toSave:   []string{"notexists"},
			format:   "oci.v1",
			contains: `No such image: notexists`,
		},
		{
			name:     "test that the same tag is invalid even if not latest",
			toSave:   []string{"busybox", "busybox0:12.04", "busybox1:12.04"},
			format:   "oci.v1",
			contains: `unable to include unique references "12.04" in OCI image`,
			toTag:    map[string]string{"busybox0:latest": "busybox0:12.04", "busybox1:latest": "busybox1:12.04"},
		},
		{
			name:     "test in case you have --ref pointing to an actual tag",
			toSave:   []string{"busybox0", "busybox1:12.04"},
			refs:     map[string]string{"busybox0": "12.04"},
			format:   "oci.v1",
			contains: `unable to include unique references "12.04" in OCI image`,
		},
		{
			name:     "test that you cannot save the same image twice with same tag",
			toSave:   []string{"busybox", "busybox0"},
			format:   "oci.v1",
			contains: `unable to include unique references "latest" in OCI image`,
			toTag:    map[string]string{"busybox:latest": "busybox0:latest"},
		},
		{
			name:     "test that invalid references aren't accepted",
			toSave:   []string{"busybox"},
			refs:     map[string]string{"busybox": "invalid:reference"},
			format:   "oci.v1",
			contains: `invalid reference "busybox=invalid:reference"`,
		},
		{
			name:     "test that invalid formats aren't accepted",
			toSave:   []string{"busybox"},
			format:   "unknown",
			contains: `format "unknown" unsupported`,
		},
	}
	for _, c := range cases {
		var err error
		for s, d := range c.toTag {
			err = client.ImageTag(ctx, s, d)
			require.NoError(t, err, c.name)
		}
		_, err = client.ImageSave(ctx, c.toSave, types.ImageSaveOptions{Format: c.format, Refs: c.refs})
		require.Error(t, err, c.name)
		assert.Contains(t, err.Error(), c.contains)

	}
	prune := []string{"busybox0:latest", "busybox0:12.04", "busybox1:latest", "busybox1:12.04"}
	for _, d := range prune {
		_, err := client.ImageRemove(ctx, d, types.ImageRemoveOptions{Force: true})
		require.NoError(t, err)
	}
}

func TestSaveOCIReferences(t *testing.T) {
	skip.IfCondition(t, !testEnv.DaemonInfo.ExperimentalBuild)
	client := testEnv.APIClient()
	ctx := context.Background()

	img, _, err := client.ImageInspectWithRaw(ctx, "busybox:latest")
	require.NoError(t, err)
	busyboxID := strings.Replace(img.ID, "sha256:", "", -1)

	source := fakecontext.New(t, "", fakecontext.WithDockerfile(`FROM busybox
		ENV oci=true`))
	defer source.Close()

	resp, err := client.ImageBuild(ctx,
		source.AsTarReader(t),
		types.ImageBuildOptions{
			Tags: []string{"busybox0"}},
	)
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	cases := []struct {
		name         string
		toSave       []string
		refs         map[string]string
		expectedRefs []string
		toTag        [][2]string
		prune        []string
	}{
		{
			name:         "test that the same tag can be saved using ref",
			toSave:       []string{"busybox:latest", "busybox0:latest"},
			refs:         map[string]string{"busybox0:latest": "busybox0-latest"},
			expectedRefs: []string{"latest", "busybox0-latest"},
		},
		{
			name:         "test save with just an image",
			toSave:       []string{"busybox:latest"},
			expectedRefs: []string{"latest"},
		},
		{
			name:         "test save with 2 tags (same underlying image)",
			toSave:       []string{"busybox:latest", "busybox:notlatest"},
			expectedRefs: []string{"latest", "notlatest"},
			toTag: [][2]string{
				{"busybox:latest", "busybox:notlatest"},
			},
			prune: []string{"busybox:notlatest"},
		},
		{
			name:         "test can save with image id",
			toSave:       []string{busyboxID},
			expectedRefs: []string{busyboxID},
		},
		{
			name:         "saving a repository saves all tags",
			toSave:       []string{"img0"},
			expectedRefs: []string{"latest", "notlatest", "another"},
			toTag: [][2]string{
				{"busybox:latest", "img0"},
				{"img0:latest", "img0:notlatest"},
				{"img0:latest", "img0:another"},
			},
			prune: []string{"img0", "img0:notlatest", "img0:another"},
		},
		{
			name:         "test --ref name:tag=reference",
			toSave:       []string{"busybox:latest", "busybox:notlatest", "img0:notlatest"},
			expectedRefs: []string{"latest", "notlatest", "img0-notlatest-ref"},
			refs:         map[string]string{"img0:notlatest": "img0-notlatest-ref"},
			toTag: [][2]string{
				{"busybox:latest", "img0"},
				{"img0:latest", "img0:notlatest"},
				{"busybox:latest", "busybox:notlatest"},
			},
			prune: []string{"img0", "img0:notlatest", "busybox:notlatest"},
		},
	}
	for _, c := range cases {
		var err error
		for _, d := range c.toTag {
			err = client.ImageTag(ctx, d[0], d[1])
			require.NoError(t, err, c.name)
		}
		reader, err := client.ImageSave(ctx, c.toSave, types.ImageSaveOptions{Format: "oci.v1", Refs: c.refs})
		require.NoError(t, err, c.name)
		defer reader.Close()

		b, err := ioutil.ReadAll(reader)
		require.NoError(t, err, c.name)
		r := bytes.NewReader(b)

		err = image.Validate(r, c.expectedRefs, log.New(os.Stderr, "", 0))
		require.NoError(t, err, c.name)
		for _, d := range c.prune {
			_, err = client.ImageRemove(ctx, d, types.ImageRemoveOptions{})
			require.NoError(t, err, c.name)
		}
	}
}
