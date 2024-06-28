package container // import "github.com/docker/docker/container"

// StoreFilter defines a function to filter
// container in the store.
type StoreFilter func(*Container) bool

// StoreReducer defines a function to
// manipulate containers in the store
type StoreReducer func(*Container)
