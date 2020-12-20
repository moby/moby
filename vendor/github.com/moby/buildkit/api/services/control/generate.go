package moby_buildkit_v1 //nolint:golint

//go:generate protoc -I=. -I=../../../vendor/ -I=../../../../../../ --gogo_out=plugins=grpc:. control.proto
