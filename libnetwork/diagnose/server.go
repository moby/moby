package diagnose

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	stackdump "github.com/docker/docker/pkg/signal"
	"github.com/docker/libnetwork/common"
	"github.com/sirupsen/logrus"
)

// HTTPHandlerFunc TODO
type HTTPHandlerFunc func(interface{}, http.ResponseWriter, *http.Request)

type httpHandlerCustom struct {
	ctx interface{}
	F   func(interface{}, http.ResponseWriter, *http.Request)
}

// ServeHTTP TODO
func (h httpHandlerCustom) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.F(h.ctx, w, r)
}

var diagPaths2Func = map[string]HTTPHandlerFunc{
	"/":          notImplemented,
	"/help":      help,
	"/ready":     ready,
	"/stackdump": stackTrace,
}

// Server when the debug is enabled exposes a
// This data structure is protected by the Agent mutex so does not require and additional mutex here
type Server struct {
	sk                net.Listener
	port              int
	mux               *http.ServeMux
	registeredHanders map[string]bool
	sync.Mutex
}

// New creates a new diagnose server
func New() *Server {
	return &Server{
		registeredHanders: make(map[string]bool),
	}
}

// Init initialize the mux for the http handling and register the base hooks
func (n *Server) Init() {
	n.mux = http.NewServeMux()

	// Register local handlers
	n.RegisterHandler(n, diagPaths2Func)
}

// RegisterHandler allows to register new handlers to the mux and to a specific path
func (n *Server) RegisterHandler(ctx interface{}, hdlrs map[string]HTTPHandlerFunc) {
	n.Lock()
	defer n.Unlock()
	for path, fun := range hdlrs {
		if _, ok := n.registeredHanders[path]; ok {
			continue
		}
		n.mux.Handle(path, httpHandlerCustom{ctx, fun})
		n.registeredHanders[path] = true
	}
}

// EnableDebug opens a TCP socket to debug the passed network DB
func (n *Server) EnableDebug(ip string, port int) {
	n.Lock()
	defer n.Unlock()

	n.port = port

	if n.sk != nil {
		logrus.Info("The server is already up and running")
		return
	}

	logrus.Infof("Starting the server listening on %d for commands", port)
	// Create the socket
	var err error
	n.sk, err = net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		http.Serve(n.sk, n.mux)
	}()
}

// DisableDebug stop the dubug and closes the tcp socket
func (n *Server) DisableDebug() {
	n.Lock()
	defer n.Unlock()
	n.sk.Close()
	n.sk = nil
}

// IsDebugEnable returns true when the debug is enabled
func (n *Server) IsDebugEnable() bool {
	n.Lock()
	defer n.Unlock()
	return n.sk != nil
}

func notImplemented(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, json := ParseHTTPFormOptions(r)
	rsp := WrongCommand("not implemented", fmt.Sprintf("URL path: %s no method implemented check /help\n", r.URL.Path))

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnose", "remoteIP": r.RemoteAddr, "method": common.CallerName(0), "url": r.URL.String()})
	log.Info("command not implemented done")

	HTTPReply(w, rsp, json)
}

func help(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, json := ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnose", "remoteIP": r.RemoteAddr, "method": common.CallerName(0), "url": r.URL.String()})
	log.Info("help done")

	n, ok := ctx.(*Server)
	var result string
	if ok {
		for path := range n.registeredHanders {
			result += fmt.Sprintf("%s\n", path)
		}
		HTTPReply(w, CommandSucceed(&StringCmd{Info: result}), json)
	}
}

func ready(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, json := ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnose", "remoteIP": r.RemoteAddr, "method": common.CallerName(0), "url": r.URL.String()})
	log.Info("ready done")
	HTTPReply(w, CommandSucceed(&StringCmd{Info: "OK"}), json)
}

func stackTrace(ctx interface{}, w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, json := ParseHTTPFormOptions(r)

	// audit logs
	log := logrus.WithFields(logrus.Fields{"component": "diagnose", "remoteIP": r.RemoteAddr, "method": common.CallerName(0), "url": r.URL.String()})
	log.Info("stack trace")

	path, err := stackdump.DumpStacks("/tmp/")
	if err != nil {
		log.WithError(err).Error("failed to write goroutines dump")
		HTTPReply(w, FailCommand(err), json)
	} else {
		log.Info("stack trace done")
		HTTPReply(w, CommandSucceed(&StringCmd{Info: fmt.Sprintf("goroutine stacks written to %s", path)}), json)
	}
}

// DebugHTTPForm helper to print the form url parameters
func DebugHTTPForm(r *http.Request) {
	for k, v := range r.Form {
		logrus.Debugf("Form[%q] = %q\n", k, v)
	}
}

// JSONOutput contains details on JSON output printing
type JSONOutput struct {
	enable      bool
	prettyPrint bool
}

// ParseHTTPFormOptions easily parse the JSON printing options
func ParseHTTPFormOptions(r *http.Request) (bool, *JSONOutput) {
	_, unsafe := r.Form["unsafe"]
	v, json := r.Form["json"]
	var pretty bool
	if len(v) > 0 {
		pretty = v[0] == "pretty"
	}
	return unsafe, &JSONOutput{enable: json, prettyPrint: pretty}
}

// HTTPReply helper function that takes care of sending the message out
func HTTPReply(w http.ResponseWriter, r *HTTPResult, j *JSONOutput) (int, error) {
	var response []byte
	if j.enable {
		var err error
		if j.prettyPrint {
			response, err = json.MarshalIndent(r, "", "  ")
			if err != nil {
				response, _ = json.MarshalIndent(FailCommand(err), "", "  ")
			}
		} else {
			response, err = json.Marshal(r)
			if err != nil {
				response, _ = json.Marshal(FailCommand(err))
			}
		}
	} else {
		response = []byte(r.String())
	}
	return fmt.Fprint(w, string(response))
}
