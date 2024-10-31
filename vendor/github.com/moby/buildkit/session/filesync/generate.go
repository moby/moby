package filesync

//go:generate protoc -I=. -I=../../vendor/ -I=../../vendor/github.com/tonistiigi/fsutil/types/ --gogoslick_out=plugins=grpc:. filesync.proto
