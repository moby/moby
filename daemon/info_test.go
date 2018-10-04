package daemon

import (
	"testing"

	"gotest.tools/assert"
)

func TestMaskURLCredentials(t *testing.T) {
	tests := []struct {
		rawURL    string
		maskedURL string
	}{
		{
			rawURL:    "",
			maskedURL: "",
		}, {
			rawURL:    "invalidURL",
			maskedURL: "invalidURL",
		}, {
			rawURL:    "http://proxy.example.com:80/",
			maskedURL: "http://proxy.example.com:80/",
		}, {
			rawURL:    "http://USER:PASSWORD@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://PASSWORD:PASSWORD@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://USER:@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://:PASSWORD@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://USER@docker:password@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://USER%40docker:password@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://USER%40docker:pa%3Fsword@proxy.example.com:80/",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/",
		}, {
			rawURL:    "http://USER%40docker:pa%3Fsword@proxy.example.com:80/hello%20world",
			maskedURL: "http://xxxxx:xxxxx@proxy.example.com:80/hello%20world",
		},
	}
	for _, test := range tests {
		maskedURL := maskCredentials(test.rawURL)
		assert.Equal(t, maskedURL, test.maskedURL)
	}
}
