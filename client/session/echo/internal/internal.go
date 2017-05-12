package internal

import "google.golang.org/grpc"

func EchoServiceDesc() *grpc.ServiceDesc {
	return &_Echo_serviceDesc
}
