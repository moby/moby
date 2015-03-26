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
	"github.com/docker/docker/autogen/dockerversion"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/registry"
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

func (cli *DockerCli) clientRequest(method, path string, in io.Reader, headers map[string][]string) (io.ReadCloser, string, int, error) {
	expectedPayload := (method == "POST" || method == "PUT")
	if expectedPayload && in == nil {
		in = bytes.NewReader([]byte{})
	}
	req, err := http.NewRequest(method, fmt.Sprintf("/v%s%s", api.APIVERSION, path), in)
	if err != nil {
		return nil, "", -1, err
	}
	req.Header.Set("User-Agent", "Docker-Client/"+dockerversion.VERSION)
	req.URL.Host = cli.addr
	req.URL.Scheme = cli.scheme

	if headers != nil {
		for k, v := range headers {
			req.Header[k] = v
		}
	}

	if expectedPayload && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "text/plain")
	}

	resp, err := cli.HTTPClient().Do(req)
	statusCode := -1
	if resp != nil {
		statusCode = resp.StatusCode
	}
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, "", statusCode, ErrConnectionRefused
		}

		if cli.tlsConfig == nil {
			return nil, "", statusCode, fmt.Errorf("%v. Are you trying to connect to a TLS-enabled daemon without TLS?", err)
		}
		return nil, "", statusCode, fmt.Errorf("An error occurred trying to connect: %v", err)
	}

	if statusCode < 200 || statusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, "", statusCode, err
		}
		if len(body) == 0 {
			return nil, "", statusCode, fmt.Errorf("Error: request returned %s for API route and version %s, check if the server supports the requested API version", http.StatusText(statusCode), req.URL)
		}
		return nil, "", statusCode, fmt.Errorf("Error response from daemon: %s", bytes.TrimSpace(body))
	}

	return resp.Body, resp.Header.Get("Content-Type"), statusCode, nil
}

func (cli *DockerCli) clientRequestAttemptLogin(method, path string, in io.Reader, out io.Writer, index *registry.IndexInfo, cmdName string) (io.ReadCloser, int, error) {
	cmdAttempt := func(authConfig registry.AuthConfig) (io.ReadCloser, int, error) {
		buf, err := json.Marshal(authConfig)
		if err != nil {
			return nil, -1, err
		}
		registryAuthHeader := []string{
			base64.URLEncoding.EncodeToString(buf),
		}

		// begin the request
		body, contentType, statusCode, err := cli.clientRequest(method, path, in, map[string][]string{
			"X-Registry-Auth": registryAuthHeader,
		})
		if err == nil && out != nil {
			// If we are streaming output, complete the stream since
			// errors may not appear until later.
			err = cli.streamBody(body, contentType, true, out, nil)
		}
		if err != nil {
			// Since errors in a stream appear after status 200 has been written,
			// we may need to change the status code.
			if strings.Contains(err.Error(), "Authentication is required") ||
				strings.Contains(err.Error(), "Status 401") ||
				strings.Contains(err.Error(), "status code 401") {
				statusCode = http.StatusUnauthorized
			}
		}
		return body, statusCode, err
	}

	// Resolve the Auth config relevant for this server
	authConfig := cli.configFile.ResolveAuthConfig(index)
	body, statusCode, err := cmdAttempt(authConfig)
	if statusCode == http.StatusUnauthorized {
		fmt.Fprintf(cli.out, "\nPlease login prior to %s:\n", cmdName)
		if err = cli.CmdLogin(index.GetAuthConfigKey()); err != nil {
			return nil, -1, err
		}
		authConfig = cli.configFile.ResolveAuthConfig(index)
		return cmdAttempt(authConfig)
	}
	return body, statusCode, err
}

func (cli *DockerCli) call(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
	params, err := cli.encodeData(data)
	if err != nil {
		return nil, -1, err
	}

	if data != nil {
		if headers == nil {
			headers = make(map[string][]string)
		}
		headers["Content-Type"] = []string{"application/json"}
	}

	body, _, statusCode, err := cli.clientRequest(method, path, params, headers)
	return body, statusCode, err
}

func (cli *DockerCli) stream(method, path string, in io.Reader, out io.Writer, headers map[string][]string) error {
	return cli.streamHelper(method, path, true, in, out, nil, headers)
}

func (cli *DockerCli) streamHelper(method, path string, setRawTerminal bool, in io.Reader, stdout, stderr io.Writer, headers map[string][]string) error {
	body, contentType, _, err := cli.clientRequest(method, path, in, headers)
	if err != nil {
		return err
	}
	return cli.streamBody(body, contentType, setRawTerminal, stdout, stderr)
}

func (cli *DockerCli) streamBody(body io.ReadCloser, contentType string, setRawTerminal bool, stdout, stderr io.Writer) error {
	defer body.Close()

	if api.MatchesContentType(contentType, "application/json") {
		return jsonmessage.DisplayJSONMessagesStream(body, stdout, cli.outFd, cli.isTerminalOut)
	}
	if stdout != nil || stderr != nil {
		// When TTY is ON, use regular copy
		var err error
		if setRawTerminal {
			_, err = io.Copy(stdout, body)
		} else {
			_, err = stdcopy.StdCopy(stdout, stderr, body)
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

	if _, _, err := readBody(cli.call("POST", path+v.Encode(), nil, nil)); err != nil {
		log.Debugf("Error resize: %s", err)
	}
}

func waitForExit(cli *DockerCli, containerID string) (int, error) {
	stream, _, err := cli.call("POST", "/containers/"+containerID+"/wait", nil, nil)
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
func getExitCode(cli *DockerCli, containerID string) (bool, int, error) {
	stream, _, err := cli.call("GET", "/containers/"+containerID+"/json", nil, nil)
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
func getExecExitCode(cli *DockerCli, execID string) (bool, int, error) {
	stream, _, err := cli.call("GET", "/exec/"+execID+"/json", nil, nil)
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
