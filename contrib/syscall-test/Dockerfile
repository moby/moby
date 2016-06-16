FROM buildpack-deps:jessie

COPY . /usr/src/

WORKDIR /usr/src/

RUN gcc -g -Wall -static userns.c -o /usr/bin/userns-test \
	&& gcc -g -Wall -static ns.c -o /usr/bin/ns-test \
	&& gcc -g -Wall -static acct.c -o /usr/bin/acct-test
