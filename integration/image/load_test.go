package image

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/gotestyourself/gotestyourself/skip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadOCI(t *testing.T) {
	skip.IfCondition(t, !testEnv.DaemonInfo.ExperimentalBuild)
	client := testEnv.APIClient()
	ctx := context.Background()

	reader, err := client.ImageSave(ctx, []string{"busybox:latest"}, types.ImageSaveOptions{
		Format: "oci.v1"})
	require.NoError(t, err)
	defer reader.Close()

	resp, err := client.ImageLoad(ctx, reader, types.ImageLoadOptions{Name: "busybox"})
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(b), "Loaded image: busybox:latest")
}
