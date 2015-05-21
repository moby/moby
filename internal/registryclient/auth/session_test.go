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
			"Docker-Distribution-API-Version": {"registry/2.0"},
			"WWW-Authenticate":                {authenticate},
		}),
		authCheck: authCheck,
		next:      h,
	}

	s := httptest.NewServer(wrapper)
	return s.URL, s.Close
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

	challenges1, _, err := Ping(&http.Client{}, e+"/v2/", "")
	if err != nil {
		t.Fatal(err)
	}
	challengeMap1 := map[string][]Challenge{
		e + "/v2/": challenges1,
	}
	transport1 := transport.NewTransport(nil, NewAuthorizer(challengeMap1, NewTokenHandler(nil, nil, repo1, "pull", "push")))
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

	challenges2, _, err := Ping(&http.Client{}, e+"/v2/", "")
	if err != nil {
		t.Fatal(err)
	}
	challengeMap2 := map[string][]Challenge{
		e + "/v2/": challenges2,
	}
	transport2 := transport.NewTransport(nil, NewAuthorizer(challengeMap2, NewTokenHandler(nil, nil, repo2, "pull", "push")))
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

	challenges, _, err := Ping(&http.Client{}, e+"/v2/", "")
	if err != nil {
		t.Fatal(err)
	}
	challengeMap := map[string][]Challenge{
		e + "/v2/": challenges,
	}
	transport1 := transport.NewTransport(nil, NewAuthorizer(challengeMap, NewTokenHandler(nil, creds, repo, "pull", "push"), NewBasicHandler(creds)))
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

	challenges, _, err := Ping(&http.Client{}, e+"/v2/", "")
	if err != nil {
		t.Fatal(err)
	}
	challengeMap := map[string][]Challenge{
		e + "/v2/": challenges,
	}
	transport1 := transport.NewTransport(nil, NewAuthorizer(challengeMap, NewBasicHandler(creds)))
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
