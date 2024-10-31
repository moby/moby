package pb

//go:generate protoc -I=. -I=../../vendor/ -I=../../vendor/github.com/gogo/protobuf/ --gogofaster_out=. ops.proto
