package filecache

import (
	"crypto/sha256"
	"io"
)

// Cache allows the compiler engine to skip compilation of wasm to machine code
// where doing so is redundant for the same wasm binary and version of wazero.
//
// This augments the default in-memory cache of compiled functions, by
// decoupling it from a wazero.Runtime instance. Concretely, a runtime loses
// its cache once closed. This cache allows the runtime to rebuild its
// in-memory cache quicker, significantly reducing first-hit penalty on a hit.
//
// See New for the example implementation.
type Cache interface {
	// Get is called when the runtime is trying to get the cached compiled functions.
	// Implementations are supposed to return compiled function in io.Reader with ok=true
	// if the key exists on the cache. In the case of not-found, this should return
	// ok=false with err=nil. content.Close() is automatically called by
	// the caller of this Get.
	//
	// Note: the returned content won't go through the validation pass of Wasm binary
	// which is applied when the binary is compiled from scratch without cache hit.
	Get(key Key) (content io.ReadCloser, ok bool, err error)
	//
	// Add is called when the runtime is trying to add the new cache entry.
	// The given `content` must be un-modified, and returned as-is in Get method.
	//
	// Note: the `content` is ensured to be safe through the validation phase applied on the Wasm binary.
	Add(key Key, content io.Reader) (err error)
	//
	// Delete is called when the cache on the `key` returned by Get is no longer usable, and
	// must be purged. Specifically, this is called happens when the wazero's version has been changed.
	// For example, that is when there's a difference between the version of compiling wazero and the
	// version of the currently used wazero.
	Delete(key Key) (err error)
}

// Key represents the 256-bit unique identifier assigned to each cache entry.
type Key = [sha256.Size]byte
