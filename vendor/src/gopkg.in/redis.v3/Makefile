all: testdeps
	go test ./... -test.v -test.cpu=1,2,4
	go test ./... -test.v -test.short -test.race

testdeps: .test/redis/src/redis-server

.PHONY: all test testdeps

.test/redis:
	mkdir -p $@
	wget -qO- https://github.com/antirez/redis/archive/unstable.tar.gz | tar xvz --strip-components=1 -C $@

.test/redis/src/redis-server: .test/redis
	cd $< && make all
