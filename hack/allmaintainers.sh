#!/bin/sh

find $1 -name MAINTAINERS -exec cat {} ';' | sed -E -e 's/^[^:]*: *(.*)$/\1/' | grep -E -v -e '^ *$' -e '^ *#.*$' | sort -u
