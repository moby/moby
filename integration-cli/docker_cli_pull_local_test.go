package main

import (
	"fmt"
	"strings"

	"github.com/go-check/check"
)

// TestPullImageWithAliases pulls a specific image tag and verifies that any aliases (i.e., other
// tags for the same image) are not also pulled down.
//
// Ref: docker/docker#8141
func (s *DockerRegistrySuite) TestPullImageWithAliases(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	repos := []string{}
	for _, tag := range []string{"recent", "fresh"} {
		repos = append(repos, fmt.Sprintf("%v:%v", repoName, tag))
	}

	// Tag and push the same image multiple times.
	for _, repo := range repos {
		s.Cmd(c, "tag", "busybox", repo)
		s.Cmd(c, "push", repo)
	}

	// Clear local images store.
	s.Cmd(c, "rmi", repos...)

	// Pull a single tag and verify it doesn't bring down all aliases.
	s.Cmd(c, "pull", repos[0])
	s.Cmd(c, "inspect", repos[0])
	for _, repo := range repos[1:] {
		if _, err := s.CmdWithError("inspect", repo); err == nil {
			c.Fatalf("Image %v shouldn't have been pulled down", repo)
		}
	}
}

// TestConcurrentPullWholeRepo pulls the same repo concurrently.
func (s *DockerRegistrySuite) TestConcurrentPullWholeRepo(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	repos := []string{}
	for _, tag := range []string{"recent", "fresh", "todays"} {
		repo := fmt.Sprintf("%v:%v", repoName, tag)
		_, err := buildImage(s, repo, fmt.Sprintf(`
		    FROM busybox
		    ENTRYPOINT ["/bin/echo"]
		    ENV FOO foo
		    ENV BAR bar
		    CMD echo %s
		`, repo), true)
		if err != nil {
			c.Fatal(err)
		}
		s.Cmd(c, "push", repo)
		repos = append(repos, repo)
	}

	// Clear local images store.
	s.Cmd(c, "rmi", repos...)

	// Run multiple re-pulls concurrently
	results := make(chan error)
	numPulls := 3

	for i := 0; i != numPulls; i++ {
		go func() {
			_, _, err := runCommandWithOutput(s.MakeCmd("pull", "-a", repoName))
			results <- err
		}()
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for i := 0; i != numPulls; i++ {
		err := <-results
		c.Assert(err, check.IsNil, check.Commentf("concurrent pull failed with error: %v", err))
	}

	// Ensure all tags were pulled successfully
	for _, repo := range repos {
		s.Cmd(c, "inspect", repo)
		out := s.Cmd(c, "run", "--rm", repo)
		if strings.TrimSpace(out) != "/bin/sh -c echo "+repo {
			c.Fatalf("CMD did not contain /bin/sh -c echo %s: %s", repo, out)
		}
	}
}

// TestConcurrentFailingPull tries a concurrent pull that doesn't succeed.
func (s *DockerRegistrySuite) TestConcurrentFailingPull(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	// Run multiple pulls concurrently
	results := make(chan error)
	numPulls := 3

	for i := 0; i != numPulls; i++ {
		go func() {
			_, _, err := runCommandWithOutput(s.MakeCmd("pull", repoName+":asdfasdf"))
			results <- err
		}()
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for i := 0; i != numPulls; i++ {
		err := <-results
		if err == nil {
			c.Fatal("expected pull to fail")
		}
	}
}

// TestConcurrentPullMultipleTags pulls multiple tags from the same repo
// concurrently.
func (s *DockerRegistrySuite) TestConcurrentPullMultipleTags(c *check.C) {
	repoName := fmt.Sprintf("%v/dockercli/busybox", privateRegistryURL)

	repos := []string{}
	for _, tag := range []string{"recent", "fresh", "todays"} {
		repo := fmt.Sprintf("%v:%v", repoName, tag)
		_, err := buildImage(s, repo, fmt.Sprintf(`
		    FROM busybox
		    ENTRYPOINT ["/bin/echo"]
		    ENV FOO foo
		    ENV BAR bar
		    CMD echo %s
		`, repo), true)
		if err != nil {
			c.Fatal(err)
		}
		s.Cmd(c, "push", repo)
		repos = append(repos, repo)
	}

	// Clear local images store.
	s.Cmd(c, "rmi", repos...)

	// Re-pull individual tags, in parallel
	results := make(chan error)

	for _, repo := range repos {
		go func(repo string) {
			_, _, err := runCommandWithOutput(s.MakeCmd("pull", repo))
			results <- err
		}(repo)
	}

	// These checks are separate from the loop above because the check
	// package is not goroutine-safe.
	for range repos {
		err := <-results
		c.Assert(err, check.IsNil, check.Commentf("concurrent pull failed with error: %v", err))
	}

	// Ensure all tags were pulled successfully
	for _, repo := range repos {
		s.Cmd(c, "inspect", repo)
		out := s.Cmd(c, "run", "--rm", repo)
		if strings.TrimSpace(out) != "/bin/sh -c echo "+repo {
			c.Fatalf("CMD did not contain /bin/sh -c echo %s: %s", repo, out)
		}
	}
}
