package reconciler

// The reconciler package contains the parts of the stacks component that does
// the actual reconciliation of stacks. The reconciler's job is to make sure
// that the desired state as reflected in the Stack object is in turn reflected
// in the Specs spawned from that object.
//
// The reconciler consists of several packages, each of which have a different
// job, split apart for ease of testing.
//
// The `reconciler` package does the work of taking an object, and deciding
// what needs to be changed about that object to bring it in line with desired
// state as defined in the stack.
//
// The `dispatcher` package decides which objects the reconciler should deal
// with, when to deal with them, and in what order.
//
// The `notifier` package contains glue code, to break an otherwise cyclic
// dependency between the reconciler and dispatcher.
