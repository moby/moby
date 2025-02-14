variable "GO_VERSION" {
  default = null
}

variable "DESTDIR" {
  default = "./bin"
}

variable "GITHUB_REPOSITORY" {
  default = null
}

variable "ACTIONS_CACHE_URL" {
  default = null
}

group "default" {
  targets = ["test"]
}

target "test" {
  target = "test-coverage"
  output = ["${DESTDIR}/coverage"]
  args = {
    GO_VERSION = GO_VERSION
    GITHUB_REPOSITORY = GITHUB_REPOSITORY
    ACTIONS_CACHE_URL = ACTIONS_CACHE_URL
  }
  secret = [
    "id=GITHUB_TOKEN,env=GITHUB_TOKEN",
    "id=ACTIONS_RUNTIME_TOKEN,env=ACTIONS_RUNTIME_TOKEN"
  ]
}

target "validate-gomod" {
  dockerfile = "./hack/dockerfiles/gomod.Dockerfile"
  output = ["type=cacheonly"]
  target = "validate"
  args = {
    GO_VERSION = GO_VERSION
  }
}

target "gomod" {
  inherits = ["validate-gomod"]
  output = ["."]
  target = "update"
}
