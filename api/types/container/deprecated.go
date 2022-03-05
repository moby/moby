package container // import "github.com/docker/docker/api/types/container"

// ContainerWaitOKBody OK response to ContainerWait operation
//
// Deprecated: use WaitResponse
type ContainerWaitOKBody = WaitResponse

// ContainerWaitOKBodyError container waiting error, if any
//
// Deprecated: use WaitExitError
type ContainerWaitOKBodyError = WaitExitError
