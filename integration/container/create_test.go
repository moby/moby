package container // import "github.com/docker/docker/integration/container"

import (
	"context"
	"strconv"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/request"
	"github.com/docker/docker/internal/testutil"
	"github.com/gotestyourself/gotestyourself/skip"
)

func TestCreateFailsWhenIdentifierDoesNotExist(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)

	testCases := []struct {
		doc           string
		image         string
		expectedError string
	}{
		{
			doc:           "image and tag",
			image:         "test456:v1",
			expectedError: "No such image: test456:v1",
		},
		{
			doc:           "image no tag",
			image:         "test456",
			expectedError: "No such image: test456",
		},
		{
			doc:           "digest",
			image:         "sha256:0cb40641836c461bc97c793971d84d758371ed682042457523e4ae701efeaaaa",
			expectedError: "No such image: sha256:0cb40641836c461bc97c793971d84d758371ed682042457523e4ae701efeaaaa",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.doc, func(t *testing.T) {
			t.Parallel()
			_, err := client.ContainerCreate(context.Background(),
				&container.Config{Image: tc.image},
				&container.HostConfig{},
				&network.NetworkingConfig{},
				"foo",
			)
			testutil.ErrorContains(t, err, tc.expectedError)
		})
	}
}

func TestCreateWithInvalidEnv(t *testing.T) {
	defer setupTest(t)()
	client := request.NewAPIClient(t)

	testCases := []struct {
		env           string
		expectedError string
	}{
		{
			env:           "",
			expectedError: "invalid environment variable:",
		},
		{
			env:           "=",
			expectedError: "invalid environment variable: =",
		},
		{
			env:           "=foo",
			expectedError: "invalid environment variable: =foo",
		},
	}

	for index, tc := range testCases {
		tc := tc
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			_, err := client.ContainerCreate(context.Background(),
				&container.Config{
					Image: "busybox",
					Env:   []string{tc.env},
				},
				&container.HostConfig{},
				&network.NetworkingConfig{},
				"foo",
			)
			testutil.ErrorContains(t, err, tc.expectedError)
		})
	}
}

// Test case for #30166 (target was not validated)
func TestCreateTmpfsMountsTarget(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")

	defer setupTest(t)()
	client := request.NewAPIClient(t)

	testCases := []struct {
		target        string
		expectedError string
	}{
		{
			target:        ".",
			expectedError: "mount path must be absolute",
		},
		{
			target:        "foo",
			expectedError: "mount path must be absolute",
		},
		{
			target:        "/",
			expectedError: "destination can't be '/'",
		},
		{
			target:        "//",
			expectedError: "destination can't be '/'",
		},
	}

	for _, tc := range testCases {
		_, err := client.ContainerCreate(context.Background(),
			&container.Config{
				Image: "busybox",
			},
			&container.HostConfig{
				Tmpfs: map[string]string{tc.target: ""},
			},
			&network.NetworkingConfig{},
			"",
		)
		testutil.ErrorContains(t, err, tc.expectedError)
	}
}
