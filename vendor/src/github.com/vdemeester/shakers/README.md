# Shakers
🐹 + 🐙 = 😽 [![Circle CI](https://circleci.com/gh/vdemeester/shakers.svg?style=svg)](https://circleci.com/gh/vdemeester/shakers)

A collection of `go-check` Checkers to ease the use of it.

## Building and testing it

You need either [docker](https://github.com/docker/docker), or `go`
and `glide` in order to build and test shakers.

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

### Using glide and `GO15VENDOREXPERIMENT`

- Get the dependencies with `glide up` (or use `go get` but you have no garantuees over the version of the dependencies)
- If you're using glide (and not standard `go get`) export `GO15VENDOREXPERIMENT` with `export GO15VENDOREXPERIMENT=1`
- Run tests with `go test .`
