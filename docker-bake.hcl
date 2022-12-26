variable "DOCKER_STATIC" {
  default = "1"
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
    APT_MIRROR = "cdn-fastly.deb.debian.org"
    DOCKER_STATIC = DOCKER_STATIC
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
