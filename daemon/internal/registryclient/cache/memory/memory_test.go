package memory

import (
	"testing"
)

// TestInMemoryBlobInfoCache checks the in memory implementation is working
// correctly.
func TestInMemoryBlobInfoCache(t *testing.T) {
	CheckBlobDescriptorCache(t, NewInMemoryBlobDescriptorCacheProvider())
}
