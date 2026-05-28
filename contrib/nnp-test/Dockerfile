# syntax=docker/dockerfile:1

ARG BASE_DEBIAN_DISTRO=trixie

FROM debian:${BASE_DEBIAN_DISTRO}-slim
RUN apt-get update && apt-get install -y gcc libc6-dev --no-install-recommends

COPY . /usr/src/

WORKDIR /usr/src/

RUN gcc -g -Wall -static nnp-test.c -o /usr/bin/nnp-test

RUN chmod +s /usr/bin/nnp-test
