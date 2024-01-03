package stack

//go:generate protoc -I=. -I=../../vendor/ --go_out=. --go_opt=paths=source_relative --go_opt=Mstack.proto=/util/stack stack.proto
