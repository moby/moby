TARGETS?=docker
MODULES?=${TARGETS:=.pp.bz2}
SHAREDIR?=/usr/share

all: ${TARGETS:=.pp.bz2}

%.pp.bz2: %.pp
	@echo Compressing $^ -\> $@
	bzip2 -9 $^

%.pp: %.te
	make -f ${SHAREDIR}/selinux/devel/Makefile $@

clean:
	rm -f *~  *.tc *.pp *.pp.bz2
	rm -rf tmp *.tar.gz

man: install-policy
	sepolicy manpage --path . --domain ${TARGETS}_t

install-policy: all
	semodule -i ${TARGETS}.pp.bz2

install: man
	install -D -m 644 ${TARGETS}.pp.bz2 ${DESTDIR}${SHAREDIR}/selinux/packages/docker.pp.bz2
	install -D -m 644 docker.if ${DESTDIR}${SHAREDIR}/selinux/devel/include/services/docker.if
	install -D -m 644 docker_selinux.8 ${DESTDIR}${SHAREDIR}/man/man8/
