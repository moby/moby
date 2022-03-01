variable "GO_VERSION" {
  default = "1.16"
}

group "default" {
  targets = ["build"]
}

target "build" {
  args = {
    GO_VERSION = "${GO_VERSION}"
  }
}

group "test" {
  targets = ["test-root", "test-noroot"]
}

target "test-root" {
  inherits = ["build"]
  target = "test"
}

target "test-noroot" {
  inherits = ["build"]
  target = "test-noroot"
}

target "lint" {
  dockerfile = "./hack/dockerfiles/lint.Dockerfile"
  args = {
    GO_VERSION = "${GO_VERSION}"
  }
}

target "validate-gomod" {
  dockerfile = "./hack/dockerfiles/gomod.Dockerfile"
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
  dockerfile = "./hack/dockerfiles/shfmt.Dockerfile"
  target = "validate"
}

target "shfmt" {
  inherits = ["validate-shfmt"]
  output = ["."]
  target = "update"
}

target "cross" {
  inherits = ["build"]
  platforms = ["linux/amd64", "linux/386", "linux/arm64", "linux/arm", "linux/ppc64le", "linux/s390x", "darwin/amd64", "darwin/arm64", "windows/amd64", "freebsd/amd64", "freebsd/arm64"]
}
