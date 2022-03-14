variable "BUNDLES_OUTPUT" {
  default = "./bundles"
}

target "_common" {
  args = {
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1
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
