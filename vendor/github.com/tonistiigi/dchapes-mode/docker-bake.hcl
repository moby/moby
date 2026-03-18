variable "GO_VERSION" {
  default = null
}

group "default" {
  targets = ["build"]
}

target "build" {
  args = {
    GO_VERSION = GO_VERSION
  }
  output = ["type=cacheonly"]
}

target "test" {
  inherits = ["build"]
  target = "test"
}

target "cross" {
  inherits = ["build"]
  platforms = ["linux/amd64", "linux/386", "linux/arm64", "linux/arm", "linux/ppc64le", "linux/s390x", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64", "freebsd/amd64", "freebsd/arm64"]
}