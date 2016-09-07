package graphdb

import "github.com/go-check/check"

func (s *DockerSuite) TestSort(c *check.C) {
	paths := []string{
		"/",
		"/myreallylongname",
		"/app/db",
	}

	sortByDepth(paths)

	if len(paths) != 3 {
		c.Fatalf("Expected 3 parts got %d", len(paths))
	}

	if paths[0] != "/app/db" {
		c.Fatalf("Expected /app/db got %s", paths[0])
	}
	if paths[1] != "/myreallylongname" {
		c.Fatalf("Expected /myreallylongname got %s", paths[1])
	}
	if paths[2] != "/" {
		c.Fatalf("Expected / got %s", paths[2])
	}
}
