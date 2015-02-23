// +build linux

package nsenter

/*
void nsenter();
__attribute__((constructor)) int init() {
	nsenter();
}
*/
import "C"
