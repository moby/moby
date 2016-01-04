FROM debian:jessie

RUN apt-get update && apt-get install -y \
	gcc \
	libc6-dev \
	--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

COPY . /usr/src/

WORKDIR /usr/src/

RUN gcc -g -Wall -static userns.c -o /usr/bin/userns-test \
	&& gcc -g -Wall -static ns.c -o /usr/bin/ns-test \
	&& gcc -g -Wall -static acct.c -o /usr/bin/acct-test
