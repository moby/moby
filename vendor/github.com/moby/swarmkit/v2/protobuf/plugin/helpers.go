package plugin

import (
	"github.com/gogo/protobuf/proto"
	google_protobuf "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
)

// DeepcopyEnabled returns true if deepcopy is enabled for the descriptor.
func DeepcopyEnabled(options *google_protobuf.MessageOptions) bool {
	return proto.GetBoolExtension(options, E_Deepcopy, true)
}
