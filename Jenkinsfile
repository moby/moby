def withGithubStatus(String context, Closure cl) {
  def setGithubStatus = { String state ->
    try {
      def backref = "${BUILD_URL}flowGraphTable/"
      def reposSourceURL = scm.repositories[0].getURIs()[0].toString()
      step(
        $class: 'GitHubCommitStatusSetter',
        contextSource: [$class: "ManuallyEnteredCommitContextSource", context: context],
        errorHandlers: [[$class: 'ShallowAnyErrorHandler']],
        reposSource: [$class: 'ManuallyEnteredRepositorySource', url: reposSourceURL],
        statusBackrefSource: [$class: 'ManuallyEnteredBackrefSource', backref: backref],
        statusResultSource: [$class: 'ConditionalStatusResultSource', results: [[$class: 'AnyBuildResult', state: state]]],
      )
    } catch (err) {
      echo "Exception from GitHubCommitStatusSetter for $context: $err"
    }
  }

  setGithubStatus 'PENDING'

  try {
    cl()
	} catch (err) {
    // AbortException signals a "normal" build failure.
    if (!(err instanceof hudson.AbortException)) {
      echo "Exception in withGithubStatus for $context: $err"
		}
		setGithubStatus 'FAILURE'
		throw err
	}
	setGithubStatus 'SUCCESS'
}


pipeline {
  agent none
  options {
    buildDiscarder(logRotator(daysToKeepStr: '30'))
    timeout(time: 3, unit: 'HOURS')
  }
  parameters {
        booleanParam(name: 'janky', defaultValue: true, description: 'x86 Build/Test')
        booleanParam(name: 'experimental', defaultValue: true, description: 'x86 Experimental Build/Test ')
        booleanParam(name: 'z', defaultValue: true, description: 'IBM Z (s390x) Build/Test')
        booleanParam(name: 'powerpc', defaultValue: true, description: 'PowerPC (ppc64le) Build/Test')
        booleanParam(name: 'vendor', defaultValue: true, description: 'Vendor')
        booleanParam(name: 'windowsRS1', defaultValue: true, description: 'Windows 2016 (RS1) Build/Test')
        booleanParam(name: 'windowsRS5', defaultValue: true, description: 'Windows 2019 (RS5) Build/Test')
  }
  stages {
    stage('Build') {
      parallel {
        stage('janky') {
          when {
            beforeAgent true
            expression { params.janky }
          }
          agent {
            node {
              label 'ubuntu-1604-overlay2-stable'
            }
          }
          steps {
            withCredentials([string(credentialsId: '52af932f-f13f-429e-8467-e7ff8b965cdb', variable: 'CODECOV_TOKEN')]) {
              withGithubStatus('janky') {
                sh '''
                  # todo: include ip_vs in base image
                  sudo modprobe ip_vs

                  GITCOMMIT=$(git rev-parse --short HEAD)
                  docker build --rm --force-rm --build-arg APT_MIRROR=cdn-fastly.deb.debian.org -t docker:$GITCOMMIT .

                  docker run --rm -t --privileged \
                    -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                    -v "$WORKSPACE/.git:/go/src/github.com/docker/docker/.git" \
                    --name docker-pr$BUILD_NUMBER \
                    -e DOCKER_GITCOMMIT=${GITCOMMIT} \
                    -e DOCKER_GRAPHDRIVER=vfs \
                    -e DOCKER_EXECDRIVER=native \
                    -e CODECOV_TOKEN \
                    -e GIT_SHA1=${GIT_COMMIT} \
                    docker:$GITCOMMIT \
                    hack/ci/janky
                '''
                sh '''
                  GITCOMMIT=$(git rev-parse --short HEAD)
                  echo "Building e2e image"
                  docker build --build-arg DOCKER_GITCOMMIT=$GITCOMMIT -t moby-e2e-test -f Dockerfile.e2e .
                '''
              }
            }
          }
          post {
            always {
              sh '''
                echo "Ensuring container killed."
                docker rm -vf docker-pr$BUILD_NUMBER || true

                echo "Chowning /workspace to jenkins user"
                docker run --rm -v "$WORKSPACE:/workspace" busybox chown -R "$(id -u):$(id -g)" /workspace
              '''
              sh '''
                echo "Creating bundles.tar.gz"
                (find bundles -name '*.log' -o -name '*.prof' -o -name integration.test | xargs tar -czf bundles.tar.gz) || true
              '''
              archiveArtifacts artifacts: 'bundles.tar.gz'
            }
          }
        }
        stage('experimental') {
          when {
            beforeAgent true
            expression { params.experimental }
          }
          agent {
            node {
              label 'ubuntu-1604-aufs-stable'
            }
          }
          steps {
            withGithubStatus('experimental') {
              sh '''
                GITCOMMIT=$(git rev-parse --short HEAD)
                docker build --rm --force-rm --build-arg APT_MIRROR=cdn-fastly.deb.debian.org -t docker:${GITCOMMIT}-exp .

                docker run --rm -t --privileged \
                    -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                    -e DOCKER_EXPERIMENTAL=y \
                    --name docker-pr-exp$BUILD_NUMBER \
                    -e DOCKER_GITCOMMIT=${GITCOMMIT} \
                    -e DOCKER_GRAPHDRIVER=vfs \
                    -e DOCKER_EXECDRIVER=native \
                    docker:${GITCOMMIT}-exp \
                    hack/ci/experimental
              '''
            }
          }
          post {
            always {
              sh '''
                echo "Ensuring container killed."
                docker rm -vf docker-pr-exp$BUILD_NUMBER || true

                echo "Chowning /workspace to jenkins user"
                docker run --rm -v "$WORKSPACE:/workspace" busybox chown -R "$(id -u):$(id -g)" /workspace
              '''
              sh '''
                echo "Creating bundles.tar.gz"
                (find bundles -name '*.log' -o -name '*.prof' -o -name integration.test | xargs tar -czf bundles.tar.gz) || true
              '''
              archiveArtifacts artifacts: 'bundles.tar.gz'
            }
          }
        }
        stage('z') {
          when {
            beforeAgent true
            expression { params.z }
          }
          agent {
            node {
              label 's390x-ubuntu-1604'
            }
          }
          steps {
            withGithubStatus('z') {
              sh '''
                GITCOMMIT=$(git rev-parse --short HEAD)

                test -f Dockerfile.s390x && \
                docker build --rm --force-rm --build-arg APT_MIRROR=cdn-fastly.deb.debian.org -t docker-s390x:$GITCOMMIT -f Dockerfile.s390x . || \
                docker build --rm --force-rm --build-arg APT_MIRROR=cdn-fastly.deb.debian.org -t docker-s390x:$GITCOMMIT -f Dockerfile .

                docker run --rm -t --privileged \
                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                    --name docker-pr-s390x$BUILD_NUMBER \
                    -e DOCKER_GRAPHDRIVER=vfs \
                    -e DOCKER_EXECDRIVER=native \
                    -e TIMEOUT="300m" \
                    -e DOCKER_GITCOMMIT=${GITCOMMIT} \
                    docker-s390x:$GITCOMMIT \
                    hack/ci/z
              '''
            }
          }
          post {
            always {
              sh '''
                echo "Ensuring container killed."
                docker rm -vf docker-pr-s390x$BUILD_NUMBER || true

                echo "Chowning /workspace to jenkins user"
                docker run --rm -v "$WORKSPACE:/workspace" s390x/busybox chown -R "$(id -u):$(id -g)" /workspace
              '''
              sh '''
                echo "Creating bundles.tar.gz"
                find bundles -name '*.log' | xargs tar -czf bundles.tar.gz
              '''
              archiveArtifacts artifacts: 'bundles.tar.gz'
            }
          }
        }
        stage('powerpc') {
          when {
            beforeAgent true
            expression { params.powerpc }
          }
          agent {
            node {
              label 'ppc64le-ubuntu-1604'
            }
          }
          steps {
            withGithubStatus('powerpc') {
              sh '''
                GITCOMMIT=$(git rev-parse --short HEAD)

                test -f Dockerfile.ppc64le && \
                docker build --rm --force-rm --build-arg APT_MIRROR=cdn-fastly.deb.debian.org -t docker-powerpc:$GITCOMMIT -f Dockerfile.ppc64le . || \
                docker build --rm --force-rm --build-arg APT_MIRROR=cdn-fastly.deb.debian.org -t docker-powerpc:$GITCOMMIT -f Dockerfile .

                docker run --rm -t --privileged \
                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                    --name docker-pr-power$BUILD_NUMBER \
                    -e DOCKER_GRAPHDRIVER=vfs \
                    -e DOCKER_EXECDRIVER=native \
                    -e DOCKER_GITCOMMIT=${GITCOMMIT} \
                    -e TIMEOUT="180m" \
                    docker-powerpc:$GITCOMMIT \
                    hack/ci/powerpc
              '''
            }
          }
          post {
            always {
              sh '''
                echo "Ensuring container killed."
                docker rm -vf docker-pr-power$BUILD_NUMBER || true

                echo "Chowning /workspace to jenkins user"
                docker run --rm -v "$WORKSPACE:/workspace" ppc64le/busybox chown -R "$(id -u):$(id -g)" /workspace
              '''
              sh '''
                echo "Creating bundles.tar.gz"
                find bundles -name '*.log' | xargs tar -czf bundles.tar.gz
              '''
              archiveArtifacts artifacts: 'bundles.tar.gz'
            }
          }
        }
        stage('vendor') {
          when {
            beforeAgent true
            expression { params.vendor }
          }
          agent {
            node {
              label 'ubuntu-1604-aufs-stable'
            }
          }
          steps {
            withGithubStatus('vendor') {
              sh '''
                GITCOMMIT=$(git rev-parse --short HEAD)

                docker build --rm --force-rm --build-arg APT_MIRROR=cdn-fastly.deb.debian.org -t dockerven:$GITCOMMIT .

                docker run --rm -t --privileged \
                  --name dockerven-pr$BUILD_NUMBER \
                  -e DOCKER_GRAPHDRIVER=vfs \
                  -e DOCKER_EXECDRIVER=native \
                  -v "$WORKSPACE/.git:/go/src/github.com/docker/docker/.git" \
                  -e DOCKER_GITCOMMIT=${GITCOMMIT} \
                  -e TIMEOUT=120m dockerven:$GITCOMMIT \
                  hack/validate/vendor
              '''
            }
          }
        }
        stage('windowsRS1') {
          when {
            beforeAgent true
            expression { params.windowsRS1 }
          }
          agent {
            node {
              label 'windows-rs1'
              customWorkspace 'c:\\gopath\\src\\github.com\\docker\\docker'
            }
          }
          steps {
            withGithubStatus('windowsRS1') {
              powershell '''
                $ErrorActionPreference = 'Stop'
                .\\hack\\ci\\windows.ps1
                exit $LastExitCode
              '''
            }
          }
        }
        stage('windowsRS5-process') {
          when {
            beforeAgent true
            expression { params.windowsRS5 }
          }
          agent {
            node {
              label 'windows-rs5'
              customWorkspace 'c:\\gopath\\src\\github.com\\docker\\docker'
            }
          }
          steps {
            withGithubStatus('windowsRS5-process') {
              powershell '''
                $ErrorActionPreference = 'Stop'
                .\\hack\\ci\\windows.ps1
                exit $LastExitCode
              '''
            }
          }
        }
      }
    }
  }
}