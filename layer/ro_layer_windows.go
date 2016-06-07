package layer

import "github.com/docker/distribution"

var _ ForeignSourcer = &roLayer{}

func (rl *roLayer) ForeignSource() *distribution.Descriptor {
	return rl.foreignSrc
}
