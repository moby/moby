FROM debian:bullseye-slim AS base
RUN apt-get update && apt-get --no-install-recommends install -y \
  gcc-x86-64-linux-gnu \
  binutils-arm-linux-gnueabihf \
  binutils-aarch64-linux-gnu \
  binutils-i686-linux-gnu \
  binutils-riscv64-linux-gnu \
  binutils-s390x-linux-gnu \
  binutils-powerpc64le-linux-gnu \
  binutils-mips64el-linux-gnuabi64 \
  binutils-mips64-linux-gnuabi64
WORKDIR /src


FROM base AS exit-amd64
COPY fixtures/exit.amd64.S .
RUN x86_64-linux-gnu-gcc -static -nostdlib -o exit exit.amd64.S

FROM base AS exit-386
COPY fixtures/exit.386.s .
RUN i686-linux-gnu-as --noexecstack -o exit.o exit.386.s && i686-linux-gnu-ld -o exit -s exit.o

FROM base AS exit-arm64
COPY fixtures/exit.arm64.s .
RUN aarch64-linux-gnu-as --noexecstack -o exit.o exit.arm64.s && aarch64-linux-gnu-ld -o exit -s exit.o

FROM base AS exit-arm
COPY fixtures/exit.arm.s .
RUN arm-linux-gnueabihf-as --noexecstack -o exit.o exit.arm.s && arm-linux-gnueabihf-ld -o exit -s exit.o

FROM base AS exit-riscv64
COPY fixtures/exit.riscv64.s .
RUN riscv64-linux-gnu-as --noexecstack -o exit.o exit.riscv64.s && riscv64-linux-gnu-ld -o exit -s exit.o

FROM base AS exit-s390x
COPY fixtures/exit.s390x.s .
RUN s390x-linux-gnu-as --noexecstack -o exit.o exit.s390x.s && s390x-linux-gnu-ld -o exit -s exit.o

FROM base AS exit-ppc64
COPY fixtures/exit.ppc64.s .
RUN powerpc64le-linux-gnu-as -mbig --noexecstack -o exit.o exit.ppc64.s && powerpc64le-linux-gnu-ld -EB -o exit -s exit.o

FROM base AS exit-ppc64le
COPY fixtures/exit.ppc64le.s .
RUN powerpc64le-linux-gnu-as --noexecstack -o exit.o exit.ppc64le.s && powerpc64le-linux-gnu-ld -o exit -s exit.o

FROM base AS exit-mips64le
COPY fixtures/exit.mips64le.s .
RUN mips64el-linux-gnuabi64-as --noexecstack -o exit.o exit.mips64le.s && mips64el-linux-gnuabi64-ld -o exit -s exit.o

FROM base AS exit-mips64
COPY fixtures/exit.mips64.s .
RUN mips64-linux-gnuabi64-as --noexecstack -o exit.o exit.mips64.s && mips64-linux-gnuabi64-ld -o exit -s exit.o

FROM golang:1.19-alpine AS generate
WORKDIR /src
COPY --from=exit-amd64 /src/exit amd64
COPY --from=exit-386 /src/exit 386
COPY --from=exit-arm64 /src/exit arm64
COPY --from=exit-arm /src/exit arm
COPY --from=exit-riscv64 /src/exit riscv64
COPY --from=exit-s390x /src/exit s390x
COPY --from=exit-ppc64 /src/exit ppc64
COPY --from=exit-ppc64le /src/exit ppc64le
COPY --from=exit-mips64le /src/exit mips64le
COPY --from=exit-mips64 /src/exit mips64
COPY generate.go .

RUN go run generate.go amd64 386 arm64 arm riscv64 s390x ppc64 ppc64le mips64le mips64 && ls -l


FROM scratch
COPY --from=generate /src/*_binary.go  /
