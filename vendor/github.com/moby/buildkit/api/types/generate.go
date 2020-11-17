package moby_buildkit_v1_types //nolint:golint

//go:generate protoc -I=. -I=../../vendor/ -I=../../../../../ --gogo_out=plugins=grpc:. worker.proto
