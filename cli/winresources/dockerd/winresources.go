// Package winresources is used to embed Windows resources into dockerd.exe.
//
// These resources are used to provide:
// * Version information
// * An icon
// * A Windows manifest declaring Windows version support
// * Events message table
//
// The resource object files are generated when building with go-winres
// in hack/make/.binary and are located in cli/winresources.
// This occurs automatically when you cross build against Windows OS.
package winresources
