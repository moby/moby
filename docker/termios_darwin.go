package main

import "syscall"

const (
	getTermios = syscall.TCIOGETA
	setTermios = syscall.TCIOSETA
)
