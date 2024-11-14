variable "GO_VERSION" {
  default = null
}

variable "DESTDIR" {
  default = "./bin"
}

target "_platforms" {
  platforms = [
    "darwin/amd64",
    "darwin/arm64",
    "freebsd/amd64",
    "freebsd/arm64",
    "linux/386",
    "linux/amd64",
    "linux/arm",
    "linux/arm64",
    "linux/ppc64le",
    "linux/s390x",
    "netbsd/amd64",
    "netbsd/arm64",
    "openbsd/amd64",
    "openbsd/arm64",
    "windows/amd64",
    "windows/arm64"
  ]
}

target "_common" {
  args = {
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1
  }
}

group "default" {
  targets = ["build"]
}

target "build" {
  inherits = ["_common"]
  args = {
    GO_VERSION = "${GO_VERSION}"
  }
}

group "test" {
  targets = ["test-root", "test-noroot"]
}

target "test-root" {
  inherits = ["build"]
  target = "test-coverage"
  output = ["${DESTDIR}/coverage"]
}

target "test-noroot" {
  inherits = ["build"]
  target = "test-noroot-coverage"
  output = ["${DESTDIR}/coverage"]
}

target "lint" {
  inherits = ["_common"]
  dockerfile = "./hack/dockerfiles/lint.Dockerfile"
  output = ["type=cacheonly"]
  args = {
    GO_VERSION = "${GO_VERSION}"
  }
}

target "lint-cross" {
  inherits = ["lint", "_platforms"]
}

target "validate-generated-files" {
  inherits = ["_common"]
  dockerfile = "./hack/dockerfiles/generated-files.Dockerfile"
  output = ["type=cacheonly"]
  target = "validate"
  args = {
    GO_VERSION = "${GO_VERSION}"
  }
}

target "generated-files" {
  inherits = ["validate-generated-files"]
  output = ["."]
  target = "update"
}

target "validate-gomod" {
  inherits = ["_common"]
  dockerfile = "./hack/dockerfiles/gomod.Dockerfile"
  output = ["type=cacheonly"]
  target = "validate"
  args = {
    # go mod may produce different results between go versions,
    # if this becomes a problem, this should be switched to use
    # a fixed go version.
    GO_VERSION = "${GO_VERSION}"
  }
}

target "gomod" {
  inherits = ["validate-gomod"]
  output = ["."]
  target = "update"
}

target "validate-shfmt" {
  inherits = ["_common"]
  dockerfile = "./hack/dockerfiles/shfmt.Dockerfile"
  output = ["type=cacheonly"]
  target = "validate"
}

target "shfmt" {
  inherits = ["validate-shfmt"]
  output = ["."]
  target = "update"
}

target "cross" {
  inherits = ["build", "_platforms"]
}
