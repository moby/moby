package tarsum

// BuilderContext is an interface extending TarSum by adding the Remove method.
// In general there was concern about adding this method to TarSum itself
// so instead it is being added just to "BuilderContext" which will then
// only be used during the .dockerignore file processing
// - see builder/evaluator.go
type BuilderContext interface {
	TarSum
	Remove(string)
}

func (ts *tarSum) Remove(filename string) {
	for i, fis := range ts.sums {
		if fis.Name() == filename {
			ts.sums = append(ts.sums[:i], ts.sums[i+1:]...)
			// Note, we don't just return because there could be
			// more than one with this name
		}
	}
}
