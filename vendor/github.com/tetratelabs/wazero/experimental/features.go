package experimental

import "github.com/tetratelabs/wazero/api"

// CoreFeaturesThreads enables threads instructions ("threads").
//
// # Notes
//
//   - The instruction list is too long to enumerate in godoc.
//     See https://github.com/WebAssembly/threads/blob/main/proposals/threads/Overview.md
//   - Atomic operations are guest-only until api.Memory or otherwise expose them to host functions.
//   - On systems without mmap available, the memory will pre-allocate to the maximum size. Many
//     binaries will use a theroetical maximum like 4GB, so if using such a binary on a system
//     without mmap, consider editing the binary to reduce the max size setting of memory.
const CoreFeaturesThreads = api.CoreFeatureSIMD << 1
