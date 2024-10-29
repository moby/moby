// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.1
// 	protoc        v3.11.4
// source: github.com/moby/buildkit/session/filesync/filesync.proto

package filesync

import (
	types "github.com/tonistiigi/fsutil/types"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// BytesMessage contains a chunk of byte data
type BytesMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *BytesMessage) Reset() {
	*x = BytesMessage{}
	mi := &file_github_com_moby_buildkit_session_filesync_filesync_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BytesMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BytesMessage) ProtoMessage() {}

func (x *BytesMessage) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_moby_buildkit_session_filesync_filesync_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BytesMessage.ProtoReflect.Descriptor instead.
func (*BytesMessage) Descriptor() ([]byte, []int) {
	return file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDescGZIP(), []int{0}
}

func (x *BytesMessage) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

var File_github_com_moby_buildkit_session_filesync_filesync_proto protoreflect.FileDescriptor

var file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDesc = []byte{
	0x0a, 0x38, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6d, 0x6f, 0x62,
	0x79, 0x2f, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x6b, 0x69, 0x74, 0x2f, 0x73, 0x65, 0x73, 0x73, 0x69,
	0x6f, 0x6e, 0x2f, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x79, 0x6e, 0x63, 0x2f, 0x66, 0x69, 0x6c, 0x65,
	0x73, 0x79, 0x6e, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x6d, 0x6f, 0x62, 0x79,
	0x2e, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x79, 0x6e, 0x63, 0x2e, 0x76, 0x31, 0x1a, 0x2d, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x74, 0x6f, 0x6e, 0x69, 0x73, 0x74, 0x69,
	0x69, 0x67, 0x69, 0x2f, 0x66, 0x73, 0x75, 0x74, 0x69, 0x6c, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73,
	0x2f, 0x77, 0x69, 0x72, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x22, 0x0a, 0x0c, 0x42,
	0x79, 0x74, 0x65, 0x73, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x64,
	0x61, 0x74, 0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x32,
	0x83, 0x01, 0x0a, 0x08, 0x46, 0x69, 0x6c, 0x65, 0x53, 0x79, 0x6e, 0x63, 0x12, 0x3a, 0x0a, 0x08,
	0x44, 0x69, 0x66, 0x66, 0x43, 0x6f, 0x70, 0x79, 0x12, 0x14, 0x2e, 0x66, 0x73, 0x75, 0x74, 0x69,
	0x6c, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x1a, 0x14,
	0x2e, 0x66, 0x73, 0x75, 0x74, 0x69, 0x6c, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x50, 0x61,
	0x63, 0x6b, 0x65, 0x74, 0x28, 0x01, 0x30, 0x01, 0x12, 0x3b, 0x0a, 0x09, 0x54, 0x61, 0x72, 0x53,
	0x74, 0x72, 0x65, 0x61, 0x6d, 0x12, 0x14, 0x2e, 0x66, 0x73, 0x75, 0x74, 0x69, 0x6c, 0x2e, 0x74,
	0x79, 0x70, 0x65, 0x73, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x65, 0x74, 0x1a, 0x14, 0x2e, 0x66, 0x73,
	0x75, 0x74, 0x69, 0x6c, 0x2e, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x50, 0x61, 0x63, 0x6b, 0x65,
	0x74, 0x28, 0x01, 0x30, 0x01, 0x32, 0x5a, 0x0a, 0x08, 0x46, 0x69, 0x6c, 0x65, 0x53, 0x65, 0x6e,
	0x64, 0x12, 0x4e, 0x0a, 0x08, 0x44, 0x69, 0x66, 0x66, 0x43, 0x6f, 0x70, 0x79, 0x12, 0x1e, 0x2e,
	0x6d, 0x6f, 0x62, 0x79, 0x2e, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x79, 0x6e, 0x63, 0x2e, 0x76, 0x31,
	0x2e, 0x42, 0x79, 0x74, 0x65, 0x73, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x1a, 0x1e, 0x2e,
	0x6d, 0x6f, 0x62, 0x79, 0x2e, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x79, 0x6e, 0x63, 0x2e, 0x76, 0x31,
	0x2e, 0x42, 0x79, 0x74, 0x65, 0x73, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x28, 0x01, 0x30,
	0x01, 0x42, 0x2b, 0x5a, 0x29, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x6d, 0x6f, 0x62, 0x79, 0x2f, 0x62, 0x75, 0x69, 0x6c, 0x64, 0x6b, 0x69, 0x74, 0x2f, 0x73, 0x65,
	0x73, 0x73, 0x69, 0x6f, 0x6e, 0x2f, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x79, 0x6e, 0x63, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDescOnce sync.Once
	file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDescData = file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDesc
)

func file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDescGZIP() []byte {
	file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDescOnce.Do(func() {
		file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDescData = protoimpl.X.CompressGZIP(file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDescData)
	})
	return file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDescData
}

var file_github_com_moby_buildkit_session_filesync_filesync_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_github_com_moby_buildkit_session_filesync_filesync_proto_goTypes = []any{
	(*BytesMessage)(nil), // 0: moby.filesync.v1.BytesMessage
	(*types.Packet)(nil), // 1: fsutil.types.Packet
}
var file_github_com_moby_buildkit_session_filesync_filesync_proto_depIdxs = []int32{
	1, // 0: moby.filesync.v1.FileSync.DiffCopy:input_type -> fsutil.types.Packet
	1, // 1: moby.filesync.v1.FileSync.TarStream:input_type -> fsutil.types.Packet
	0, // 2: moby.filesync.v1.FileSend.DiffCopy:input_type -> moby.filesync.v1.BytesMessage
	1, // 3: moby.filesync.v1.FileSync.DiffCopy:output_type -> fsutil.types.Packet
	1, // 4: moby.filesync.v1.FileSync.TarStream:output_type -> fsutil.types.Packet
	0, // 5: moby.filesync.v1.FileSend.DiffCopy:output_type -> moby.filesync.v1.BytesMessage
	3, // [3:6] is the sub-list for method output_type
	0, // [0:3] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_github_com_moby_buildkit_session_filesync_filesync_proto_init() }
func file_github_com_moby_buildkit_session_filesync_filesync_proto_init() {
	if File_github_com_moby_buildkit_session_filesync_filesync_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   2,
		},
		GoTypes:           file_github_com_moby_buildkit_session_filesync_filesync_proto_goTypes,
		DependencyIndexes: file_github_com_moby_buildkit_session_filesync_filesync_proto_depIdxs,
		MessageInfos:      file_github_com_moby_buildkit_session_filesync_filesync_proto_msgTypes,
	}.Build()
	File_github_com_moby_buildkit_session_filesync_filesync_proto = out.File
	file_github_com_moby_buildkit_session_filesync_filesync_proto_rawDesc = nil
	file_github_com_moby_buildkit_session_filesync_filesync_proto_goTypes = nil
	file_github_com_moby_buildkit_session_filesync_filesync_proto_depIdxs = nil
}
