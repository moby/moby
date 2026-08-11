package protogen

import (
	"os"
	"sort"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"gotest.tools/v3/assert"
)

// TestWireContractIsUnchanged verifies the runtime service against its golden
// descriptor. This protocol cannot be changed in place because separately built
// extensions speak it during startup. The comparison ignores Go-level details
// but requires every wire identifier, field, type, cardinality, and reservation
// to match. Deliberate changes require a new service version.
func TestWireContractIsUnchanged(t *testing.T) {
	golden, err := os.ReadFile("testdata/runtime_v1.textproto")
	assert.NilError(t, err)

	var want descriptorpb.FileDescriptorProto
	assert.NilError(t, prototext.Unmarshal(golden, &want))

	got := normalize(protodesc.ToFileDescriptorProto(File_internal_extensions_sdk_sdkapi_extension_proto))
	if !proto.Equal(got, &want) {
		t.Fatalf("the extension runtime wire contract changed.\n--- want (protoc, runtime.proto)\n%s\n--- got (mobyextgen, Go-first)\n%s",
			prototext.Format(&want), prototext.Format(got))
	}

	// Check the identifiers used by deployed extensions.
	assert.Equal(t, got.GetPackage(), "moby.extension.runtime.v1")
	assert.Equal(t, serviceName, "moby.extension.runtime.v1.Extension")
}

func normalize(fd *descriptorpb.FileDescriptorProto) *descriptorpb.FileDescriptorProto {
	fd = proto.Clone(fd).(*descriptorpb.FileDescriptorProto)
	fd.Name = nil
	fd.Options = nil
	sort.Slice(fd.MessageType, func(i, j int) bool { return fd.MessageType[i].GetName() < fd.MessageType[j].GetName() })
	for _, m := range fd.MessageType {
		sort.Slice(m.Field, func(i, j int) bool { return m.Field[i].GetNumber() < m.Field[j].GetNumber() })
	}
	return fd
}
