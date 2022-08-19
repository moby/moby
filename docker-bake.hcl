variable "APT_MIRROR" {
  default = "deb.debian.org"
}
variable "DOCKER_LINKMODE" {
  default = "static"
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
    DOCKER_LINKMODE = DOCKER_LINKMODE
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
