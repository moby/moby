package dockerfile

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestIsExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "existing-file")
	err := os.WriteFile(tmpFile, []byte("something"), 0o644)
	assert.NilError(t, err)

	testcases := []struct {
		doc      string
		path     string
		expected bool
	}{
		{
			doc:      "directory exists",
			path:     tmpDir,
			expected: true,
		},
		{
			doc:      "path doesn't exist",
			path:     "/bogus/path/does/not/exist",
			expected: false,
		},
		{
			doc:      "file exists",
			path:     tmpFile,
			expected: false,
		},
	}

	for _, testcase := range testcases {
		result, err := isExistingDirectory(testcase.path)
		if !assert.Check(t, err) {
			continue
		}
		assert.Check(t, is.Equal(testcase.expected, result), testcase.doc)
	}
}

func TestGetFilenameForDownload(t *testing.T) {
	testcases := []struct {
		path        string
		disposition string
		expected    string
	}{
		{
			path:     "https://www.example.com/",
			expected: "",
		},
		{
			path:     "https://www.example.com/xyz",
			expected: "xyz",
		},
		{
			path:     "https://www.example.com/xyz.html",
			expected: "xyz.html",
		},
		{
			path:     "https://www.example.com/xyz/",
			expected: "",
		},
		{
			path:     "https://www.example.com/xyz/uvw",
			expected: "uvw",
		},
		{
			path:     "https://www.example.com/xyz/uvw.html",
			expected: "uvw.html",
		},
		{
			path:     "https://www.example.com/xyz/uvw/",
			expected: "",
		},
		{
			path:     "/",
			expected: "",
		},
		{
			path:     "/xyz",
			expected: "xyz",
		},
		{
			path:     "/xyz.html",
			expected: "xyz.html",
		},
		{
			path:     "/xyz/",
			expected: "",
		},
		{
			path:        "/xyz/",
			disposition: "attachment; filename=xyz.html",
			expected:    "xyz.html",
		},
		{
			disposition: "",
			expected:    "",
		},
		{
			disposition: "attachment; filename=xyz",
			expected:    "xyz",
		},
		{
			disposition: "attachment; filename=xyz.html",
			expected:    "xyz.html",
		},
		{
			disposition: `attachment; filename="xyz"`,
			expected:    "xyz",
		},
		{
			disposition: `attachment; filename="xyz.html"`,
			expected:    "xyz.html",
		},
		{
			disposition: `attachment; filename="/xyz.html"`,
			expected:    "xyz.html",
		},
		{
			disposition: `attachment; filename="/xyz/uvw"`,
			expected:    "uvw",
		},
		{
			disposition: `attachment; filename="Naïve file.txt"`,
			expected:    "Naïve file.txt",
		},
	}
	for _, testcase := range testcases {
		resp := http.Response{
			Header: make(map[string][]string),
		}
		if testcase.disposition != "" {
			resp.Header.Add("Content-Disposition", testcase.disposition)
		}
		filename := getFilenameForDownload(testcase.path, &resp)
		assert.Check(t, is.Equal(testcase.expected, filename))
	}
}
