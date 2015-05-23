package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/parsers"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/api"
	"github.com/docker/libnetwork/client"
	"github.com/gorilla/mux"
)

var (
	// DefaultHTTPHost is used if only port is provided to -H flag e.g. docker -d -H tcp://:8080
	DefaultHTTPHost = "127.0.0.1"
	// DefaultHTTPPort is the default http port used by dnet
	DefaultHTTPPort = 2385
	// DefaultUnixSocket exported
	DefaultUnixSocket = "/var/run/dnet.sock"
)

func main() {
	_, stdout, stderr := term.StdStreams()
	logrus.SetOutput(stderr)

	err := dnetCommand(stdout, stderr)
	if err != nil {
		os.Exit(1)
	}
}

func dnetCommand(stdout, stderr io.Writer) error {
	flag.Parse()

	if *flHelp {
		flag.Usage()
		return nil
	}

	if *flLogLevel != "" {
		lvl, err := logrus.ParseLevel(*flLogLevel)
		if err != nil {
			fmt.Fprintf(stderr, "Unable to parse logging level: %s\n", *flLogLevel)
			return err
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	if *flDebug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if *flHost == "" {
		defaultHost := os.Getenv("DNET_HOST")
		if defaultHost == "" {
			// TODO : Add UDS support
			defaultHost = fmt.Sprintf("tcp://%s:%d", DefaultHTTPHost, DefaultHTTPPort)
		}
		*flHost = defaultHost
	}

	dc, err := newDnetConnection(*flHost)
	if err != nil {
		if *flDaemon {
			logrus.Error(err)
		} else {
			fmt.Fprint(stderr, err)
		}
		return err
	}

	if *flDaemon {
		err := dc.dnetDaemon()
		if err != nil {
			logrus.Errorf("dnet Daemon exited with an error : %v", err)
		}
		return err
	}

	cli := client.NewNetworkCli(stdout, stderr, dc.httpCall)
	if err := cli.Cmd("dnet", flag.Args()...); err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}
	return nil
}

type dnetConnection struct {
	// proto holds the client protocol i.e. unix.
	proto string
	// addr holds the client address.
	addr string
}

func (d *dnetConnection) dnetDaemon() error {
	controller, err := libnetwork.New()
	if err != nil {
		fmt.Println("Error starting dnetDaemon :", err)
		return err
	}
	httpHandler := api.NewHTTPHandler(controller)
	r := mux.NewRouter().StrictSlash(false)
	post := r.PathPrefix("/{.*}/networks").Subrouter()
	post.Methods("GET", "PUT", "POST", "DELETE").HandlerFunc(httpHandler)
	return http.ListenAndServe(d.addr, r)
}

func newDnetConnection(val string) (*dnetConnection, error) {
	url, err := parsers.ParseHost(DefaultHTTPHost, DefaultUnixSocket, val)
	if err != nil {
		return nil, err
	}
	protoAddrParts := strings.SplitN(url, "://", 2)
	if len(protoAddrParts) != 2 {
		return nil, fmt.Errorf("bad format, expected tcp://ADDR")
	}
	if strings.ToLower(protoAddrParts[0]) != "tcp" {
		return nil, fmt.Errorf("dnet currently only supports tcp transport")
	}

	return &dnetConnection{protoAddrParts[0], protoAddrParts[1]}, nil
}

func (d *dnetConnection) httpCall(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
	var in io.Reader
	in, err := encodeData(data)
	if err != nil {
		return nil, -1, err
	}

	req, err := http.NewRequest(method, fmt.Sprintf("/dnet%s", path), in)
	if err != nil {
		return nil, -1, err
	}

	setupRequestHeaders(method, data, req, headers)

	req.URL.Host = d.addr
	req.URL.Scheme = "http"

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	statusCode := -1
	if resp != nil {
		statusCode = resp.StatusCode
	}
	if err != nil {
		return nil, statusCode, fmt.Errorf("error when trying to connect: %v", err)
	}

	if statusCode < 200 || statusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, statusCode, err
		}
		return nil, statusCode, fmt.Errorf("error : %s", bytes.TrimSpace(body))
	}

	return resp.Body, statusCode, nil
}

func setupRequestHeaders(method string, data interface{}, req *http.Request, headers map[string][]string) {
	if data != nil {
		if headers == nil {
			headers = make(map[string][]string)
		}
		headers["Content-Type"] = []string{"application/json"}
	}

	expectedPayload := (method == "POST" || method == "PUT")

	if expectedPayload && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "text/plain")
	}

	if headers != nil {
		for k, v := range headers {
			req.Header[k] = v
		}
	}
}

func encodeData(data interface{}) (*bytes.Buffer, error) {
	params := bytes.NewBuffer(nil)
	if data != nil {
		if err := json.NewEncoder(params).Encode(data); err != nil {
			return nil, err
		}
	}
	return params, nil
}
