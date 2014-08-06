// +build linux

package namespaces

/*
__attribute__((constructor)) init() {
	nsenter();
}
*/
import "C"
