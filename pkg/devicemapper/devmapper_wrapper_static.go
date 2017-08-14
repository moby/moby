// +build linux,cgo,static_build

package devicemapper

// #cgo pkg-config: --static devmapper
import "C"
