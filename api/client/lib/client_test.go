package lib

import (
	"net/url"
	"testing"
)

func TestGetAPIPath(t *testing.T) {
	cases := []struct {
		v string
		p string
		q url.Values
		e string
	}{
		{"", "/containers/json", nil, "/containers/json"},
		{"", "/containers/json", url.Values{}, "/containers/json"},
		{"", "/containers/json", url.Values{"s": []string{"c"}}, "/containers/json?s=c"},
		{"1.22", "/containers/json", nil, "/v1.22/containers/json"},
		{"1.22", "/containers/json", url.Values{}, "/v1.22/containers/json"},
		{"1.22", "/containers/json", url.Values{"s": []string{"c"}}, "/v1.22/containers/json?s=c"},
		{"v1.22", "/containers/json", nil, "/v1.22/containers/json"},
		{"v1.22", "/containers/json", url.Values{}, "/v1.22/containers/json"},
		{"v1.22", "/containers/json", url.Values{"s": []string{"c"}}, "/v1.22/containers/json?s=c"},
	}

	for _, cs := range cases {
		c, err := NewClient("unix:///var/run/docker.sock", cs.v, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		g := c.getAPIPath(cs.p, cs.q)
		if g != cs.e {
			t.Fatalf("Expected %s, got %s", cs.e, g)
		}
	}
}
