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
	"github.com/docker/docker/pkg/reexec"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/api"
	"github.com/docker/libnetwork/client"
	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/gorilla/mux"
)

const (
	// DefaultHTTPHost is used if only port is provided to -H flag e.g. docker -d -H tcp://:8080
	DefaultHTTPHost = "127.0.0.1"
	// DefaultHTTPPort is the default http port used by dnet
	DefaultHTTPPort = 2385
	// DefaultUnixSocket exported
	DefaultUnixSocket = "/var/run/dnet.sock"
	cfgFileEnv        = "LIBNETWORK_CFG"
	defaultCfgFile    = "/etc/default/libnetwork.toml"
)

func main() {
	if reexec.Init() {
		return
	}

	_, stdout, stderr := term.StdStreams()
	logrus.SetOutput(stderr)

	err := dnetCommand(stdout, stderr)
	if err != nil {
		os.Exit(1)
	}
}

func parseConfig(cfgFile string) (*config.Config, error) {
	if strings.Trim(cfgFile, " ") == "" {
		cfgFile = os.Getenv(cfgFileEnv)
		if strings.Trim(cfgFile, " ") == "" {
			cfgFile = defaultCfgFile
		}
	}
	return config.ParseConfig(cfgFile)
}

func processConfig(cfg *config.Config) []config.Option {
	options := []config.Option{}
	if cfg == nil {
		return options
	}
	dn := "bridge"
	if strings.TrimSpace(cfg.Daemon.DefaultNetwork) != "" {
		dn = cfg.Daemon.DefaultNetwork
	}
	options = append(options, config.OptionDefaultNetwork(dn))

	dd := "bridge"
	if strings.TrimSpace(cfg.Daemon.DefaultDriver) != "" {
		dd = cfg.Daemon.DefaultDriver
	}
	options = append(options, config.OptionDefaultDriver(dd))

	if cfg.Daemon.Labels != nil {
		options = append(options, config.OptionLabels(cfg.Daemon.Labels))
	}
	if strings.TrimSpace(cfg.Datastore.Client.Provider) != "" {
		options = append(options, config.OptionKVProvider(cfg.Datastore.Client.Provider))
	}
	if strings.TrimSpace(cfg.Datastore.Client.Address) != "" {
		options = append(options, config.OptionKVProviderURL(cfg.Datastore.Client.Address))
	}
	return options
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

func createDefaultNetwork(c libnetwork.NetworkController) {
	nw := c.Config().Daemon.DefaultNetwork
	d := c.Config().Daemon.DefaultDriver
	createOptions := []libnetwork.NetworkOption{}
	genericOption := options.Generic{}

	if nw != "" && d != "" {
		// Bridge driver is special due to legacy reasons
		if d == "bridge" {
			genericOption[netlabel.GenericData] = map[string]interface{}{
				"BridgeName":            nw,
				"AllowNonDefaultBridge": "true",
			}
			networkOption := libnetwork.NetworkOptionGeneric(genericOption)
			createOptions = append(createOptions, networkOption)
		}
		_, err := c.NewNetwork(d, nw, createOptions...)
		if err != nil {
			logrus.Errorf("Error creating default network : %s : %v", nw, err)
		}
	}
}

type dnetConnection struct {
	// proto holds the client protocol i.e. unix.
	proto string
	// addr holds the client address.
	addr string
}

func (d *dnetConnection) dnetDaemon() error {
	cfg, err := parseConfig(*flCfgFile)
	var cOptions []config.Option
	if err == nil {
		cOptions = processConfig(cfg)
	}
	controller, err := libnetwork.New(cOptions...)
	if err != nil {
		fmt.Println("Error starting dnetDaemon :", err)
		return err
	}
	createDefaultNetwork(controller)
	httpHandler := api.NewHTTPHandler(controller)
	r := mux.NewRouter().StrictSlash(false)
	post := r.PathPrefix("/{.*}/networks").Subrouter()
	post.Methods("GET", "PUT", "POST", "DELETE").HandlerFunc(httpHandler)
	post = r.PathPrefix("/networks").Subrouter()
	post.Methods("GET", "PUT", "POST", "DELETE").HandlerFunc(httpHandler)
	post = r.PathPrefix("/{.*}/services").Subrouter()
	post.Methods("GET", "PUT", "POST", "DELETE").HandlerFunc(httpHandler)
	post = r.PathPrefix("/services").Subrouter()
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

func (d *dnetConnection) httpCall(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, http.Header, int, error) {
	var in io.Reader
	in, err := encodeData(data)
	if err != nil {
		return nil, nil, -1, err
	}

	req, err := http.NewRequest(method, fmt.Sprintf("%s", path), in)
	if err != nil {
		return nil, nil, -1, err
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
		return nil, nil, statusCode, fmt.Errorf("error when trying to connect: %v", err)
	}

	if statusCode < 200 || statusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, statusCode, err
		}
		return nil, nil, statusCode, fmt.Errorf("error : %s", bytes.TrimSpace(body))
	}

	return resp.Body, resp.Header, statusCode, nil
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
