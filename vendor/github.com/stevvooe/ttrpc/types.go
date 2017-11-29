package ttrpc

import (
	"fmt"

	spb "google.golang.org/genproto/googleapis/rpc/status"
)

type Request struct {
	Service string `protobuf:"bytes,1,opt,name=service,proto3"`
	Method  string `protobuf:"bytes,2,opt,name=method,proto3"`
	Payload []byte `protobuf:"bytes,3,opt,name=payload,proto3"`
}

func (r *Request) Reset()         { *r = Request{} }
func (r *Request) String() string { return fmt.Sprintf("%+#v", r) }
func (r *Request) ProtoMessage()  {}

type Response struct {
	Status  *spb.Status `protobuf:"bytes,1,opt,name=status,proto3"`
	Payload []byte      `protobuf:"bytes,2,opt,name=payload,proto3"`
}

func (r *Response) Reset()         { *r = Response{} }
func (r *Response) String() string { return fmt.Sprintf("%+#v", r) }
func (r *Response) ProtoMessage()  {}
