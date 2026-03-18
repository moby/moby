variable "COVER_FILENAME" {
    default = null
}

variable "BENCH_FILENAME" {
    default = null
}

variable "GO_VERSION" {
    default = null
}

target "default" {
    targets = ["build"]
}

target "_all_platforms" {
    platforms = [
        "linux/amd64",
        "linux/arm64",
        "linux/arm/v7",
        "linux/arm/v6",
        "linux/386",
        "linux/ppc64le",
        "linux/s390x",
        "darwin/amd64",
        "darwin/arm64",
        "windows/amd64",
    ]
}

target "build" {
    output = ["type=cacheonly"]
    args = {
        GO_VERSION = GO_VERSION
    }
}

target "build-all" {
    inherits = ["build", "_all_platforms"]
}

target "test" {
    target = "test"
    args = {
        COVER_FILENAME = COVER_FILENAME
        GO_VERSION = GO_VERSION
    }
    output = [COVER_FILENAME!=null?".":"type=cacheonly"]
}

target "bench" {
    target = "bench"
    args = {
        BENCH_FILENAME = BENCH_FILENAME
        GO_VERSION = GO_VERSION
    }
    output = [BENCH_FILENAME!=null?".":"type=cacheonly"]
}

target "lint" {
    dockerfile = "hack/dockerfiles/lint.Dockerfile"
    output = ["type=cacheonly"]
}

target "lint-all" {
    inherits = ["lint", "_all_platforms"]
}