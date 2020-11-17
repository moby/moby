// +build !s390x

package archutil

func s390xSupported() error {
	return check(Binarys390x)
}
