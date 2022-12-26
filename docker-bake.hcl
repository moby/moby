variable "APT_MIRROR" {
  default = "cdn-fastly.deb.debian.org"
}
variable "DOCKER_DEBUG" {
  default = ""
}
variable "DOCKER_STATIC" {
  default = "1"
}
variable "DOCKER_LDFLAGS" {
  default = ""
}
variable "DOCKER_BUILDTAGS" {
  default = ""
}
variable "DOCKER_GITCOMMIT" {
  default = "HEAD"
}

# Docker version such as 23.0.0-dev. Automatically generated through Git ref.
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

# GITHUB_REF is the actual ref that triggers the workflow and used as version
# when tag is pushed: https://docs.github.com/en/actions/learn-github-actions/environment-variables#default-environment-variables
variable "GITHUB_REF" {
  default = ""
}

# GITHUB_SHA is the commit SHA that triggered the workflow and used as commit.
# https://docs.github.com/en/actions/learn-github-actions/environment-variables#default-environment-variables
variable "GITHUB_SHA" {
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
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1
    APT_MIRROR = APT_MIRROR
    DOCKER_DEBUG = DOCKER_DEBUG
    DOCKER_STATIC = DOCKER_STATIC
    DOCKER_LDFLAGS = DOCKER_LDFLAGS
    DOCKER_BUILDTAGS = DOCKER_BUILDTAGS
    DOCKER_GITCOMMIT = DOCKER_GITCOMMIT != "" ? DOCKER_GITCOMMIT : GITHUB_SHA
    VERSION = VERSION != "" ? VERSION : GITHUB_REF
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
    "windows/amd64"
  ]
}

#
# build dockerd and docker-proxy
#

target "binary" {
  inherits = ["_common"]
  target = "binary"
  output = [bindir(DOCKER_STATIC == "1" ? "binary" : "dynbinary")]
}

target "dynbinary" {
  inherits = ["binary"]
  output = [bindir("dynbinary")]
  args = {
    DOCKER_STATIC = "0"
  }
}

target "binary-cross" {
  inherits = ["binary", "_platforms"]
}

#
# dev
#

variable "DEV_IMAGE" {
  default = "docker-dev"
}
variable "SYSTEMD" {
  default = "false"
}

target "dev" {
  inherits = ["_common"]
  target = "final"
  args = {
    SYSTEMD = SYSTEMD
  }
  tags = [DEV_IMAGE]
  output = ["type=docker"]
}
