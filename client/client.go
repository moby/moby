package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dotcloud/docker/term"
	"github.com/dotcloud/docker/utils"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

var (
	ErrConnectionRefused = errors.New("Can't connect to docker daemon. Is 'docker -d' running on this host?")
)

func New(proto, addr string, version float64) *Client {
	return &Client{
		proto:   proto,
		addr:    addr,
		version: version,
	}
}

type Client struct {
	proto   string
	addr    string
	version float64
}

func (client *Client) Call(method, path string, data interface{}) ([]byte, int, error) {
	var params io.Reader
	if data != nil {
		buf, err := json.Marshal(data)
		if err != nil {
			return nil, -1, err
		}
		params = bytes.NewBuffer(buf)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("/v%g%s", client.version, path), params)
	if err != nil {
		return nil, -1, err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("Docker-Client/%g", client.version))
	req.Host = client.addr
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}
	dial, err := net.Dial(client.proto, client.addr)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, ErrConnectionRefused
		}
		return nil, -1, err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	defer clientconn.Close()
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, -1, ErrConnectionRefused
		}
		return nil, -1, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, -1, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		if len(body) == 0 {
			return nil, resp.StatusCode, fmt.Errorf("Error: %s", http.StatusText(resp.StatusCode))
		}
		return nil, resp.StatusCode, fmt.Errorf("Error: %s", body)
	}
	return body, resp.StatusCode, nil
}

func (client *Client) Stream(method, path string, in io.Reader, out io.Writer, headers map[string][]string) error {
	if (method == "POST" || method == "PUT") && in == nil {
		in = bytes.NewReader([]byte{})
	}
	req, err := http.NewRequest(method, fmt.Sprintf("/v%g%s", client.version, path), in)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("Docker-Client/%g", client.version))
	req.Host = client.addr
	if method == "POST" {
		req.Header.Set("Content-Type", "plain/text")
	}

	if headers != nil {
		for k, v := range headers {
			req.Header[k] = v
		}
	}

	dial, err := net.Dial(client.proto, client.addr)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
		}
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	defer clientconn.Close()
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
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
		return fmt.Errorf("Error: %s", body)
	}

	if utils.MatchesContentType(resp.Header.Get("Content-Type"), "application/json") {
		return utils.DisplayJSONMessagesStream(resp.Body, out)
	} else {
		if _, err := io.Copy(out, resp.Body); err != nil {
			return err
		}
	}
	return nil
}

func (client *Client) Hijack(method, path string, setRawTerminal, isTerminal bool, terminalFd uintptr, in io.ReadCloser, stdout, stderr io.Writer) error {

	req, err := http.NewRequest(method, fmt.Sprintf("/v%g%s", client.version, path), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", fmt.Sprintf("Docker-Client/%g", client.version))
	req.Header.Set("Content-Type", "plain/text")
	req.Host = client.addr

	dial, err := net.Dial(client.proto, client.addr)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Can't connect to docker daemon. Is 'docker -d' running on this host?")
		}
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	defer clientconn.Close()

	// Server hijacks the connection, error 'connection closed' expected
	clientconn.Do(req)

	rwc, br := clientconn.Hijack()
	defer rwc.Close()

	var receiveStdout chan error

	if stdout != nil {
		receiveStdout = utils.Go(func() (err error) {
			// When TTY is ON, use regular copy
			if setRawTerminal {
				_, err = io.Copy(stdout, br)
			} else {
				_, err = utils.StdCopy(stdout, stderr, br)
			}
			utils.Debugf("[hijack] End of stdout")
			return err
		})
	}

	if in != nil && setRawTerminal && isTerminal && os.Getenv("NORAW") == "" {
		oldState, err := term.SetRawTerminal(terminalFd)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(terminalFd, oldState)
	}

	sendStdin := utils.Go(func() error {
		if in != nil {
			io.Copy(rwc, in)
			utils.Debugf("[hijack] End of stdin")
		}
		if tcpc, ok := rwc.(*net.TCPConn); ok {
			if err := tcpc.CloseWrite(); err != nil {
				utils.Debugf("Couldn't send EOF: %s\n", err)
			}
		} else if unixc, ok := rwc.(*net.UnixConn); ok {
			if err := unixc.CloseWrite(); err != nil {
				utils.Debugf("Couldn't send EOF: %s\n", err)
			}
		}
		// Discard errors due to pipe interruption
		return nil
	})

	if stdout != nil {
		if err := <-receiveStdout; err != nil {
			utils.Debugf("Error receiveStdout: %s", err)
			return err
		}
	}

	if !isTerminal {
		if err := <-sendStdin; err != nil {
			utils.Debugf("Error sendStdin: %s", err)
			return err
		}
	}
	return nil
}

func (client *Client) Upload(values *url.Values, in io.Reader, out io.Writer, headers map[string][]string) error {
	req, err := http.NewRequest("POST", fmt.Sprintf("/v%g/build?%s", client.version, values.Encode()), in)
	if err != nil {
		return err
	}
	req.Header = headers
	dial, err := net.Dial(client.proto, client.addr)
	if err != nil {
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	resp, err := clientconn.Do(req)
	defer clientconn.Close()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Check for errors
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return fmt.Errorf("Error: %s", http.StatusText(resp.StatusCode))
		}
		return fmt.Errorf("Error: %s", body)
	}

	// Output the result
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}

	return nil
}
