package random

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/go-check/check"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type DockerSuite struct{}

var _ = check.Suite(&DockerSuite{})

// for go test -v -race
func (s *DockerSuite) TestConcurrency(c *check.C) {
	rnd := rand.New(NewSource())
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			rnd.Int63()
			wg.Done()
		}()
	}
	wg.Wait()
}
