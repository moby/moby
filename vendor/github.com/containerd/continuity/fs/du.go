package fs

import "context"

// Usage of disk information
type Usage struct {
	Inodes int64
	Size   int64
}

// DiskUsage counts the number of inodes and disk usage for the resources under
// path.
func DiskUsage(roots ...string) (Usage, error) {
	return diskUsage(roots...)
}

// DiffUsage counts the numbers of inodes and disk usage in the
// diff between the 2 directories. The first path is intended
// as the base directory and the second as the changed directory.
func DiffUsage(ctx context.Context, a, b string) (Usage, error) {
	return diffUsage(ctx, a, b)
}
