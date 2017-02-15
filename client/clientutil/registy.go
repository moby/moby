package clientutil

import (
	"encoding/base64"
	"encoding/json"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/client/config/configfile"
	"github.com/docker/docker/registry"
)

// ElectAuthServer returns the default registry to use (by asking the daemon)
func ElectAuthServer(ctx context.Context, cli client.APIClient) (string, error) {
	// The daemon `/info` endpoint informs us of the default registry being
	// used. This is essential in cross-platforms environment, where for
	// example a Linux client might be interacting with a Windows daemon, hence
	// the default registry URL might be Windows specific.
	serverAddress := registry.IndexServer
	if info, err := cli.Info(ctx); err != nil {
		// FIXME: clientutil should not require logrus pkg?
		logrus.Warnf("failed to get default registry endpoint from daemon (%v). Using system default: %s", err, serverAddress)
	} else {
		serverAddress = info.IndexServerAddress
	}
	return serverAddress, nil
}

// EncodeAuthToBase64 serializes the auth configuration as JSON base64 payload
func EncodeAuthToBase64(authConfig types.AuthConfig) (string, error) {
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// ResolveAuthConfig is like registry.ResolveAuthConfig, but if using the
// default index, it uses the default index name for the daemon's platform,
// not the client's platform.
func ResolveAuthConfig(ctx context.Context, cli client.APIClient, configFile *configfile.ConfigFile, index *registrytypes.IndexInfo) (types.AuthConfig, error) {
	configKey := index.Name
	if index.Official {
		var err error
		configKey, err = ElectAuthServer(ctx, cli)
		if err != nil {
			return types.AuthConfig{}, err
		}
	}
	a, _ := CredentialsStore(configFile, configKey).Get(configKey)
	return a, nil
}
