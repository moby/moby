#!groovy

def printInfo() {
    sh 'docker version'
    sh 'docker info'
    sh '''
    echo "check-config.sh version: ${CHECK_CONFIG_COMMIT}"
    curl -fsSL -o ${WORKSPACE}/check-config.sh "https://raw.githubusercontent.com/moby/moby/${CHECK_CONFIG_COMMIT}/contrib/check-config.sh" \
    && bash ${WORKSPACE}/check-config.sh || true
    '''
}

def buildDevImage() {
    sh 'docker build --force-rm --build-arg APT_MIRROR -t docker:${GIT_COMMIT} .'
}

def runUnitTest() {
    sh '''
    docker run --rm -t --privileged \
      -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
      --name docker-pr$BUILD_NUMBER \
      -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
      -e DOCKER_GRAPHDRIVER \
      docker:${GIT_COMMIT} \
      hack/test/unit
    '''
}

def cleanupContainer() {
    sh '''
    echo 'Ensuring container killed.'
    docker rm -vf docker-pr$BUILD_NUMBER || true
    '''
}

def chownWorkSpace() {
    sh '''
    echo 'Chowning /workspace to jenkins user'
    docker run --rm -v "$WORKSPACE:/workspace" busybox chown -R "$(id -u):$(id -g)" /workspace
    '''
}

return this
