package client

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/docker/distribution/testutil"
)

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

func testServerWithAuth(rrm testutil.RequestResponseMap, authenticate string, authCheck func(string) bool) (*RepositoryEndpoint, func()) {
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
	e := RepositoryEndpoint{Endpoint: s.URL, Mirror: false}
	return &e, s.Close
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
				Route:  "/hello",
			},
			Response: testutil.Response{
				StatusCode: http.StatusAccepted,
			},
		},
	})

	authenicate := fmt.Sprintf("Bearer realm=%q,service=%q", te.Endpoint+"/token", service)
	validCheck := func(a string) bool {
		return a == "Bearer statictoken"
	}
	e, c := testServerWithAuth(m, authenicate, validCheck)
	defer c()

	client, err := e.HTTPClient(repo1)
	if err != nil {
		t.Fatalf("Error creating http client: %s", err)
	}

	req, _ := http.NewRequest("GET", e.Endpoint+"/hello", nil)
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

	client2, err := e2.HTTPClient(repo2)
	if err != nil {
		t.Fatalf("Error creating http client: %s", err)
	}

	req, _ = http.NewRequest("GET", e.Endpoint+"/hello", nil)
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
				Route:  "/hello",
			},
			Response: testutil.Response{
				StatusCode: http.StatusAccepted,
			},
		},
	})

	authenicate2 := fmt.Sprintf("Bearer realm=%q,service=%q", te.Endpoint+"/token", service)
	bearerCheck := func(a string) bool {
		return a == "Bearer statictoken"
	}
	e, c := testServerWithAuth(m, authenicate2, bearerCheck)
	defer c()

	e.Credentials = &testCredentialStore{
		username: username,
		password: password,
	}

	client, err := e.HTTPClient(repo)
	if err != nil {
		t.Fatalf("Error creating http client: %s", err)
	}

	req, _ := http.NewRequest("GET", e.Endpoint+"/hello", nil)
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
				Route:  "/hello",
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
	e.Credentials = &testCredentialStore{
		username: username,
		password: password,
	}

	client, err := e.HTTPClient("test/repo/basic")
	if err != nil {
		t.Fatalf("Error creating http client: %s", err)
	}

	req, _ := http.NewRequest("GET", e.Endpoint+"/hello", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Error sending get request: %s", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("Unexpected status code: %d, expected %d", resp.StatusCode, http.StatusAccepted)
	}
}
