// +build linux freebsd darwin openbsd solaris

package layer

import(
	//import package string
	"github.com/docker/docker/pkg/stringid"
)
func (ls *layerStore) mountID(name string) string {
	//randomId
	return stringid.GenerateRandomID()
}
