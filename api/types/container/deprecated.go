package container // import "github.com/docker/docker/api/types/container"

// ContainerCreateCreatedBody OK response to ContainerCreate operation
//
// Deprecated: use CreateResponse
type ContainerCreateCreatedBody = CreateResponse

// ContainerWaitOKBody OK response to ContainerWait operation
//
// Deprecated: use WaitResponse
type ContainerWaitOKBody = WaitResponse

// ContainerWaitOKBodyError container waiting error, if any
//
// Deprecated: use WaitExitError
type ContainerWaitOKBodyError = WaitExitError
