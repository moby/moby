//go:generate protoc -I=../../../.. --go_out=../../../.. --go_opt=paths=source_relative --go-vtproto_out=../../../.. --go-vtproto_opt=paths=source_relative,features=marshal+unmarshal+size daemon/logger/internal/logdriver/entry.proto

package logdriver
