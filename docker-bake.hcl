variable "APT_MIRROR" {
  default = "deb.debian.org"
}
variable "DOCKER_DEBUG" {
  default = ""
}
variable "DOCKER_STRIP" {
  default = ""
}
variable "DOCKER_LINKMODE" {
  default = "static"
}
variable "DOCKER_LDFLAGS" {
  default = ""
}
variable "DOCKER_BUILDMODE" {
  default = ""
}
variable "DOCKER_BUILDTAGS" {
  default = ""
}

# Docker version such as 17.04.0-dev. Automatically generated through Git ref.
variable "VERSION" {
  default = ""
}

# The platform name, such as "Docker Engine - Community".
variable "PLATFORM" {
  default = ""
}

# The product name, used to set version.ProductName, which is used to set
# BuildKit's ExportedProduct variable in order to show useful error messages
# to users when a certain version of the product doesn't support a BuildKit feature.
variable "PRODUCT" {
  default = ""
}

# Sets the version.DefaultProductLicense string, such as "Community Engine".
# This field can contain a summary of the product license of the daemon if a
# commercial license has been applied to the daemon.
variable "DEFAULT_PRODUCT_LICENSE" {
  default = ""
}

# The name of the packager (e.g. "Docker, Inc."). This used to set CompanyName
# in the manifest.
variable "PACKAGER_NAME" {
  default = ""
}

# Defines the output folder
variable "DESTDIR" {
  default = ""
}
function "bindir" {
  params = [defaultdir]
  result = DESTDIR != "" ? DESTDIR : "./bundles/${defaultdir}"
}

target "_common" {
  args = {
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1 # https://github.com/moby/buildkit/blob/master/frontend/dockerfile/docs/syntax.md#built-in-build-args
    APT_MIRROR = APT_MIRROR
    DOCKER_DEBUG = DOCKER_DEBUG
    DOCKER_STRIP = DOCKER_STRIP
    DOCKER_LINKMODE = DOCKER_LINKMODE
    DOCKER_LDFLAGS = DOCKER_LDFLAGS
    DOCKER_BUILDMODE = DOCKER_BUILDMODE
    DOCKER_BUILDTAGS = DOCKER_BUILDTAGS
    VERSION = VERSION
    PLATFORM = PLATFORM
    PRODUCT = PRODUCT
    DEFAULT_PRODUCT_LICENSE = DEFAULT_PRODUCT_LICENSE
    PACKAGER_NAME = PACKAGER_NAME
  }
}

group "default" {
  targets = ["binary"]
}

target "_platforms" {
  platforms = [
    "linux/amd64",
    "linux/arm/v5",
    "linux/arm/v6",
    "linux/arm/v7",
    "linux/arm64",
    "linux/ppc64le",
    "linux/s390x",
    "windows/amd64",
    "windows/arm64"
  ]
}

#
# binaries targets build dockerd, docker-proxy and docker-init
#

target "binary" {
  inherits = ["_common"]
  target = "binary"
  output = [bindir(DOCKER_LINKMODE == "static" ? "binary" : "dynbinary")]
}

target "binary-cross" {
  inherits = ["binary", "_platforms"]
}

target "binary-smoketest" {
  inherits = ["_common"]
  target = "smoketest"
  output = ["type=cacheonly"]
  platforms = [
    "linux/amd64",
    "linux/arm64",
    "linux/ppc64le",
    "linux/s390x"
  ]
}

#
# all targets build binaries and extra tools as well (containerd, runc, ...)
#

target "all" {
  inherits = ["_common"]
  target = "all"
  output = [bindir("all")]
}

target "all-cross" {
  inherits = ["all", "_platforms"]
}

#
# dev
#

variable "SYSTEMD" {
  default = "false"
}

target "dev" {
  inherits = ["_common"]
  target = "dev"
  args = {
    SYSTEMD = SYSTEMD
  }
  tags = ["docker-dev"]
  output = ["type=docker"]
}

#
# simple
#

target "simple" {
  inherits = ["_common"]
  dockerfile = "Dockerfile.simple"
  tags = ["docker:simple"]
  output = ["type=docker"]
  contexts = {
    tini = "target:_tini"
    runc = "target:_runc"
    containerd = "target:_containerd"
    rootlesskit = "target:_rootlesskit"
    dockercli = "target:_dockercli"
  }
}

target "_tini" {
  inherits = ["_common"]
  target = "tini"
}
target "_runc" {
  inherits = ["_common"]
  target = "runc"
}
target "_containerd" {
  inherits = ["_common"]
  target = "containerd"
}
target "_rootlesskit" {
  inherits = ["_common"]
  target = "rootlesskit"
}
target "_dockercli" {
  inherits = ["_common"]
  target = "dockercli"
}
