package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/distribution/testutil"
)

func testServer(rrm testutil.RequestResponseMap) (string, func()) {
	h := testutil.NewHandler(rrm)
	s := httptest.NewServer(h)
	return s.URL, s.Close
}

type testAuthenticationWrapper struct {
	headers   http.Header
	authCheck func(string) bool
	next      http.Handler
}

func (w *testAuthenticationWrapper) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" || !w.authCheck(auth) {
		h := rw.Header()
		for k, values := range w.headers {
			h[k] = values
		}
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.next.ServeHTTP(rw, r)
}

func testServerWithAuth(rrm testutil.RequestResponseMap, authenticate string, authCheck func(string) bool) (string, func()) {
	h := testutil.NewHandler(rrm)
	wrapper := &testAuthenticationWrapper{

		headers: http.Header(map[string][]string{
			"X-API-Version":       {"registry/2.0"},
			"X-Multi-API-Version": {"registry/2.0", "registry/2.1", "trust/1.0"},
			"WWW-Authenticate":    {authenticate},
		}),
		authCheck: authCheck,
		next:      h,
	}

	s := httptest.NewServer(wrapper)
	return s.URL, s.Close
}

// ping pings the provided endpoint to determine its required authorization challenges.
// If a version header is provided, the versions will be returned.
func ping(manager ChallengeManager, endpoint, versionHeader string) ([]APIVersion, error) {
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := manager.AddResponse(resp); err != nil {
		return nil, err
	}

	return APIVersions(resp, versionHeader), err
}

type testCredentialStore struct {
	username string
	password string
}

func (tcs *testCredentialStore) Basic(*url.URL) (string, string) {
	return tcs.username, tcs.password
}

func TestEndpointAuthorizeToken(t *testing.T) {
	service := "localhost.localdomain"
	repo1 := "some/registry"
	repo2 := "other/registry"
	scope1 := fmt.Sprintf("repository:%s:pull,push", repo1)
	scope2 := fmt.Sprintf("repository:%s:pull,push", repo2)
	tokenMap := testutil.RequestResponseMap([]testutil.RequestResponseMapping{
		{
			Request: testutil.Request{
				Method: "GET",
				Route:  fmt.Sprintf("/token?scope=%s&service=%s", url.QueryEscape(scope1), service),
			},
			Response: testutil.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(`{"token":"statictoken"}`),
			},
		},
		{
			Request: testutil.Request{
				Method: "GET",
				Route:  fmt.Sprintf("/token?scope=%s&service=%s", url.QueryEscape(scope2), service),
			},
			Response: testutil.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(`{"token":"badtoken"}`),
			},
		},
	})
	te, tc := testServer(tokenMap)
	defer tc()

	m := testutil.RequestResponseMap([]testutil.RequestResponseMapping{
		{
			Request: testutil.Request{
				Method: "GET",
				Route:  "/v2/hello",
			},
			Response: testutil.Response{
				StatusCode: http.StatusAccepted,
			},
		},
	})

	authenicate := fmt.Sprintf("Bearer realm=%q,service=%q", te+"/token", service)
	validCheck := func(a string) bool {
		return a == "Bearer statictoken"
	}
	e, c := testServerWithAuth(m, authenicate, validCheck)
	defer c()

	challengeManager1 := NewSimpleChallengeManager()
	versions, err := ping(challengeManager1, e+"/v2/", "x-api-version")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("Unexpected version count: %d, expected 1", len(versions))
	}
	if check := (APIVersion{Type: "registry", Version: "2.0"}); versions[0] != check {
		t.Fatalf("Unexpected api version: %#v, expected %#v", versions[0], check)
	}
	transport1 := transport.NewTransport(nil, NewAuthorizer(challengeManager1, NewTokenHandler(nil, nil, repo1, "pull", "push")))
	client := &http.Client{Transport: transport1}

	req, _ := http.NewRequest("GET", e+"/v2/hello", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error sending get request: %s", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("Unexpected status code: %d, expected %d", resp.StatusCode, http.StatusAccepted)
	}

	badCheck := func(a string) bool {
		return a == "Bearer statictoken"
	}
	e2, c2 := testServerWithAuth(m, authenicate, badCheck)
	defer c2()

	challengeManager2 := NewSimpleChallengeManager()
	versions, err = ping(challengeManager2, e+"/v2/", "x-multi-api-version")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 3 {
		t.Fatalf("Unexpected version count: %d, expected 3", len(versions))
	}
	if check := (APIVersion{Type: "registry", Version: "2.0"}); versions[0] != check {
		t.Fatalf("Unexpected api version: %#v, expected %#v", versions[0], check)
	}
	if check := (APIVersion{Type: "registry", Version: "2.1"}); versions[1] != check {
		t.Fatalf("Unexpected api version: %#v, expected %#v", versions[1], check)
	}
	if check := (APIVersion{Type: "trust", Version: "1.0"}); versions[2] != check {
		t.Fatalf("Unexpected api version: %#v, expected %#v", versions[2], check)
	}
	transport2 := transport.NewTransport(nil, NewAuthorizer(challengeManager2, NewTokenHandler(nil, nil, repo2, "pull", "push")))
	client2 := &http.Client{Transport: transport2}

	req, _ = http.NewRequest("GET", e2+"/v2/hello", nil)
	resp, err = client2.Do(req)
	if err != nil {
		t.Fatalf("Error sending get request: %s", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Unexpected status code: %d, expected %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func TestEndpointAuthorizeTokenBasic(t *testing.T) {
	service := "localhost.localdomain"
	repo := "some/fun/registry"
	scope := fmt.Sprintf("repository:%s:pull,push", repo)
	username := "tokenuser"
	password := "superSecretPa$$word"

	tokenMap := testutil.RequestResponseMap([]testutil.RequestResponseMapping{
		{
			Request: testutil.Request{
				Method: "GET",
				Route:  fmt.Sprintf("/token?account=%s&scope=%s&service=%s", username, url.QueryEscape(scope), service),
			},
			Response: testutil.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(`{"token":"statictoken"}`),
			},
		},
	})

	authenicate1 := fmt.Sprintf("Basic realm=localhost")
	basicCheck := func(a string) bool {
		return a == fmt.Sprintf("Basic %s", basicAuth(username, password))
	}
	te, tc := testServerWithAuth(tokenMap, authenicate1, basicCheck)
	defer tc()

	m := testutil.RequestResponseMap([]testutil.RequestResponseMapping{
		{
			Request: testutil.Request{
				Method: "GET",
				Route:  "/v2/hello",
			},
			Response: testutil.Response{
				StatusCode: http.StatusAccepted,
			},
		},
	})

	authenicate2 := fmt.Sprintf("Bearer realm=%q,service=%q", te+"/token", service)
	bearerCheck := func(a string) bool {
		return a == "Bearer statictoken"
	}
	e, c := testServerWithAuth(m, authenicate2, bearerCheck)
	defer c()

	creds := &testCredentialStore{
		username: username,
		password: password,
	}

	challengeManager := NewSimpleChallengeManager()
	_, err := ping(challengeManager, e+"/v2/", "")
	if err != nil {
		t.Fatal(err)
	}
	transport1 := transport.NewTransport(nil, NewAuthorizer(challengeManager, NewTokenHandler(nil, creds, repo, "pull", "push"), NewBasicHandler(creds)))
	client := &http.Client{Transport: transport1}

	req, _ := http.NewRequest("GET", e+"/v2/hello", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error sending get request: %s", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("Unexpected status code: %d, expected %d", resp.StatusCode, http.StatusAccepted)
	}
}

func TestEndpointAuthorizeBasic(t *testing.T) {
	m := testutil.RequestResponseMap([]testutil.RequestResponseMapping{
		{
			Request: testutil.Request{
				Method: "GET",
				Route:  "/v2/hello",
			},
			Response: testutil.Response{
				StatusCode: http.StatusAccepted,
			},
		},
	})

	username := "user1"
	password := "funSecretPa$$word"
	authenicate := fmt.Sprintf("Basic realm=localhost")
	validCheck := func(a string) bool {
		return a == fmt.Sprintf("Basic %s", basicAuth(username, password))
	}
	e, c := testServerWithAuth(m, authenicate, validCheck)
	defer c()
	creds := &testCredentialStore{
		username: username,
		password: password,
	}

	challengeManager := NewSimpleChallengeManager()
	_, err := ping(challengeManager, e+"/v2/", "")
	if err != nil {
		t.Fatal(err)
	}
	transport1 := transport.NewTransport(nil, NewAuthorizer(challengeManager, NewBasicHandler(creds)))
	client := &http.Client{Transport: transport1}

	req, _ := http.NewRequest("GET", e+"/v2/hello", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error sending get request: %s", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("Unexpected status code: %d, expected %d", resp.StatusCode, http.StatusAccepted)
	}
}
