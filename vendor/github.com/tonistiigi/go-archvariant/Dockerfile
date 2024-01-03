ARG GO_VERSION=1.17

FROM --platform=$BUILDPLATFORM tonistiigi/xx AS xx

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
COPY --from=xx / /
RUN apk add --no-cache git
WORKDIR /src
ARG TARGETPLATFORM
RUN --mount=target=. \
  TARGETPLATFORM=$TARGETPLATFORM xx-go build -o /out/amd64variant ./cmd/amd64variant &&  \
    xx-verify --static /out/amd64variant

FROM scratch
COPY --from=build /out/amd64variant .