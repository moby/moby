# syntax=docker/dockerfile:1.20.0@sha256:26147acbda4f14c5add9946e2fd2ed543fc402884fd75146bd342a7f6271dc1d

# NOTE: the image digest is not pinned to a specific version,
# as the image is frequently updated with apt security updates.
#
# TODO: consider pinning for better build reproducibility.
# A reproduction build will need executing apt-get with snapshot mode.
ARG BASE_DEBIAN_DISTRO=trixie

FROM debian:${BASE_DEBIAN_DISTRO}-slim
RUN apt-get update && apt-get install -y gcc libc6-dev --no-install-recommends

COPY . /usr/src/

WORKDIR /usr/src/

RUN gcc -g -Wall -static nnp-test.c -o /usr/bin/nnp-test

RUN chmod +s /usr/bin/nnp-test
