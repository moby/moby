package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"

	"gotest.tools/v3/assert"
)

func (s *DockerSuite) BenchmarkConcurrentContainerActions(c *testing.B) {
	maxConcurrency := runtime.GOMAXPROCS(0)
	numIterations := c.N
	outerGroup := &sync.WaitGroup{}
	outerGroup.Add(maxConcurrency)
	chErr := make(chan error, numIterations*2*maxConcurrency)

	for i := 0; i < maxConcurrency; i++ {
		go func() {
			defer outerGroup.Done()
			innerGroup := &sync.WaitGroup{}
			innerGroup.Add(2)

			go func() {
				defer innerGroup.Done()
				for i := 0; i < numIterations; i++ {
					args := []string{"run", "-d", "busybox"}
					args = append(args, sleepCommandForDaemonPlatform()...)
					out, _, err := dockerCmdWithError(args...)
					if err != nil {
						chErr <- fmt.Errorf(out)
						return
					}

					id := strings.TrimSpace(out)
					tmpDir, err := os.MkdirTemp("", "docker-concurrent-test-"+id)
					if err != nil {
						chErr <- err
						return
					}
					defer os.RemoveAll(tmpDir)
					out, _, err = dockerCmdWithError("cp", id+":/tmp", tmpDir)
					if err != nil {
						chErr <- fmt.Errorf(out)
						return
					}

					out, _, err = dockerCmdWithError("kill", id)
					if err != nil {
						chErr <- fmt.Errorf(out)
					}

					out, _, err = dockerCmdWithError("start", id)
					if err != nil {
						chErr <- fmt.Errorf(out)
					}

					out, _, err = dockerCmdWithError("kill", id)
					if err != nil {
						chErr <- fmt.Errorf(out)
					}

					// don't do an rm -f here since it can potentially ignore errors from the graphdriver
					out, _, err = dockerCmdWithError("rm", id)
					if err != nil {
						chErr <- fmt.Errorf(out)
					}
				}
			}()

			go func() {
				defer innerGroup.Done()
				for i := 0; i < numIterations; i++ {
					out, _, err := dockerCmdWithError("ps")
					if err != nil {
						chErr <- fmt.Errorf(out)
					}
				}
			}()

			innerGroup.Wait()
		}()
	}

	outerGroup.Wait()
	close(chErr)

	for err := range chErr {
		assert.NilError(c, err)
	}
}
