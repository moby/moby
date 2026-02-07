variable "ROOT_SIGNING_VERSION" {
    type    = string
    # default = "8842feefbb65effea46ff4a0f2b6aad91e685fe9" # expired root
    # default = "9d8b5c5e3bed603c80b57fcc316b7a1af688c57e" # expired timestamp
    default = "a72700d5c80d43a209d31325fee46facc6f0cf31"
    description = "The git commit hash of sigstore/root-signing to use for embedded roots."
}

variable "DOCKER_HARDENED_IMAGES_KEYRING_VERSION" {
    type    = string
    default = "04ae44966821da8e5cdcb4c51137dee69297161a"
    description = "The git branch or commit hash of docker/hardened-images-keyring to use for DHI verification."
}

target "_common" {
  args = {
    BUILDKIT_CONTEXT_KEEP_GIT_DIR = 1
  }
}

target "tuf-root" {
    target = "tuf-root"
    output = [{
        type = "local",
        dest = "roots/tuf-root"
    }]
    args = {
        ROOT_SIGNING_VERSION = ROOT_SIGNING_VERSION
    }
}

target "validate-tuf-root" {
    target = "validate-tuf-root"
    output = [{
        type = "cacheonly"
    }]
    args = {
        ROOT_SIGNING_VERSION = ROOT_SIGNING_VERSION
    }
}

group "validate-all" {
  targets = ["lint", "lint-gopls", "validate-vendor", "validate-dockerfile", "validate-generated-files"]
}

group "validate-generated-files" {
  targets = ["validate-tuf-root"]
}

target "lint" {
  dockerfile = "./hack/dockerfiles/lint.Dockerfile"
  output = ["type=cacheonly"]
  args = {
    GOLANGCI_FROM_SOURCE = "true"
  }
}

target "validate-vendor" {
  inherits = ["_common"]
  dockerfile = "./hack/dockerfiles/vendor.Dockerfile"
  target = "validate"
  output = ["type=cacheonly"]
}

target "validate-dockerfile" {
  matrix = {
    dockerfile = [
      "Dockerfile",
      "./hack/dockerfiles/lint.Dockerfile",
      "./hack/dockerfiles/vendor.Dockerfile"
    ]
  }
  name = "validate-dockerfile-${md5(dockerfile)}"
  dockerfile = dockerfile
  call = "check"
}

target "lint-gopls" {
    inherits = [ "lint" ]
    target = "gopls-analyze"
}

target "vendor" {
  inherits = ["_common"]
  dockerfile = "./hack/dockerfiles/vendor.Dockerfile"
  target = "update"
  output = ["."]
}

target "mod-outdated" {
  inherits = ["_common"]
  dockerfile = "./hack/dockerfiles/vendor.Dockerfile"
  target = "outdated"
  no-cache-filter = ["outdated"]
  output = ["type=cacheonly"]
}

target "binary" {
    target = "binary"
    platforms = [ "local" ]
    output = [{
        type = "local",
        dest = "bin/"
    }]
}

target "_all_platforms" {
  platforms = [
    "freebsd/amd64",
    "linux/amd64",
    "linux/arm64",
    "linux/s390x",
    "linux/ppc64le",
    "linux/riscv64",
    "windows/amd64",
    "windows/arm64",
    "darwin/amd64",
    "darwin/arm64",
  ]
}

target "binary-all" {
    inherits = [ "binary", "_all_platforms" ]
}

target "dhi-pubkey" {
    target = "dhi-pubkey"
    output = [{
        type = "local",
        dest = "roots/dhi"
    }]
    args = {
        DOCKER_HARDENED_IMAGES_KEYRING_VERSION = DOCKER_HARDENED_IMAGES_KEYRING_VERSION
    }
}

target "validate-dhi-pubkey" {
    target = "validate-dhi-pubkey"
    output = [{
        type = "cacheonly"
    }]
    args = {
        DOCKER_HARDENED_IMAGES_KEYRING_VERSION = DOCKER_HARDENED_IMAGES_KEYRING_VERSION
    }
}
