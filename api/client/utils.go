package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	gosignal "os/signal"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/registry"
	"github.com/docker/docker/utils"
)

var (
	ErrConnectionRefused = errors.New("Cannot connect to the Docker daemon. Is 'docker -d' running on this host?")
)

func (cli *DockerCli) HTTPClient() *http.Client {
	return &http.Client{Transport: cli.transport}
}

func (cli *DockerCli) encodeData(data interface{}) (*bytes.Buffer, error) {
	params := bytes.NewBuffer(nil)
	if data != nil {
		if env, ok := data.(engine.Env); ok {
			if err := env.Encode(params); err != nil {
				return nil, err
			}
		} else {
			buf, err := json.Marshal(data)
			if err != nil {
				return nil, err
			}
			if _, err := params.Write(buf); err != nil {
				return nil, err
			}
		}
	}
	return params, nil
}

func (cli *DockerCli) call(method, path string, data interface{}, passAuthInfo bool) (io.ReadCloser, int, error) {
	params, err := cli.encodeData(data)
	if err != nil {
		return nil, -1, err
	}
	req, err := http.NewRequest(method, fmt.Sprintf("/v%s%s", api.APIVERSION, path), params)
	if err != nil {
		return nil, -1, err
	}
	if passAuthInfo {
		cli.LoadConfigFile()
		// Resolve the Auth config relevant for this server
		authConfig := cli.configFile.ResolveAuthConfig(registry.IndexServerAddress())
		getHeaders := func(authConfig registry.AuthConfig) (map[string][]string, error) {
			buf, err := json.Marshal(authConfig)
			if err != nil {
				return nil, err
			}
			registryAuthHeader := []string{
				base64.URLEncoding.EncodeToString(buf),
			}
			return map[string][]string{"X-Registry-Auth": registryAuthHeader}, nil
		}
		if headers, err := getHeaders(authConfig); err == nil && headers != nil {
			for k, v := range headers {
				req.Header[k] = v
			}
		}
	}
	req.Header.Set("User-Agent", "Docker-Client/"+dockerversion.VERSION)
	req.URL.Host = cli.addr
	req.URL.Scheme = cli.scheme
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	resp, err := cli.HTTPClient().Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, ErrConnectionRefused
		}

		if cli.tlsConfig == nil {
			return nil, -1, fmt.Errorf("%v. Are you trying to connect to a TLS-enabled daemon without TLS?", err)
		}
		return nil, -1, fmt.Errorf("An error occurred trying to connect: %v", err)

	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, -1, err
		}
		if len(body) == 0 {
			return nil, resp.StatusCode, fmt.Errorf("Error: request returned %s for API route and version %s, check if the server supports the requested API version", http.StatusText(resp.StatusCode), req.URL)
		}
		return nil, resp.StatusCode, fmt.Errorf("Error response from daemon: %s", bytes.TrimSpace(body))
	}

	return resp.Body, resp.StatusCode, nil
}

func (cli *DockerCli) stream(method, path string, in io.Reader, out io.Writer, headers map[string][]string) error {
	return cli.streamHelper(method, path, true, in, out, nil, headers)
}

func (cli *DockerCli) streamHelper(method, path string, setRawTerminal bool, in io.Reader, stdout, stderr io.Writer, headers map[string][]string) error {
	if (method == "POST" || method == "PUT") && in == nil {
		in = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequest(method, fmt.Sprintf("/v%s%s", api.APIVERSION, path), in)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+dockerversion.VERSION)
	req.URL.Host = cli.addr
	req.URL.Scheme = cli.scheme
	if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}

	if headers != nil {
		for k, v := range headers {
			req.Header[k] = v
		}
	}
	resp, err := cli.HTTPClient().Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Cannot connect to the Docker daemon. Is 'docker -d' running on this host?")
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return fmt.Errorf("Error :%s", http.StatusText(resp.StatusCode))
		}
		return fmt.Errorf("Error: %s", bytes.TrimSpace(body))
	}

	if api.MatchesContentType(resp.Header.Get("Content-Type"), "application/json") {
		return utils.DisplayJSONMessagesStream(resp.Body, stdout, cli.outFd, cli.isTerminalOut)
	}
	if stdout != nil || stderr != nil {
		// When TTY is ON, use regular copy
		if setRawTerminal {
			_, err = io.Copy(stdout, resp.Body)
		} else {
			_, err = stdcopy.StdCopy(stdout, stderr, resp.Body)
		}
		log.Debugf("[stream] End of stdout")
		return err
	}
	return nil
}

func (cli *DockerCli) resizeTty(id string, isExec bool) {
	height, width := cli.getTtySize()
	if height == 0 && width == 0 {
		return
	}
	v := url.Values{}
	v.Set("h", strconv.Itoa(height))
	v.Set("w", strconv.Itoa(width))

	path := ""
	if !isExec {
		path = "/containers/" + id + "/resize?"
	} else {
		path = "/exec/" + id + "/resize?"
	}

	if _, _, err := readBody(cli.call("POST", path+v.Encode(), nil, false)); err != nil {
		log.Debugf("Error resize: %s", err)
	}
}

func waitForExit(cli *DockerCli, containerId string) (int, error) {
	stream, _, err := cli.call("POST", "/containers/"+containerId+"/wait", nil, false)
	if err != nil {
		return -1, err
	}

	var out engine.Env
	if err := out.Decode(stream); err != nil {
		return -1, err
	}
	return out.GetInt("StatusCode"), nil
}

// getExitCode perform an inspect on the container. It returns
// the running state and the exit code.
func getExitCode(cli *DockerCli, containerId string) (bool, int, error) {
	stream, _, err := cli.call("GET", "/containers/"+containerId+"/json", nil, false)
	if err != nil {
		// If we can't connect, then the daemon probably died.
		if err != ErrConnectionRefused {
			return false, -1, err
		}
		return false, -1, nil
	}

	var result engine.Env
	if err := result.Decode(stream); err != nil {
		return false, -1, err
	}

	state := result.GetSubEnv("State")
	return state.GetBool("Running"), state.GetInt("ExitCode"), nil
}

// getExecExitCode perform an inspect on the exec command. It returns
// the running state and the exit code.
func getExecExitCode(cli *DockerCli, execId string) (bool, int, error) {
	stream, _, err := cli.call("GET", "/exec/"+execId+"/json", nil, false)
	if err != nil {
		// If we can't connect, then the daemon probably died.
		if err != ErrConnectionRefused {
			return false, -1, err
		}
		return false, -1, nil
	}

	var result engine.Env
	if err := result.Decode(stream); err != nil {
		return false, -1, err
	}

	return result.GetBool("Running"), result.GetInt("ExitCode"), nil
}

func (cli *DockerCli) monitorTtySize(id string, isExec bool) error {
	cli.resizeTty(id, isExec)

	sigchan := make(chan os.Signal, 1)
	gosignal.Notify(sigchan, signal.SIGWINCH)
	go func() {
		for _ = range sigchan {
			cli.resizeTty(id, isExec)
		}
	}()
	return nil
}

func (cli *DockerCli) getTtySize() (int, int) {
	if !cli.isTerminalOut {
		return 0, 0
	}
	ws, err := term.GetWinsize(cli.outFd)
	if err != nil {
		log.Debugf("Error getting size: %s", err)
		if ws == nil {
			return 0, 0
		}
	}
	return int(ws.Height), int(ws.Width)
}

func readBody(stream io.ReadCloser, statusCode int, err error) ([]byte, int, error) {
	if stream != nil {
		defer stream.Close()
	}
	if err != nil {
		return nil, statusCode, err
	}
	body, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, -1, err
	}
	return body, statusCode, nil
}
