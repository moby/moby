//go:build !s390x
// +build !s390x

package archutil

func s390xSupported() (string, error) {
	return check("390x", Binarys390x)
}
