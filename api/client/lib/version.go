package lib

import (
	"encoding/json"
	"runtime"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/utils"
)

// VersionResponse holds version information for the client and the server
type VersionResponse struct {
	Client *types.Version
	Server *types.Version
}

// ServerOK return true when the client could connect to the docker server
// and parse the information received. It returns false otherwise.
func (v VersionResponse) ServerOK() bool {
	return v.Server == nil
}

// SystemVersion returns information of the docker client and server host.
func (cli *Client) SystemVersion() (VersionResponse, error) {
	client := &types.Version{
		Version:      dockerversion.Version,
		APIVersion:   api.Version,
		GoVersion:    runtime.Version(),
		GitCommit:    dockerversion.GitCommit,
		BuildTime:    dockerversion.BuildTime,
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Experimental: utils.ExperimentalBuild(),
	}

	resp, err := cli.GET("/version", nil, nil)
	if err != nil {
		return VersionResponse{Client: client}, err
	}
	defer ensureReaderClosed(resp)

	var server types.Version
	err = json.NewDecoder(resp.body).Decode(&server)
	if err != nil {
		return VersionResponse{Client: client}, err
	}
	return types.VersionResponse{Client: client, Server: &server}, nil
}
