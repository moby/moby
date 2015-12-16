package lib

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/runconfig"
	"github.com/docker/go-units"
)

var headerRegexp = regexp.MustCompile(`\ADocker/.+\s\((.+)\)\z`)

// ImageBuild sends request to the daemon to build images.
// The Body in the response implement an io.ReadCloser and it's up to the caller to
// close it.
func (cli *Client) ImageBuild(options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	query, err := imageBuildOptionsToQuery(options)
	if err != nil {
		return types.ImageBuildResponse{}, err
	}

	headers := http.Header(make(map[string][]string))
	buf, err := json.Marshal(options.AuthConfigs)
	if err != nil {
		return types.ImageBuildResponse{}, err
	}
	headers.Add("X-Registry-Config", base64.URLEncoding.EncodeToString(buf))
	headers.Set("Content-Type", "application/tar")

	serverResp, err := cli.postRaw("/build", query, options.Context, headers)
	if err != nil {
		return types.ImageBuildResponse{}, err
	}

	osType := getDockerOS(serverResp.header.Get("Server"))

	return types.ImageBuildResponse{
		Body:   serverResp.body,
		OSType: osType,
	}, nil
}

func imageBuildOptionsToQuery(options types.ImageBuildOptions) (url.Values, error) {
	query := url.Values{
		"t": options.Tags,
	}
	if options.SuppressOutput {
		query.Set("q", "1")
	}
	if options.RemoteContext != "" {
		query.Set("remote", options.RemoteContext)
	}
	if options.NoCache {
		query.Set("nocache", "1")
	}
	if options.Remove {
		query.Set("rm", "1")
	} else {
		query.Set("rm", "0")
	}

	if options.ForceRemove {
		query.Set("forcerm", "1")
	}

	if options.PullParent {
		query.Set("pull", "1")
	}

	if !runconfig.IsolationLevel.IsDefault(runconfig.IsolationLevel(options.Isolation)) {
		query.Set("isolation", options.Isolation)
	}

	query.Set("cpusetcpus", options.CPUSetCPUs)
	query.Set("cpusetmems", options.CPUSetMems)
	query.Set("cpushares", strconv.FormatInt(options.CPUShares, 10))
	query.Set("cpuquota", strconv.FormatInt(options.CPUQuota, 10))
	query.Set("cpuperiod", strconv.FormatInt(options.CPUPeriod, 10))
	query.Set("memory", strconv.FormatInt(options.Memory, 10))
	query.Set("memswap", strconv.FormatInt(options.MemorySwap, 10))
	query.Set("cgroupparent", options.CgroupParent)

	if options.ShmSize != "" {
		parsedShmSize, err := units.RAMInBytes(options.ShmSize)
		if err != nil {
			return query, err
		}
		query.Set("shmsize", strconv.FormatInt(parsedShmSize, 10))
	}

	query.Set("dockerfile", options.Dockerfile)

	ulimitsJSON, err := json.Marshal(options.Ulimits)
	if err != nil {
		return query, err
	}
	query.Set("ulimits", string(ulimitsJSON))

	buildArgs := runconfig.ConvertKVStringsToMap(options.BuildArgs)
	buildArgsJSON, err := json.Marshal(buildArgs)
	if err != nil {
		return query, err
	}
	query.Set("buildargs", string(buildArgsJSON))

	return query, nil
}

func getDockerOS(serverHeader string) string {
	var osType string
	matches := headerRegexp.FindStringSubmatch(serverHeader)
	if len(matches) > 0 {
		osType = matches[1]
	}
	return osType
}
