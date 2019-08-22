#!groovy
pipeline {
    agent none

    options {
        buildDiscarder(logRotator(daysToKeepStr: '30'))
        timeout(time: 2, unit: 'HOURS')
        timestamps()
    }
    parameters {
        booleanParam(name: 'unit_validate', defaultValue: true, description: 'x86 unit tests and vendor check')
        booleanParam(name: 'janky', defaultValue: true, description: 'x86 Build/Test')
        booleanParam(name: 'z', defaultValue: true, description: 'IBM Z (s390x) Build/Test')
        booleanParam(name: 'powerpc', defaultValue: true, description: 'PowerPC (ppc64le) Build/Test')
        booleanParam(name: 'windowsRS1', defaultValue: false, description: 'Windows 2016 (RS1) Build/Test')
        booleanParam(name: 'windowsRS5', defaultValue: false, description: 'Windows 2019 (RS5) Build/Test')
    }
    environment {
        DOCKER_BUILDKIT     = '1'
        DOCKER_EXPERIMENTAL = '1'
        DOCKER_GRAPHDRIVER  = 'overlay2'
        APT_MIRROR          = 'cdn-fastly.deb.debian.org'
        CHECK_CONFIG_COMMIT = '78405559cfe5987174aa2cb6463b9b2c1b917255'
        TIMEOUT             = '120m'
    }
    stages {
        stage('Build') {
            parallel {
                stage('unit-validate') {
                    when {
                        beforeAgent true
                        expression { params.unit_validate }
                    }
                    agent { label 'amd64 && ubuntu-1804 && overlay2' }

                    stages {
                        stage("Print info") {
                            steps {
                                sh 'docker version'
                                sh 'docker info'
                                sh '''
                                echo "check-config.sh version: ${CHECK_CONFIG_COMMIT}"
                                curl -fsSL -o ${WORKSPACE}/check-config.sh "https://raw.githubusercontent.com/moby/moby/${CHECK_CONFIG_COMMIT}/contrib/check-config.sh" \
                                && bash ${WORKSPACE}/check-config.sh || true
                                '''
                            }
                        }
                        stage("Build dev image") {
                            steps {
                                sh 'docker build --force-rm --build-arg APT_MIRROR -t docker:${GIT_COMMIT} .'
                            }
                        }
                        stage("Validate") {
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  -v "$WORKSPACE/.git:/go/src/github.com/docker/docker/.git" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  docker:${GIT_COMMIT} \
                                  hack/validate/default
                                '''
                            }
                        }
                        stage("Docker-py") {
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh \
                                    dynbinary-daemon \
                                    test-docker-py
                                '''
                            }
                            post {
                                always {
                                    junit testResults: 'bundles/test-docker-py/junit-report.xml', allowEmptyResults: true

                                    sh '''
                                    echo "Ensuring container killed."
                                    docker rm -vf docker-pr$BUILD_NUMBER || true
                                    '''

                                    sh '''
                                    echo 'Chowning /workspace to jenkins user'
                                    docker run --rm -v "$WORKSPACE:/workspace" busybox chown -R "$(id -u):$(id -g)" /workspace
                                    '''

                                    sh '''
                                    echo 'Creating docker-py-bundles.tar.gz'
                                    tar -czf docker-py-bundles.tar.gz bundles/test-docker-py/*.xml bundles/test-docker-py/*.log
                                    '''

                                    archiveArtifacts artifacts: 'docker-py-bundles.tar.gz'
                                }
                            }
                        }
                        stage("Static") {
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh binary-daemon
                                '''
                            }
                        }
                        stage("Cross") {
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh cross
                                '''
                            }
                        }
                        // needs to be last stage that calls make.sh for the junit report to work
                        stage("Unit tests") {
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  docker:${GIT_COMMIT} \
                                  hack/test/unit
                                '''
                            }
                            post {
                                always {
                                    junit testResults: 'bundles/junit-report.xml', allowEmptyResults: true
                                }
                            }
                        }
                        stage("Validate vendor") {
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/.git:/go/src/github.com/docker/docker/.git" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  docker:${GIT_COMMIT} \
                                  hack/validate/vendor
                                '''
                            }
                        }
                        stage("Build e2e image") {
                            steps {
                                sh '''
                                echo "Building e2e image"
                                docker build --build-arg DOCKER_GITCOMMIT=${GIT_COMMIT} -t moby-e2e-test -f Dockerfile.e2e .
                                '''
                            }
                        }
                    }

                    post {
                        always {
                            sh '''
                            echo 'Ensuring container killed.'
                            docker rm -vf docker-pr$BUILD_NUMBER || true
                            '''

                            sh '''
                            echo 'Chowning /workspace to jenkins user'
                            docker run --rm -v "$WORKSPACE:/workspace" busybox chown -R "$(id -u):$(id -g)" /workspace
                            '''

                            sh '''
                            echo 'Creating unit-bundles.tar.gz'
                            tar -czvf unit-bundles.tar.gz bundles/junit-report.xml bundles/go-test-report.json bundles/profile.out
                            '''

                            archiveArtifacts artifacts: 'unit-bundles.tar.gz'
                        }
                        cleanup {
                            sh 'make clean'
                            deleteDir()
                        }
                    }
                }
                stage('janky') {
                    when {
                        beforeAgent true
                        expression { params.janky }
                    }
                    agent { label 'amd64 && ubuntu-1804 && overlay2' }

                    stages {
                        stage("Print info") {
                            steps {
                                sh 'docker version'
                                sh 'docker info'
                                sh '''
                                echo "check-config.sh version: ${CHECK_CONFIG_COMMIT}"
                                curl -fsSL -o ${WORKSPACE}/check-config.sh "https://raw.githubusercontent.com/moby/moby/${CHECK_CONFIG_COMMIT}/contrib/check-config.sh" \
                                && bash ${WORKSPACE}/check-config.sh || true
                                '''
                            }
                        }
                        stage("Build dev image") {
                            steps {
                                sh '''
                                # todo: include ip_vs in base image
                                sudo modprobe ip_vs
                
                                docker build --force-rm --build-arg APT_MIRROR -t docker:${GIT_COMMIT} .
                                '''
                            }
                        }
                        stage("Run tests") {
                            steps {
                                sh '''#!/bin/bash
                                # bash is needed so 'jobs -p' works properly
                                # it also accepts setting inline envvars for functions without explicitly exporting
 
                                run_tests() {
                                        [ -n "$TESTDEBUG" ] && rm= || rm=--rm;
                                        docker run $rm -t --privileged \
                                          -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                          -v "$WORKSPACE/.git:/go/src/github.com/docker/docker/.git" \
                                          --name "$CONTAINER_NAME" \
                                          -e KEEPBUNDLE=1 \
                                          -e TESTDEBUG \
                                          -e TESTFLAGS \
                                          -e TEST_INTEGRATION_DEST \
                                          -e TEST_SKIP_INTEGRATION \
                                          -e TEST_SKIP_INTEGRATION_CLI \
                                          -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                          -e DOCKER_GRAPHDRIVER \
                                          -e TIMEOUT \
                                          docker:${GIT_COMMIT} \
                                          hack/make.sh \
                                            "$1" \
                                            test-integration
                                }

                                trap "exit" INT TERM
                                trap 'pids=$(jobs -p); echo "Remaining pids to kill: [$pids]"; [ -z "$pids" ] || kill $pids' EXIT

                                CONTAINER_NAME=docker-pr$BUILD_NUMBER

                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  -v "$WORKSPACE/.git:/go/src/github.com/docker/docker/.git" \
                                  --name ${CONTAINER_NAME}-build \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh \
                                    dynbinary-daemon

                                # flaky + integration
                                TEST_INTEGRATION_DEST=1 CONTAINER_NAME=${CONTAINER_NAME}-1 TEST_SKIP_INTEGRATION_CLI=1 run_tests test-integration-flaky &

                                # integration-cli first set
                                TEST_INTEGRATION_DEST=2 CONTAINER_NAME=${CONTAINER_NAME}-2 TEST_SKIP_INTEGRATION=1 TESTFLAGS="-check.f ^(DockerSuite|DockerNetworkSuite|DockerHubPullSuite|DockerRegistrySuite|DockerSchema1RegistrySuite|DockerRegistryAuthTokenSuite|DockerRegistryAuthHtpasswdSuite)" run_tests &

                                # integration-cli second set
                                TEST_INTEGRATION_DEST=3 CONTAINER_NAME=${CONTAINER_NAME}-3 TEST_SKIP_INTEGRATION=1 TESTFLAGS="-check.f ^(DockerSwarmSuite|DockerDaemonSuite|DockerExternalVolumeSuite)" run_tests &

                                set +x
                                c=0
                                for job in $(jobs -p); do
                                        wait ${job} || c=$?
                                done
                                exit $c
                                '''
                            }
                        }
                    }

                    post {
                        always {
                            sh '''
                            echo "Ensuring container killed."
                            docker rm -vf docker-pr$BUILD_NUMBER || true
                            '''

                            sh '''
                            echo "Chowning /workspace to jenkins user"
                            docker run --rm -v "$WORKSPACE:/workspace" busybox chown -R "$(id -u):$(id -g)" /workspace
                            '''

                            sh '''
                            echo "Creating janky-bundles.tar.gz"
                            # exclude overlay2 directories
                            find bundles -path '*/root/*overlay2' -prune -o -type f \\( -name '*.log' -o -name '*.prof' \\) -print | xargs tar -czf janky-bundles.tar.gz
                            '''

                            archiveArtifacts artifacts: 'janky-bundles.tar.gz'
                        }
                        cleanup {
                            sh 'make clean'
                            deleteDir()
                        }
                    }
                }
                stage('z') {
                    when {
                        beforeAgent true
                        expression { params.z }
                    }
                    agent { label 's390x-ubuntu-1604' }
                    // s390x machines run on Docker 18.06, and buildkit has some bugs on that version
                    environment { DOCKER_BUILDKIT = '0' }

                    stages {
                        stage("Print info") {
                            steps {
                                sh 'docker version'
                                sh 'docker info'
                                sh '''
                                echo "check-config.sh version: ${CHECK_CONFIG_COMMIT}"
                                curl -fsSL -o ${WORKSPACE}/check-config.sh "https://raw.githubusercontent.com/moby/moby/${CHECK_CONFIG_COMMIT}/contrib/check-config.sh" \
                                && bash ${WORKSPACE}/check-config.sh || true
                                '''
                            }
                        }
                        stage("Build dev image") {
                            steps {
                                sh '''
                                docker build --force-rm --build-arg APT_MIRROR -t docker:${GIT_COMMIT} -f Dockerfile .
                                '''
                            }
                        }
                        stage("Unit tests") {
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  docker:${GIT_COMMIT} \
                                  hack/test/unit
                                '''
                            }
                            post {
                                always {
                                    junit testResults: 'bundles/junit-report.xml', allowEmptyResults: true
                                }
                            }
                        }
                        stage("Integration tests") {
                            environment { TEST_SKIP_INTEGRATION_CLI = '1' }
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  -e TEST_SKIP_INTEGRATION_CLI \
                                  -e TIMEOUT \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh \
                                    dynbinary \
                                    test-integration
                                '''
                            }
                        }
                    }

                    post {
                        always {
                            sh '''
                            echo "Ensuring container killed."
                            docker rm -vf docker-pr$BUILD_NUMBER || true
                            '''

                            sh '''
                            echo "Chowning /workspace to jenkins user"
                            docker run --rm -v "$WORKSPACE:/workspace" busybox chown -R "$(id -u):$(id -g)" /workspace
                            '''

                            sh '''
                            echo "Creating s390x-integration-bundles.tar.gz"
                            # exclude overlay2 directories
                            find bundles -path '*/root/*overlay2' -prune -o -type f \\( -name '*.log' -o -name '*.prof' \\) -print | xargs tar -czf s390x-integration-bundles.tar.gz
                            '''

                            archiveArtifacts artifacts: 's390x-integration-bundles.tar.gz'
                        }
                        cleanup {
                            sh 'make clean'
                            deleteDir()
                        }
                    }
                }
                stage('z-master') {
                    when {
                        beforeAgent true
                        branch 'master'
                        expression { params.z }
                    }
                    agent { label 's390x-ubuntu-1604' }
                    // s390x machines run on Docker 18.06, and buildkit has some bugs on that version
                    environment { DOCKER_BUILDKIT = '0' }

                    stages {
                        stage("Print info") {
                            steps {
                                sh 'docker version'
                                sh 'docker info'
                                sh '''
                                echo "check-config.sh version: ${CHECK_CONFIG_COMMIT}"
                                curl -fsSL -o ${WORKSPACE}/check-config.sh "https://raw.githubusercontent.com/moby/moby/${CHECK_CONFIG_COMMIT}/contrib/check-config.sh" \
                                && bash ${WORKSPACE}/check-config.sh || true
                                '''
                            }
                        }
                        stage("Build dev image") {
                            steps {
                                sh '''
                                docker build --force-rm --build-arg APT_MIRROR -t docker:${GIT_COMMIT} -f Dockerfile .
                                '''
                            }
                        }
                        stage("Integration-cli tests") {
                            environment { TEST_SKIP_INTEGRATION = '1' }
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  -e TEST_SKIP_INTEGRATION \
                                  -e TIMEOUT \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh \
                                    dynbinary \
                                    test-integration
                                '''
                            }
                        }
                    }

                    post {
                        always {
                            sh '''
                            echo "Ensuring container killed."
                            docker rm -vf docker-pr$BUILD_NUMBER || true
                            '''

                            sh '''
                            echo "Chowning /workspace to jenkins user"
                            docker run --rm -v "$WORKSPACE:/workspace" busybox chown -R "$(id -u):$(id -g)" /workspace
                            '''

                            sh '''
                            echo "Creating s390x-integration-cli-bundles.tar.gz"
                            find bundles -path '*/root/*overlay2' -prune -o -type f \\( -name '*.log' -o -name '*.prof' \\) -print | xargs tar -czf s390x-integration-cli-bundles.tar.gz
                            '''

                            archiveArtifacts artifacts: 's390x-integration-cli-bundles.tar.gz'
                        }
                        cleanup {
                            sh 'make clean'
                            deleteDir()
                        }
                    }
                }
                stage('powerpc') {
                    when {
                        beforeAgent true
                        expression { params.powerpc }
                    }
                    agent { label 'ppc64le-ubuntu-1604' }
                    // power machines run on Docker 18.06, and buildkit has some bugs on that version
                    environment { DOCKER_BUILDKIT = '0' }

                    stages {
                        stage("Print info") {
                            steps {
                                sh 'docker version'
                                sh 'docker info'
                                sh '''
                                echo "check-config.sh version: ${CHECK_CONFIG_COMMIT}"
                                curl -fsSL -o ${WORKSPACE}/check-config.sh "https://raw.githubusercontent.com/moby/moby/${CHECK_CONFIG_COMMIT}/contrib/check-config.sh" \
                                && bash ${WORKSPACE}/check-config.sh || true
                                '''
                            }
                        }
                        stage("Build dev image") {
                            steps {
                                sh 'docker build --force-rm --build-arg APT_MIRROR -t docker:${GIT_COMMIT} -f Dockerfile .'
                            }
                        }
                        stage("Unit tests") {
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  docker:${GIT_COMMIT} \
                                  hack/test/unit
                                '''
                            }
                            post {
                                always {
                                    junit testResults: 'bundles/junit-report.xml', allowEmptyResults: true
                                }
                            }
                        }
                        stage("Integration tests") {
                            environment { TEST_SKIP_INTEGRATION_CLI = '1' }
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  -e TEST_SKIP_INTEGRATION_CLI \
                                  -e TIMEOUT \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh \
                                    dynbinary \
                                    test-integration
                                '''
                            }
                        }
                    }

                    post {
                        always {
                            sh '''
                            echo "Ensuring container killed."
                            docker rm -vf docker-pr$BUILD_NUMBER || true
                            '''

                            sh '''
                            echo "Chowning /workspace to jenkins user"
                            docker run --rm -v "$WORKSPACE:/workspace" busybox chown -R "$(id -u):$(id -g)" /workspace
                            '''

                            sh '''
                            echo "Creating powerpc-integration-bundles.tar.gz"
                            # exclude overlay2 directories
                            find bundles -path '*/root/*overlay2' -prune -o -type f \\( -name '*.log' -o -name '*.prof' \\) -print | xargs tar -czf powerpc-integration-bundles.tar.gz
                            '''

                            archiveArtifacts artifacts: 'powerpc-integration-bundles.tar.gz'
                        }
                        cleanup {
                            sh 'make clean'
                            deleteDir()
                        }
                    }
                }
                stage('powerpc-master') {
                    when {
                        beforeAgent true
                        branch 'master'
                        expression { params.powerpc }
                    }
                    agent { label 'ppc64le-ubuntu-1604' }
                    // power machines run on Docker 18.06, and buildkit has some bugs on that version
                    environment { DOCKER_BUILDKIT = '0' }

                    stages {
                        stage("Print info") {
                            steps {
                                sh 'docker version'
                                sh 'docker info'
                                sh '''
                                echo "check-config.sh version: ${CHECK_CONFIG_COMMIT}"
                                curl -fsSL -o ${WORKSPACE}/check-config.sh "https://raw.githubusercontent.com/moby/moby/${CHECK_CONFIG_COMMIT}/contrib/check-config.sh" \
                                && bash ${WORKSPACE}/check-config.sh || true
                                '''
                            }
                        }
                        stage("Build dev image") {
                            steps {
                                sh 'docker build --force-rm --build-arg APT_MIRROR -t docker:${GIT_COMMIT} -f Dockerfile .'
                            }
                        }
                        stage("Integration-cli tests") {
                            environment { TEST_SKIP_INTEGRATION = '1' }
                            steps {
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  -e TEST_SKIP_INTEGRATION \
                                  -e TIMEOUT \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh \
                                    dynbinary \
                                    test-integration
                                '''
                            }
                        }
                    }

                    post {
                        always {
                            sh '''
                            echo "Ensuring container killed."
                            docker rm -vf docker-pr$BUILD_NUMBER || true
                            '''

                            sh '''
                            echo "Chowning /workspace to jenkins user"
                            docker run --rm -v "$WORKSPACE:/workspace" busybox chown -R "$(id -u):$(id -g)" /workspace
                            '''

                            sh '''
                            echo "Creating powerpc-integration-cli-bundles.tar.gz"
                            find bundles -path '*/root/*overlay2' -prune -o -type f \\( -name '*.log' -o -name '*.prof' \\) -print | xargs tar -czf powerpc-integration-cli-bundles.tar.gz
                            '''

                            archiveArtifacts artifacts: 'powerpc-integration-cli-bundles.tar.gz'
                        }
                        cleanup {
                            sh 'make clean'
                            deleteDir()
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
                    stages {
                        stage("Print info") {
                            steps {
                                sh 'docker version'
                                sh 'docker info'
                            }
                        }
                        stage("Run tests") {
                            steps {
                                powershell '''
                                $ErrorActionPreference = 'Stop'
                                .\\hack\\ci\\windows.ps1
                                exit $LastExitCode
                                '''
                            }
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
                    stages {
                        stage("Print info") {
                            steps {
                                sh 'docker version'
                                sh 'docker info'
                            }
                        }
                        stage("Run tests") {
                            steps {
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
}
