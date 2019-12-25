package moby_buildkit_v1_frontend

//go:generate protoc -I=. -I=../../../vendor/ -I=../../../../../../ --gogo_out=plugins=grpc:. gateway.proto
