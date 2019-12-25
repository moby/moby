package suite

import "testing"

// SetupAllSuite has a SetupSuite method, which will run before the
// tests in the suite are run.
type SetupAllSuite interface {
	SetUpSuite(t *testing.T)
}

// SetupTestSuite has a SetupTest method, which will run before each
// test in the suite.
type SetupTestSuite interface {
	SetUpTest(t *testing.T)
}

// TearDownAllSuite has a TearDownSuite method, which will run after
// all the tests in the suite have been run.
type TearDownAllSuite interface {
	TearDownSuite(t *testing.T)
}

// TearDownTestSuite has a TearDownTest method, which will run after
// each test in the suite.
type TearDownTestSuite interface {
	TearDownTest(t *testing.T)
}

// TimeoutTestSuite has a OnTimeout method, which will run after
// a single test times out after a period specified by -timeout flag.
type TimeoutTestSuite interface {
	OnTimeout()
}
