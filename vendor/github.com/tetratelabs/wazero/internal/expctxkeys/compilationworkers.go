package expctxkeys

// CompilationWorkers is a context.Context Value key.
// Its associated value should be an int representing the number of workers
// we want to spawn to compile a given wasm input.
type CompilationWorkers struct{}
