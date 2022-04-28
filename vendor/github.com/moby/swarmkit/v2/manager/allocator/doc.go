// Package allocator aims to manage allocation of different
// cluster-wide resources on behalf of the manager. In particular, it
// manages a set of independent allocator processes which can mostly
// execute concurrently with only a minimal need for coordination.
//
// One of the instances where it needs coordination is when deciding to
// move a task to the PENDING state. Since a task can move to the
// PENDING state only when all the task allocators have completed,
// they must cooperate. The way `allocator` achieves this is by creating
// a `taskBallot` to which all task allocators register themselves as
// mandatory voters. For each task that needs allocation, each allocator
// independently votes to indicate the completion of their allocation.
// Once all registered voters have voted then the task is moved to the
// PENDING state.
//
// Other than the coordination needed for task PENDING state, all
// the allocators function fairly independently.
package allocator
