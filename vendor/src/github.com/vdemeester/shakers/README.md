# Shakers
🐹 + 🐙 = 😽 [![Circle CI](https://circleci.com/gh/vdemeester/shakers.svg?style=svg)](https://circleci.com/gh/vdemeester/shakers)

A collection of `go-check` Checkers to ease the use of it.

## Building and testing it

You need either [docker](https://github.com/docker/docker), or `go`
and `godep` in order to build and test shakers.

### Using Docker and Makefile

You need to run the ``test-unit`` target. 
```bash
$ make test-unit
docker build -t "shakers-dev:master" .
# […]
docker run --rm -it   "shakers-dev:master" ./script/make.sh test-unit
---> Making bundle: test-unit (in .)
+ go test -cover -coverprofile=cover.out .
ok      github.com/vdemeester/shakers   0.015s  coverage: 96.0% of statements

Test success
```
