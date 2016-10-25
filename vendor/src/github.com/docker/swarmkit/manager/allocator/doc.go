// Package allocator aims to manage allocation of different
// cluster-wide resources on behalf of the manager. In particular, it
// manages a set of independent allocator processes which can mostly
// execute concurrently with only a minimal need for coordination.
//
// One of the instances where it needs coordination is when to move a
// task to ALLOCATED state. Since a task can move to ALLOCATED state
// only when all task allocators have completed their service of
// allocation, they all have to agree on that. The way this achieved
// in `allocator` is by creating a `taskBallot` to which all task
// allocators register themselves as mandatory voters. For each task
// that needs allocation, each allocator independently votes to indicate
// the completion of their allocation. Once all registered voters have
// voted then the task is moved to ALLOCATED state.
//
// Other than the coordination needed for task ALLOCATED state, all
// the allocators function fairly independently.
package allocator
