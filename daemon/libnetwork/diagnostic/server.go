package diagnostic

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/caller"
)

// Server when the debug is enabled exposes a
// This data structure is protected by the Agent mutex so does not require and additional mutex here
type Server struct {
	mu       sync.Mutex
	enable   bool
	srv      *http.Server
	port     int
	mux      *http.ServeMux
	handlers map[string]http.Handler
}

// New creates a new diagnostic server
func New() *Server {
	s := &Server{
		mux:      http.NewServeMux(),
		handlers: make(map[string]http.Handler),
	}
	s.HandleFunc("/", notImplemented)
	s.HandleFunc("/help", s.help)
	s.HandleFunc("/ready", ready)
	return s
}

// Handle registers the handler for the given pattern,
// replacing any existing handler.
func (s *Server) Handle(pattern string, handler http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.handlers[pattern]; !ok {
		// Register a handler on the mux which allows the underlying handler to
		// be dynamically switched out. The http.ServeMux will panic if one
		// attempts to register a handler for the same pattern twice.
		s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			s.mu.Lock()
			h := s.handlers[pattern]
			s.mu.Unlock()
			h.ServeHTTP(w, r)
		})
	}
	s.handlers[pattern] = handler
}

// HandleFunc registers the handler function for the given pattern,
// replacing any existing handler.
func (s *Server) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	s.Handle(pattern, http.HandlerFunc(handler))
}

// ServeHTTP this is the method called bu the ListenAndServe, and is needed to allow us to
// use our custom mux
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Enable opens a TCP socket to debug the passed network DB
func (s *Server) Enable(ip string, port int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.port = port
	log.G(context.TODO()).WithFields(log.Fields{"port": s.port, "ip": ip}).Warn("Starting network diagnostic server")

	// FIXME(thaJeztah): this check won't allow re-configuring the port on reload.
	if s.enable {
		log.G(context.TODO()).WithFields(log.Fields{"port": s.port, "ip": ip}).Info("Network diagnostic server is already up and running")
		return
	}

	addr := net.JoinHostPort(ip, strconv.Itoa(s.port))
	log.G(context.TODO()).WithFields(log.Fields{"port": s.port, "ip": ip}).Infof("Starting network diagnostic server listening on %s for commands", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Minute, // "G112: Potential Slowloris Attack (gosec)"; not a real concern for our use, so setting a long timeout.
	}
	s.srv = srv
	s.enable = true
	go func(n *Server) {
		// Ignore ErrServerClosed that is returned on the Shutdown call
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.G(context.TODO()).Errorf("ListenAndServe error: %s", err)
			n.mu.Lock()
			defer n.mu.Unlock()
			n.enable = false
		}
	}(s)
}

// Shutdown stop the debug and closes the tcp socket
func (s *Server) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enable {
		return
	}
	if s.srv != nil {
		if err := s.srv.Shutdown(context.Background()); err != nil {
			log.G(context.TODO()).WithError(err).Warn("Error during network diagnostic server shutdown")
		}
		s.srv = nil
	}
	s.enable = false
	log.G(context.TODO()).Info("Network diagnostic server shutdown complete")
}

// Enabled returns true when the debug is enabled
func (s *Server) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enable
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	_, jsonOutput := ParseHTTPFormOptions(r)
	rsp := WrongCommand("not implemented", fmt.Sprintf("URL path: %s no method implemented check /help\n", r.URL.Path))

	// audit logs
	log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	}).Info("command not implemented done")

	_, _ = HTTPReply(w, rsp, jsonOutput)
}

func (s *Server) help(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	_, jsonOutput := ParseHTTPFormOptions(r)

	// audit logs
	log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	}).Info("help done")

	var result strings.Builder
	s.mu.Lock()
	for path := range s.handlers {
		result.WriteString(fmt.Sprintf("%s\n", path))
	}
	s.mu.Unlock()
	_, _ = HTTPReply(w, CommandSucceed(&StringCmd{Info: result.String()}), jsonOutput)
}

func ready(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	_, jsonOutput := ParseHTTPFormOptions(r)

	// audit logs
	log.G(context.TODO()).WithFields(log.Fields{
		"component": "diagnostic",
		"remoteIP":  r.RemoteAddr,
		"method":    caller.Name(0),
		"url":       r.URL.String(),
	}).Info("ready done")
	_, _ = HTTPReply(w, CommandSucceed(&StringCmd{Info: "OK"}), jsonOutput)
}

// DebugHTTPForm helper to print the form url parameters
func DebugHTTPForm(r *http.Request) {
	for k, v := range r.Form {
		log.G(context.TODO()).Debugf("Form[%q] = %q\n", k, v)
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
	v, enableJSON := r.Form["json"]
	var pretty bool
	if len(v) > 0 {
		pretty = v[0] == "pretty"
	}
	return unsafe, &JSONOutput{enable: enableJSON, prettyPrint: pretty}
}

// HTTPReply helper function that takes care of sending the message out
func HTTPReply(w http.ResponseWriter, r *HTTPResult, j *JSONOutput) (int, error) {
	var response []byte
	if j.enable {
		w.Header().Set("Content-Type", "application/json")
		var err error
		if j.prettyPrint {
			response, err = json.MarshalIndent(r, "", "  ")
			if err != nil {
				response, _ = json.MarshalIndent(FailCommand(err), "", "  ") //nolint:errchkjson // ignore "Error return value of `encoding/json.MarshalIndent` is not checked: unsafe type `StringInterface`"
			}
		} else {
			response, err = json.Marshal(r)
			if err != nil {
				response, _ = json.Marshal(FailCommand(err)) //nolint:errchkjson // ignore "Error return value of `encoding/json.MarshalIndent` is not checked: unsafe type `StringInterface`"
			}
		}
	} else {
		response = []byte(r.String())
	}
	return fmt.Fprint(w, string(response))
}
