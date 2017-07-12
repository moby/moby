package container

import (
	"context"
	"strconv"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/require"
)

func TestAPICreateWithNotExistImage(t *testing.T) {
	defer setupTest(t)()
	clt := createClient(t)

	testCases := []struct {
		image         string
		expectedError string
	}{
		{
			image:         "test456:v1",
			expectedError: "No such image: test456:v1",
		},
		{
			image:         "test456",
			expectedError: "No such image: test456",
		},
		{
			image:         "sha256:0cb40641836c461bc97c793971d84d758371ed682042457523e4ae701efeaaaa",
			expectedError: "No such image: sha256:0cb40641836c461bc97c793971d84d758371ed682042457523e4ae701efeaaaa",
		},
	}

	for index, tc := range testCases {
		tc := tc
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			_, err := clt.ContainerCreate(context.Background(),
				&container.Config{
					Image: tc.image,
				},
				&container.HostConfig{},
				&network.NetworkingConfig{},
				"foo",
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestAPICreateEmptyEnv(t *testing.T) {
	defer setupTest(t)()
	clt := createClient(t)

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
			_, err := clt.ContainerCreate(context.Background(),
				&container.Config{
					Image: "busybox",
					Env:   []string{tc.env},
				},
				&container.HostConfig{},
				&network.NetworkingConfig{},
				"foo",
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expectedError)
		})
	}
}
