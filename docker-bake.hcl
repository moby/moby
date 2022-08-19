variable "APT_MIRROR" {
  default = "deb.debian.org"
}
variable "BUNDLES_OUTPUT" {
  default = "./bundles"
}
variable "DOCKER_CROSSPLATFORMS" {
  default = ""
}

target "_common" {
  args = {
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1
    APT_MIRROR = APT_MIRROR
  }
}

group "default" {
  targets = ["binary"]
}

target "binary" {
  inherits = ["_common"]
  target = "binary"
  output = [BUNDLES_OUTPUT]
}

target "dynbinary" {
  inherits = ["binary"]
  target = "dynbinary"
}

target "cross" {
  inherits = ["binary"]
  args = {
    CROSS = "true"
    DOCKER_CROSSPLATFORMS = DOCKER_CROSSPLATFORMS
  }
  target = "cross"
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
