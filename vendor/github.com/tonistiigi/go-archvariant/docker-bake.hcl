variable GO_VERSION {
  default = "1.17"
}

target "_base" {
  args = {
    GO_VERSION = GO_VERSION
  }
}

target "binary" {
  inherits = ["_base"]
  platforms = ["local"]
  output = ["bin"]
}

target "all-arch" {
  inherits = ["_base"]
  platforms = [
    "linux/amd64",
    "linux/arm64",
    "linux/arm",
    "linux/riscv64",
    "linux/386",
    "windows/amd64",
    "windows/arm64",
    "darwin/amd64",
    "darwin/arm64",
  ]
}