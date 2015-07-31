package main

import (
	"fmt"
	"time"

	"github.com/go-check/check"
)

// To reproduce the deadlock we have to create many images before the test.
func createImages(c *check.C, n int) {
	for i := 0; i < n; i++ {
		dockerfile := fmt.Sprintf(`
	FROM busybox
	RUN echo a >/t%d.txt
	RUN echo b >/%d.txt`, i, i)
		_, err := buildImage("dummy", dockerfile, false)
		c.Assert(err, check.IsNil)
	}
}

func (s *DockerSuite) TestBuildDeadlock(c *check.C) {
	createImages(c, 50)
	dockerfile := `
	FROM busybox
	RUN echo a >/1.txt
	RUN echo b >/2.txt
	RUN echo c >/3.txt
	RUN ps -ef >/ps1.txt
	RUN ps -ef >/ps2.txt
	RUN ps -ef >/ps3.txt
	RUN ps -ef >/ps4.txt
	RUN ps -ef >/ps5.txt
	RUN ps -ef >/ps6.txt
	`
	ch := make(chan struct{})
	go func() {
		_, err := buildImage("dummy", dockerfile, false)
		c.Assert(err, check.IsNil)
		close(ch)
	}()

	go func() {
		<-time.After(5 * time.Minute)
		_, out, err := sockRequest("GET", "/debug/pprof/goroutine?debug=2", nil)
		c.Assert(err, check.IsNil)
		c.Fatalf("docker build didn't return after 5 minutes, possibly a deadlock. %s", string(out))
	}()

	for i := 0; i < 100; i++ {
		fmt.Println(i)
		dockerCmd(c, "images")
	}
	<-ch
}

func (s *DockerSuite) TestRmiDeadlock(c *check.C) {
	createImages(c, 50)
	dockerfile := `
	FROM busybox
	RUN echo a >/1.txt
	RUN echo b >/2.txt
	RUN echo c >/3.txt
	RUN ps -ef >/ps1.txt
	RUN ps -ef >/ps2.txt
	RUN ps -ef >/ps3.txt
	RUN ps -ef >/ps4.txt
	RUN ps -ef >/ps5.txt
	RUN ps -ef >/ps6.txt
	`
	id, err := buildImage("dummy", dockerfile, false)
	c.Assert(err, check.IsNil)

	ch := make(chan struct{})
	go func() {
		dockerCmd(c, "rmi", id)
		close(ch)
	}()

	go func() {
		<-time.After(5 * time.Minute)
		_, out, err := sockRequest("GET", "/debug/pprof/goroutine?debug=2", nil)
		c.Assert(err, check.IsNil)
		c.Fatalf("docker rmi %s didn't return after 5 minutes, possibly a deadlock. %s", id, string(out))
	}()

	for i := 0; i < 100; i++ {
		fmt.Println(i)
		dockerCmd(c, "images")
	}
	<-ch
}
