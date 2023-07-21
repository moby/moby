#!/bin/bash

for i in *.go; do golint $i | grep -q . || echo $i; done > .linted
