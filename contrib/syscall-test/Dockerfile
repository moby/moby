FROM buildpack-deps:jessie

COPY . /usr/src/

WORKDIR /usr/src/

RUN gcc -g -Wall -static userns.c -o /usr/bin/userns-test \
	&& gcc -g -Wall -static ns.c -o /usr/bin/ns-test \
	&& gcc -g -Wall -static acct.c -o /usr/bin/acct-test

RUN [ "$(uname -m)" = "x86_64" ] && gcc -s -m32 -nostdlib exit32.s -o /usr/bin/exit32-test || true
