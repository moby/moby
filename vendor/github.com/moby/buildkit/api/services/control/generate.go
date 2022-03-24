package moby_buildkit_v1 //nolint:revive

//go:generate protoc -I=. -I=../../../vendor/ -I=../../../../../../ --gogo_out=plugins=grpc:. control.proto
