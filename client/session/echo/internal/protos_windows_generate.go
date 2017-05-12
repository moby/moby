// +build windows

package internal

//go:generate protoc --gogoslick_out=plugins=grpc:. echo.proto -I..\..\..\..\vendor\;..\..\..\..\vendor\github.com\gogo\protobuf\protobuf\;.
