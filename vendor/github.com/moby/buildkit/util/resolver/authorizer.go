package resolver

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/auth"
	remoteserrors "github.com/containerd/containerd/remotes/errors"
	"github.com/moby/buildkit/session"
	sessionauth "github.com/moby/buildkit/session/auth"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type authHandlerNS struct {
	counter int64 // needs to be 64bit aligned for 32bit systems

	mu       sync.Mutex
	handlers map[string]*authHandler
	hosts    map[string][]docker.RegistryHost
	sm       *session.Manager
	g        flightcontrol.Group
}

func newAuthHandlerNS(sm *session.Manager) *authHandlerNS {
	return &authHandlerNS{
		handlers: map[string]*authHandler{},
		hosts:    map[string][]docker.RegistryHost{},
		sm:       sm,
	}
}

func (a *authHandlerNS) get(host string, sm *session.Manager, g session.Group) *authHandler {
	if g != nil {
		if iter := g.SessionIterator(); iter != nil {
			for {
				id := iter.NextSession()
				if id == "" {
					break
				}
				h, ok := a.handlers[host+"/"+id]
				if ok {
					h.lastUsed = time.Now()
					return h
				}
			}
		}
	}

	// link another handler
	for k, h := range a.handlers {
		parts := strings.SplitN(k, "/", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == host {
			if h.authority != nil {
				session, ok, err := sessionauth.VerifyTokenAuthority(host, h.authority, sm, g)
				if err == nil && ok {
					a.handlers[host+"/"+session] = h
					h.lastUsed = time.Now()
					return h
				}
			} else {
				session, username, password, err := sessionauth.CredentialsFunc(sm, g)(host)
				if err == nil {
					if username == h.common.Username && password == h.common.Secret {
						a.handlers[host+"/"+session] = h
						h.lastUsed = time.Now()
						return h
					}
				}
			}
		}
	}

	return nil
}

func (a *authHandlerNS) set(host, session string, h *authHandler) {
	a.handlers[host+"/"+session] = h
}

func (a *authHandlerNS) delete(h *authHandler) {
	for k, v := range a.handlers {
		if v == h {
			delete(a.handlers, k)
		}
	}
}

type dockerAuthorizer struct {
	client *http.Client

	sm       *session.Manager
	session  session.Group
	handlers *authHandlerNS
}

func newDockerAuthorizer(client *http.Client, handlers *authHandlerNS, sm *session.Manager, group session.Group) *dockerAuthorizer {
	return &dockerAuthorizer{
		client:   client,
		handlers: handlers,
		sm:       sm,
		session:  group,
	}
}

// Authorize handles auth request.
func (a *dockerAuthorizer) Authorize(ctx context.Context, req *http.Request) error {
	a.handlers.mu.Lock()
	defer a.handlers.mu.Unlock()

	// skip if there is no auth handler
	ah := a.handlers.get(req.URL.Host, a.sm, a.session)
	if ah == nil {
		return nil
	}

	auth, err := ah.authorize(ctx, a.sm, a.session)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", auth)
	return nil
}

func (a *dockerAuthorizer) getCredentials(host string) (sessionID, username, secret string, err error) {
	return sessionauth.CredentialsFunc(a.sm, a.session)(host)
}

func (a *dockerAuthorizer) AddResponses(ctx context.Context, responses []*http.Response) error {
	a.handlers.mu.Lock()
	defer a.handlers.mu.Unlock()

	last := responses[len(responses)-1]
	host := last.Request.URL.Host

	handler := a.handlers.get(host, a.sm, a.session)

	for _, c := range auth.ParseAuthHeader(last.Header) {
		if c.Scheme == auth.BearerAuth {
			var oldScopes []string
			if err := invalidAuthorization(c, responses); err != nil {
				a.handlers.delete(handler)

				if handler != nil {
					oldScopes = handler.common.Scopes
				}
				handler = nil

				// this hacky way seems to be best method to detect that error is fatal and should not be retried with a new token
				if c.Parameters["error"] == "insufficient_scope" && parseScopes(oldScopes).contains(parseScopes(strings.Split(c.Parameters["scope"], " "))) {
					return err
				}
			}

			// reuse existing handler
			//
			// assume that one registry will return the common
			// challenge information, including realm and service.
			// and the resource scope is only different part
			// which can be provided by each request.
			if handler != nil {
				return nil
			}

			var username, secret string
			session, pubKey, err := sessionauth.GetTokenAuthority(host, a.sm, a.session)
			if err != nil {
				return err
			}
			if pubKey == nil {
				session, username, secret, err = a.getCredentials(host)
				if err != nil {
					return err
				}
			}

			common, err := auth.GenerateTokenOptions(ctx, host, username, secret, c)
			if err != nil {
				return err
			}
			common.Scopes = parseScopes(append(common.Scopes, oldScopes...)).normalize()

			a.handlers.set(host, session, newAuthHandler(host, a.client, c.Scheme, pubKey, common))

			return nil
		} else if c.Scheme == auth.BasicAuth {
			session, username, secret, err := a.getCredentials(host)
			if err != nil {
				return err
			}

			if username != "" && secret != "" {
				common := auth.TokenOptions{
					Username: username,
					Secret:   secret,
				}

				a.handlers.set(host, session, newAuthHandler(host, a.client, c.Scheme, nil, common))

				return nil
			}
		}
	}
	return errors.Wrap(errdefs.ErrNotImplemented, "failed to find supported auth scheme")
}

// authResult is used to control limit rate.
type authResult struct {
	sync.WaitGroup
	token   string
	err     error
	expires time.Time
}

// authHandler is used to handle auth request per registry server.
type authHandler struct {
	sync.Mutex

	client *http.Client

	// only support basic and bearer schemes
	scheme auth.AuthenticationScheme

	// common contains common challenge answer
	common auth.TokenOptions

	// scopedTokens caches token indexed by scopes, which used in
	// bearer auth case
	scopedTokens map[string]*authResult

	lastUsed time.Time

	host string

	authority *[32]byte
}

func newAuthHandler(host string, client *http.Client, scheme auth.AuthenticationScheme, authority *[32]byte, opts auth.TokenOptions) *authHandler {
	return &authHandler{
		host:         host,
		client:       client,
		scheme:       scheme,
		common:       opts,
		scopedTokens: map[string]*authResult{},
		lastUsed:     time.Now(),
		authority:    authority,
	}
}

func (ah *authHandler) authorize(ctx context.Context, sm *session.Manager, g session.Group) (string, error) {
	switch ah.scheme {
	case auth.BasicAuth:
		return ah.doBasicAuth(ctx)
	case auth.BearerAuth:
		return ah.doBearerAuth(ctx, sm, g)
	default:
		return "", errors.Wrapf(errdefs.ErrNotImplemented, "failed to find supported auth scheme: %s", string(ah.scheme))
	}
}

func (ah *authHandler) doBasicAuth(ctx context.Context) (string, error) {
	username, secret := ah.common.Username, ah.common.Secret

	if username == "" || secret == "" {
		return "", fmt.Errorf("failed to handle basic auth because missing username or secret")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + secret))
	return fmt.Sprintf("Basic %s", auth), nil
}

func (ah *authHandler) doBearerAuth(ctx context.Context, sm *session.Manager, g session.Group) (token string, err error) {
	// copy common tokenOptions
	to := ah.common

	to.Scopes = parseScopes(docker.GetTokenScopes(ctx, to.Scopes)).normalize()

	// Docs: https://docs.docker.com/registry/spec/auth/scope
	scoped := strings.Join(to.Scopes, " ")

	ah.Lock()
	for {
		r, exist := ah.scopedTokens[scoped]
		if !exist {
			// no entry cached
			break
		}
		ah.Unlock()
		r.Wait()
		if r.err != nil {
			select {
			case <-ctx.Done():
				return "", r.err
			default:
			}
		}
		if !errors.Is(r.err, context.Canceled) &&
			(r.expires.IsZero() || r.expires.After(time.Now())) {
			return r.token, r.err
		}
		// r.err is canceled or token expired. Get rid of it and try again
		ah.Lock()
		r2, exist := ah.scopedTokens[scoped]
		if exist && r == r2 {
			delete(ah.scopedTokens, scoped)
		}
	}

	// only one fetch token job
	r := new(authResult)
	r.Add(1)
	ah.scopedTokens[scoped] = r
	ah.Unlock()

	var issuedAt time.Time
	var expires int
	defer func() {
		token = fmt.Sprintf("Bearer %s", token)
		r.token, r.err = token, err
		if err == nil {
			if issuedAt.IsZero() {
				issuedAt = time.Now()
			}
			if exp := issuedAt.Add(time.Duration(float64(expires)*0.9) * time.Second); time.Now().Before(exp) {
				r.expires = exp
			}
		}
		r.Done()
	}()

	if ah.authority != nil {
		resp, err := sessionauth.FetchToken(&sessionauth.FetchTokenRequest{
			ClientID: "buildkit-client",
			Host:     ah.host,
			Realm:    to.Realm,
			Service:  to.Service,
			Scopes:   to.Scopes,
		}, sm, g)
		if err != nil {
			return "", err
		}
		issuedAt, expires = time.Unix(resp.IssuedAt, 0), int(resp.ExpiresIn)
		return resp.Token, nil
	}

	// fetch token for the resource scope
	if to.Secret != "" {
		defer func() {
			err = errors.Wrap(err, "failed to fetch oauth token")
		}()
		// try GET first because Docker Hub does not support POST
		// switch once support has landed
		resp, err := auth.FetchToken(ctx, ah.client, nil, to)
		if err != nil {
			var errStatus remoteserrors.ErrUnexpectedStatus
			if errors.As(err, &errStatus) {
				// retry with POST request
				// As of September 2017, GCR is known to return 404.
				// As of February 2018, JFrog Artifactory is known to return 401.
				if (errStatus.StatusCode == 405 && to.Username != "") || errStatus.StatusCode == 404 || errStatus.StatusCode == 401 {
					resp, err := auth.FetchTokenWithOAuth(ctx, ah.client, nil, "buildkit-client", to)
					if err != nil {
						return "", err
					}
					issuedAt, expires = resp.IssuedAt, resp.ExpiresIn
					return resp.AccessToken, nil
				}
				log.G(ctx).WithFields(logrus.Fields{
					"status": errStatus.Status,
					"body":   string(errStatus.Body),
				}).Debugf("token request failed")
			}
			return "", err
		}
		issuedAt, expires = resp.IssuedAt, resp.ExpiresIn
		return resp.Token, nil
	}
	// do request anonymously
	resp, err := auth.FetchToken(ctx, ah.client, nil, to)
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch anonymous token")
	}
	issuedAt, expires = resp.IssuedAt, resp.ExpiresIn

	return resp.Token, nil
}

func invalidAuthorization(c auth.Challenge, responses []*http.Response) error {
	errStr := c.Parameters["error"]
	if errStr == "" {
		return nil
	}

	n := len(responses)
	if n == 1 || (n > 1 && !sameRequest(responses[n-2].Request, responses[n-1].Request)) {
		return nil
	}

	return errors.Wrapf(docker.ErrInvalidAuthorization, "server message: %s", errStr)
}

func sameRequest(r1, r2 *http.Request) bool {
	if r1.Method != r2.Method {
		return false
	}
	if *r1.URL != *r2.URL {
		return false
	}
	return true
}

type scopes map[string]map[string]struct{}

func parseScopes(s []string) scopes {
	// https://docs.docker.com/registry/spec/auth/scope/
	m := map[string]map[string]struct{}{}
	for _, scope := range s {
		parts := strings.SplitN(scope, ":", 3)
		names := []string{parts[0]}
		if len(parts) > 1 {
			names = append(names, parts[1])
		}
		var actions []string
		if len(parts) == 3 {
			actions = append(actions, strings.Split(parts[2], ",")...)
		}
		name := strings.Join(names, ":")
		ma, ok := m[name]
		if !ok {
			ma = map[string]struct{}{}
			m[name] = ma
		}

		for _, a := range actions {
			ma[a] = struct{}{}
		}
	}
	return m
}

func (s scopes) normalize() []string {
	names := make([]string, 0, len(s))
	for n := range s {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]string, 0, len(s))

	for _, n := range names {
		actions := make([]string, 0, len(s[n]))
		for a := range s[n] {
			actions = append(actions, a)
		}
		sort.Strings(actions)

		out = append(out, n+":"+strings.Join(actions, ","))
	}
	return out
}

func (s scopes) contains(s2 scopes) bool {
	for n := range s2 {
		v, ok := s[n]
		if !ok {
			return false
		}
		for a := range s2[n] {
			if _, ok := v[a]; !ok {
				return false
			}
		}
	}
	return true
}
