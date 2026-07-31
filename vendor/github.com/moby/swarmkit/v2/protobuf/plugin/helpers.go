package plugin

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// DeepcopyEnabled returns true if deepcopy is enabled for the descriptor.
func DeepcopyEnabled(options *descriptorpb.MessageOptions) bool {
	if options == nil {
		return true
	}
	// The extension defaults to true, and proto.GetExtension already honours
	// the default for an unpopulated proto2 field.
	enabled, ok := proto.GetExtension(options, E_Deepcopy).(bool)
	return !ok || enabled
}
