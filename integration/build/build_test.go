package build

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/integration-cli/cli/build/fakecontext"
	"github.com/docker/docker/integration/util/request"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			require.NoError(t, tw.WriteHeader(&tar.Header{
				Name: "Dockerfile",
				Size: int64(len(dockerfile)),
			}))
			_, err := tw.Write(dockerfile)
			require.NoError(t, err)
			require.NoError(t, tw.Close())
			resp, err := client.ImageBuild(ctx, buff, types.ImageBuildOptions{Remove: c.rm, ForceRemove: c.forceRm, NoCache: true})
			require.NoError(t, err)
			defer resp.Body.Close()
			filter, err := buildContainerIdsFilter(resp.Body)
			require.NoError(t, err)
			remainingContainers, err := client.ContainerList(ctx, types.ContainerListOptions{Filters: filter, All: true})
			require.NoError(t, err)
			require.Equal(t, c.numberOfIntermediateContainers, len(remainingContainers), "Expected %v remaining intermediate containers, got %v", c.numberOfIntermediateContainers, len(remainingContainers))
		})
	}
}

func buildContainerIdsFilter(buildOutput io.Reader) (filters.Args, error) {
	const intermediateContainerPrefix = " ---> Running in "
	filter := filters.NewArgs()

	dec := json.NewDecoder(buildOutput)
	for {
		m := jsonmessage.JSONMessage{}
		err := dec.Decode(&m)
		if err == io.EOF {
			return filter, nil
		}
		if err != nil {
			return filter, err
		}
		if ix := strings.Index(m.Stream, intermediateContainerPrefix); ix != -1 {
			filter.Add("id", strings.TrimSpace(m.Stream[ix+len(intermediateContainerPrefix):]))
		}
	}
}

func TestBuildMultiStageParentConfig(t *testing.T) {
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
	resp, err := apiclient.ImageBuild(ctx,
		source.AsTarReader(t),
		types.ImageBuildOptions{
			Remove:      true,
			ForceRemove: true,
			Tags:        []string{"build1"},
		})
	require.NoError(t, err)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
	require.NoError(t, err)

	image, _, err := apiclient.ImageInspectWithRaw(ctx, "build1")
	require.NoError(t, err)

	assert.Equal(t, "/foo/sub2", image.Config.WorkingDir)
	assert.Contains(t, image.Config.Env, "WHO=parent")
}
