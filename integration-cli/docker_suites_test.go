package main

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// IntegTestSuite is the default test suite. All tests which don't
// need to be added to a specifc suite should be added to this suite.
type IntegTestSuite struct {
	suite.Suite
}

func TestDockerIntegrationTestSuitei(t *testing.T) {
	suite.Run(t, new(IntegTestSuite))
}

func (suite *IntegTestSuite) TearDownTest() {
	deleteAllContainers()
}
