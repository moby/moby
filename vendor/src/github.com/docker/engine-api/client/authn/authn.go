package authn

import (
	"net/http"
	"strings"
)

// Logger is an interface for debug and logging callbacks.
type Logger interface {
	Debug(formatted string)
	Info(formatted string)
	Error(formatted string)
}

// ignorelogs is an implementation of Logger that ignores everything it's
// passed.  We use it as the default set of logging callbacks if none are set.
type ignorelogs struct {
}

func (l *ignorelogs) Debug(formatted string) {}
func (l *ignorelogs) Info(formatted string)  {}
func (l *ignorelogs) Error(formatted string) {}

// authResponder is an interface that wraps the scheme,
// authRespond, and authCompleted methods.
//
// At initialization time, an implementation of authResponder should register
// itself by calling registerAuthResponder.
type authResponder interface {
	// Scheme should return the name of the authorization scheme for which
	// the responder should be called.
	scheme() string
	// authRespond, given the authentication header value associated with
	// the scheme that it implements, can decide if the request should be
	// retried.  If it returns true, then the request is retransmitted to
	// the server, presumably because it has added an authentication header
	// which it believes the server will accept.
	authRespond(challenge string, req *http.Request) (bool, error)
	// AuthCompleted, given a (possibly empty) WWW-Authenticate header and
	// a successful response, should decide if the server's reply should be
	// accepted.
	authCompleted(challenge string, resp *http.Response) (bool, error)
}

// authResponderCreators functions create a new authResponder.
var authResponderCreators = []func(logger Logger, authers []interface{}) authResponder{}

// Run through all of the registered responder creators and build a map of
// names-to-responder-instances.
func createAuthResponders(logger Logger, authers []interface{}) map[string]authResponder {
	if logger == nil {
		logger = &ignorelogs{}
	}
	ars := make(map[string]authResponder)
	for _, arc := range authResponderCreators {
		responder := arc(logger, authers)
		if responder != nil {
			scheme := strings.ToLower(responder.scheme())
			ars[scheme] = responder
		}
	}
	return ars
}
