package build // import "github.com/docker/docker/integration/build"

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/builder/buildutil"
	"github.com/docker/docker/internal/test/fakecontext"
	"github.com/docker/docker/internal/test/request"
	"gotest.tools/assert"
	is "gotest.tools/assert/cmp"
	"gotest.tools/skip"
)

func TestBuildWithRemoveAndForceRemove(t *testing.T) {
	defer setupTest(t)()
	t.Parallel()
	cases := []struct {
		name                           string
		dockerfile                     string
		numberOfIntermediateContainers int
		rm                             bool
		forceRm                        bool
	}{
		{
			name: "successful build with no removal",
			dockerfile: `FROM busybox
			RUN exit 0
			RUN exit 0`,
			numberOfIntermediateContainers: 2,
			rm:      false,
			forceRm: false,
		},
		{
			name: "successful build with remove",
			dockerfile: `FROM busybox
			RUN exit 0
			RUN exit 0`,
			numberOfIntermediateContainers: 0,
			rm:      true,
			forceRm: false,
		},
		{
			name: "successful build with remove and force remove",
			dockerfile: `FROM busybox
			RUN exit 0
			RUN exit 0`,
			numberOfIntermediateContainers: 0,
			rm:      true,
			forceRm: true,
		},
		{
			name: "failed build with no removal",
			dockerfile: `FROM busybox
			RUN exit 0
			RUN exit 1`,
			numberOfIntermediateContainers: 2,
			rm:      false,
			forceRm: false,
		},
		{
			name: "failed build with remove",
			dockerfile: `FROM busybox
			RUN exit 0
			RUN exit 1`,
			numberOfIntermediateContainers: 1,
			rm:      true,
			forceRm: false,
		},
		{
			name: "failed build with remove and force remove",
			dockerfile: `FROM busybox
			RUN exit 0
			RUN exit 1`,
			numberOfIntermediateContainers: 0,
			rm:      true,
			forceRm: true,
		},
	}

	client := request.NewAPIClient(t)
	ctx := context.Background()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dockerfile := []byte(c.dockerfile)

			buff := bytes.NewBuffer(nil)
			tw := tar.NewWriter(buff)
			assert.NilError(t, tw.WriteHeader(&tar.Header{
				Name: "Dockerfile",
				Size: int64(len(dockerfile)),
			}))
			_, err := tw.Write(dockerfile)
			assert.NilError(t, err)
			assert.NilError(t, tw.Close())
			resp, _ := buildutil.Build(client, buildutil.BuildInput{Context: buff}, types.ImageBuildOptions{Remove: c.rm, ForceRemove: c.forceRm, NoCache: true})
			filter, err := buildContainerIdsFilter(bytes.NewReader(resp.Output))
			assert.NilError(t, err)
			remainingContainers, err := client.ContainerList(ctx, types.ContainerListOptions{Filters: filter, All: true})
			assert.NilError(t, err)
			assert.Equal(t, c.numberOfIntermediateContainers, len(remainingContainers), "Expected %v remaining intermediate containers, got %v", c.numberOfIntermediateContainers, len(remainingContainers))
		})
	}
}

func buildContainerIdsFilter(buildOutput io.Reader) (filters.Args, error) {
	const intermediateContainerPrefix = " ---> Running in "
	filter := filters.NewArgs()

	s := bufio.NewScanner(buildOutput)
	for s.Scan() {
		t := s.Text()
		if ix := strings.Index(t, intermediateContainerPrefix); ix != -1 {
			filter.Add("id", strings.TrimSpace(t[ix+len(intermediateContainerPrefix):]))
		}
	}
	return filter, s.Err()
}

func TestBuildMultiStageParentConfig(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.35"), "broken in earlier versions")
	dockerfile := `
		FROM busybox AS stage0
		ENV WHO=parent
		WORKDIR /foo

		FROM stage0
		ENV WHO=sibling1
		WORKDIR sub1

		FROM stage0
		WORKDIR sub2
	`
	ctx := context.Background()
	source := fakecontext.New(t, "", fakecontext.WithDockerfile(dockerfile))
	defer source.Close()

	apiclient := testEnv.APIClient()
	_, err := buildutil.Build(apiclient, buildutil.BuildInput{Context: source.AsTarReader(t)}, types.ImageBuildOptions{
		Remove:      true,
		ForceRemove: true,
		Tags:        []string{"build1"},
	})
	assert.NilError(t, err)

	image, _, err := apiclient.ImageInspectWithRaw(ctx, "build1")
	assert.NilError(t, err)

	assert.Check(t, is.Equal("/foo/sub2", image.Config.WorkingDir))
	assert.Check(t, is.Contains(image.Config.Env, "WHO=parent"))
}

// Test cases in #36996
func TestBuildLabelWithTargets(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.38"), "test added after 1.38")
	bldName := "build-a"
	testLabels := map[string]string{
		"foo":  "bar",
		"dead": "beef",
	}

	dockerfile := `
		FROM busybox AS target-a
		CMD ["/dev"]
		LABEL label-a=inline-a
		FROM busybox AS target-b
		CMD ["/dist"]
		LABEL label-b=inline-b
		`

	ctx := context.Background()
	source := fakecontext.New(t, "", fakecontext.WithDockerfile(dockerfile))
	defer source.Close()

	apiclient := testEnv.APIClient()
	// For `target-a` build
	_, err := buildutil.Build(apiclient,
		buildutil.BuildInput{Context: source.AsTarReader(t)},
		types.ImageBuildOptions{
			Remove:      true,
			ForceRemove: true,
			Tags:        []string{bldName},
			Labels:      testLabels,
			Target:      "target-a",
		})
	assert.NilError(t, err)

	image, _, err := apiclient.ImageInspectWithRaw(ctx, bldName)
	assert.NilError(t, err)

	testLabels["label-a"] = "inline-a"
	for k, v := range testLabels {
		x, ok := image.Config.Labels[k]
		assert.Assert(t, ok)
		assert.Assert(t, x == v)
	}

	// For `target-b` build
	bldName = "build-b"
	delete(testLabels, "label-a")
	_, err = buildutil.Build(apiclient,
		buildutil.BuildInput{Context: source.AsTarReader(t)},
		types.ImageBuildOptions{
			Remove:      true,
			ForceRemove: true,
			Tags:        []string{bldName},
			Labels:      testLabels,
			Target:      "target-b",
		})
	assert.NilError(t, err)

	image, _, err = apiclient.ImageInspectWithRaw(ctx, bldName)
	assert.NilError(t, err)

	testLabels["label-b"] = "inline-b"
	for k, v := range testLabels {
		x, ok := image.Config.Labels[k]
		assert.Assert(t, ok)
		assert.Assert(t, x == v)
	}
}

func TestBuildWithEmptyLayers(t *testing.T) {
	dockerfile := `
		FROM    busybox
		COPY    1/ /target/
		COPY    2/ /target/
		COPY    3/ /target/
	`
	source := fakecontext.New(t, "",
		fakecontext.WithDockerfile(dockerfile),
		fakecontext.WithFile("1/a", "asdf"),
		fakecontext.WithFile("2/a", "asdf"),
		fakecontext.WithFile("3/a", "asdf"))
	defer source.Close()

	apiclient := testEnv.APIClient()
	_, err := buildutil.Build(apiclient,
		buildutil.BuildInput{Context: source.AsTarReader(t)},
		types.ImageBuildOptions{
			Remove:      true,
			ForceRemove: true,
		})
	assert.NilError(t, err)
}

// TestBuildMultiStageOnBuild checks that ONBUILD commands are applied to
// multiple subsequent stages
// #35652
func TestBuildMultiStageOnBuild(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.33"), "broken in earlier versions")
	defer setupTest(t)()
	// test both metadata and layer based commands as they may be implemented differently
	dockerfile := `FROM busybox AS stage1
ONBUILD RUN echo 'foo' >somefile
ONBUILD ENV bar=baz

FROM stage1
RUN cat somefile # fails if ONBUILD RUN fails

FROM stage1
RUN cat somefile`

	source := fakecontext.New(t, "",
		fakecontext.WithDockerfile(dockerfile))
	defer source.Close()

	apiclient := testEnv.APIClient()
	resp, err := buildutil.Build(apiclient,
		buildutil.BuildInput{Context: source.AsTarReader(t)},
		types.ImageBuildOptions{
			Remove:      true,
			ForceRemove: true,
		})

	assert.NilError(t, err)

	assert.Check(t, is.Contains(string(resp.Output), "Successfully built"))

	image, _, err := apiclient.ImageInspectWithRaw(context.Background(), resp.ID)
	assert.NilError(t, err)
	assert.Check(t, is.Contains(image.Config.Env, "bar=baz"))
}

// #35403 #36122
func TestBuildUncleanTarFilenames(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.37"), "broken in earlier versions")
	defer setupTest(t)()

	dockerfile := `FROM scratch
COPY foo /
FROM scratch
COPY bar /`

	buf := bytes.NewBuffer(nil)
	w := tar.NewWriter(buf)
	writeTarRecord(t, w, "Dockerfile", dockerfile)
	writeTarRecord(t, w, "../foo", "foocontents0")
	writeTarRecord(t, w, "/bar", "barcontents0")
	err := w.Close()
	assert.NilError(t, err)

	apiclient := testEnv.APIClient()
	_, err = buildutil.Build(apiclient,
		buildutil.BuildInput{Context: buf},
		types.ImageBuildOptions{
			Remove:      true,
			ForceRemove: true,
		})
	assert.NilError(t, err)

	// repeat with changed data should not cause cache hits

	buf = bytes.NewBuffer(nil)
	w = tar.NewWriter(buf)
	writeTarRecord(t, w, "Dockerfile", dockerfile)
	writeTarRecord(t, w, "../foo", "foocontents1")
	writeTarRecord(t, w, "/bar", "barcontents1")
	err = w.Close()
	assert.NilError(t, err)

	resp, err := buildutil.Build(apiclient,
		buildutil.BuildInput{Context: buf},
		types.ImageBuildOptions{
			Remove:      true,
			ForceRemove: true,
		})

	assert.NilError(t, err)
	assert.Assert(t, !strings.Contains(string(resp.Output), "Using cache"))
}

// docker/for-linux#135
// #35641
func TestBuildMultiStageLayerLeak(t *testing.T) {
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.37"), "broken in earlier versions")
	defer setupTest(t)()

	// all commands need to match until COPY
	dockerfile := `FROM busybox
WORKDIR /foo
COPY foo .
FROM busybox
WORKDIR /foo
COPY bar .
RUN [ -f bar ]
RUN [ ! -f foo ]
`

	source := fakecontext.New(t, "",
		fakecontext.WithFile("foo", "0"),
		fakecontext.WithFile("bar", "1"),
		fakecontext.WithDockerfile(dockerfile))
	defer source.Close()

	apiclient := testEnv.APIClient()
	resp, err := buildutil.Build(apiclient,
		buildutil.BuildInput{Context: source.AsTarReader(t)},
		types.ImageBuildOptions{
			Remove:      true,
			ForceRemove: true,
		})

	assert.NilError(t, err)

	assert.Check(t, is.Contains(string(resp.Output), "Successfully built"))
}

func writeTarRecord(t *testing.T, w *tar.Writer, fn, contents string) {
	err := w.WriteHeader(&tar.Header{
		Name:     fn,
		Mode:     0600,
		Size:     int64(len(contents)),
		Typeflag: '0',
	})
	assert.NilError(t, err)
	_, err = w.Write([]byte(contents))
	assert.NilError(t, err)
}
