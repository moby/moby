#!groovy
pipeline {
    agent none

    options {
        buildDiscarder(logRotator(daysToKeepStr: '30'))
        timeout(time: 2, unit: 'HOURS')
        timestamps()
    }
    parameters {
        booleanParam(name: 'unit_validate', defaultValue: true, description: 'amd64 (x86_64) unit tests and vendor check')
        booleanParam(name: 'validate_force', defaultValue: false, description: 'force validation steps to be run, even if no changes were detected')
        booleanParam(name: 'amd64', defaultValue: true, description: 'amd64 (x86_64) Build/Test')
        booleanParam(name: 'rootless', defaultValue: true, description: 'amd64 (x86_64) Build/Test (Rootless mode)')
        booleanParam(name: 'cgroup2', defaultValue: true, description: 'amd64 (x86_64) Build/Test (cgroup v2)')
        booleanParam(name: 'arm64', defaultValue: true, description: 'ARM (arm64) Build/Test')
        booleanParam(name: 's390x', defaultValue: false, description: 'IBM Z (s390x) Build/Test')
        booleanParam(name: 'ppc64le', defaultValue: false, description: 'PowerPC (ppc64le) Build/Test')
        booleanParam(name: 'windowsRS5', defaultValue: true, description: 'Windows 2019 (RS5) Build/Test')
        booleanParam(name: 'windows2022', defaultValue: true, description: 'Windows 2022 (LTSC) Build/Test')
        booleanParam(name: 'windows2022containerd', defaultValue: true, description: 'Windows 2022 (LTSC) with containerd Build/Test')
        booleanParam(name: 'dco', defaultValue: true, description: 'Run the DCO check')
    }
    environment {
        DOCKER_BUILDKIT     = '1'
        DOCKER_EXPERIMENTAL = '1'
        DOCKER_GRAPHDRIVER  = 'overlay2'
        APT_MIRROR          = 'cdn-fastly.deb.debian.org'
        CHECK_CONFIG_COMMIT = '33a3680e08d1007e72c3b3f1454f823d8e9948ee'
        TESTDEBUG           = '0'
        TIMEOUT             = '120m'
    }
    stages {
        stage('pr-hack') {
            when { changeRequest() }
            steps {
                script {
                    echo "Workaround for PR auto-cancel feature. Borrowed from https://issues.jenkins-ci.org/browse/JENKINS-43353"
                    def buildNumber = env.BUILD_NUMBER as int
                    if (buildNumber > 1) milestone(buildNumber - 1)
                    milestone(buildNumber)
                }
            }
        }
        stage('Build') {
            parallel {
                stage('amd64-unit-validate') {
                    agent { label 'amd64 && ubuntu-2004' }
                    environment {
                        TEST_SKIP_INTEGRATION_CLI = '1'
                    }

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
                                sh 'docker build --force-rm --build-arg APT_MIRROR --build-arg CROSS=true -t docker:${GIT_COMMIT} .'
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
                                  -e TEST_FORCE_VALIDATE \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
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
                                  -e TEST_SKIP_INTEGRATION_CLI \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh \
                                    dynbinary \
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

                                    catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE', message: 'Failed to create bundles.tar.gz') {
                                        sh '''
                                        bundleName=amd64-docker-py
                                        echo "Creating ${bundleName}-bundles.tar.gz"
                                        tar -czf ${bundleName}-bundles.tar.gz bundles/test-docker-py/*.xml bundles/test-docker-py/*.log
                                        '''

                                        archiveArtifacts artifacts: '*-bundles.tar.gz', allowEmptyArchive: true
                                    }
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
                                  hack/make.sh binary
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
                                sudo modprobe ip6table_filter
                                '''
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
                                  docker:${GIT_COMMIT} \
                                  hack/test/unit
                                '''
                            }
                            post {
                                always {
                                    junit testResults: 'bundles/junit-report*.xml', allowEmptyResults: true
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
                                  -e TEST_FORCE_VALIDATE \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
                                  docker:${GIT_COMMIT} \
                                  hack/validate/vendor
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

                            catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE', message: 'Failed to create bundles.tar.gz') {
                                sh '''
                                bundleName=amd64-unit
                                echo "Creating ${bundleName}-bundles.tar.gz"
                                tar -czvf ${bundleName}-bundles.tar.gz bundles/junit-report*.xml bundles/go-test-report*.json bundles/profile*.out
                                '''

                                archiveArtifacts artifacts: '*-bundles.tar.gz', allowEmptyArchive: true
                            }
                        }
                        cleanup {
                            sh 'make clean'
                            deleteDir()
                        }
                    }
                }
                stage('arm64-unit-validate') {
                    agent { label 'arm64 && ubuntu-2004' }
                    environment {
                        TEST_SKIP_INTEGRATION_CLI = '1'
                    }

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
                                sh 'docker build --force-rm --build-arg APT_MIRROR --build-arg CROSS=true -t docker:${GIT_COMMIT} .'
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
                                  -e TEST_FORCE_VALIDATE \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
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
                                  -e TEST_SKIP_INTEGRATION_CLI \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh \
                                    dynbinary \
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

                                    catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE', message: 'Failed to create bundles.tar.gz') {
                                        sh '''
                                        bundleName=arm64-docker-py
                                        echo "Creating ${bundleName}-bundles.tar.gz"
                                        tar -czf ${bundleName}-bundles.tar.gz bundles/test-docker-py/*.xml bundles/test-docker-py/*.log
                                        '''

                                        archiveArtifacts artifacts: '*-bundles.tar.gz', allowEmptyArchive: true
                                    }
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
                                  hack/make.sh binary
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
                                sudo modprobe ip6table_filter
                                '''
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
                                  docker:${GIT_COMMIT} \
                                  hack/test/unit
                                '''
                            }
                            post {
                                always {
                                    junit testResults: 'bundles/junit-report*.xml', allowEmptyResults: true
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
                                  -e TEST_FORCE_VALIDATE \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
                                  docker:${GIT_COMMIT} \
                                  hack/validate/vendor
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

                            catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE', message: 'Failed to create bundles.tar.gz') {
                                sh '''
                                bundleName=arm64-unit
                                echo "Creating ${bundleName}-bundles.tar.gz"
                                tar -czvf ${bundleName}-bundles.tar.gz bundles/junit-report*.xml bundles/go-test-report*.json bundles/profile*.out
                                '''

                                archiveArtifacts artifacts: '*-bundles.tar.gz', allowEmptyArchive: true
                            }
                        }
                        cleanup {
                            sh 'make clean'
                            deleteDir()
                        }
                    }
                }
                stage('arm64') {
                    agent { label 'arm64 && ubuntu-2004' }
                    environment {
                        TEST_SKIP_INTEGRATION_CLI = '1'
                    }

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
                        stage("Unit tests") {
                            steps {
                                sh '''
                                sudo modprobe ip6table_filter
                                '''
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
                                  docker:${GIT_COMMIT} \
                                  hack/test/unit
                                '''
                            }
                            post {
                                always {
                                    junit testResults: 'bundles/junit-report*.xml', allowEmptyResults: true
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
                                  -e TESTDEBUG \
                                  -e TEST_SKIP_INTEGRATION_CLI \
                                  -e TIMEOUT \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh \
                                    dynbinary \
                                    test-integration
                                '''
                            }
                            post {
                                always {
                                    junit testResults: 'bundles/**/*-report.xml', allowEmptyResults: true
                                }
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

                            catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE', message: 'Failed to create bundles.tar.gz') {
                                sh '''
                                bundleName=arm64-integration
                                echo "Creating ${bundleName}-bundles.tar.gz"
                                # exclude overlay2 directories
                                find bundles -path '*/root/*overlay2' -prune -o -type f \\( -name '*-report.json' -o -name '*.log' -o -name '*.prof' -o -name '*-report.xml' \\) -print | xargs tar -czf ${bundleName}-bundles.tar.gz
                                '''

                                archiveArtifacts artifacts: '*-bundles.tar.gz', allowEmptyArchive: true
                            }
                        }
                        cleanup {
                            sh 'make clean'
                            deleteDir()
                        }
                    }
                }
                stage('amd64') {
                    agent { label 'amd64 && ubuntu-2004' }
                    environment {
                        TEST_SKIP_INTEGRATION_CLI = '1'
                    }

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
                        stage("Unit tests") {
                            steps {
                                sh '''
                                sudo modprobe ip6table_filter
                                '''
                                sh '''
                                docker run --rm -t --privileged \
                                  -v "$WORKSPACE/bundles:/go/src/github.com/docker/docker/bundles" \
                                  --name docker-pr$BUILD_NUMBER \
                                  -e DOCKER_EXPERIMENTAL \
                                  -e DOCKER_GITCOMMIT=${GIT_COMMIT} \
                                  -e DOCKER_GRAPHDRIVER \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
                                  docker:${GIT_COMMIT} \
                                  hack/test/unit
                                '''
                            }
                            post {
                                always {
                                    junit testResults: 'bundles/junit-report*.xml', allowEmptyResults: true
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
                                  -e TESTDEBUG \
                                  -e TEST_SKIP_INTEGRATION_CLI \
                                  -e TIMEOUT \
                                  -e VALIDATE_REPO=${GIT_URL} \
                                  -e VALIDATE_BRANCH=${CHANGE_TARGET} \
                                  docker:${GIT_COMMIT} \
                                  hack/make.sh \
                                    dynbinary \
                                    test-integration
                                '''
                            }
                            post {
                                always {
                                    junit testResults: 'bundles/**/*-report.xml', allowEmptyResults: true
                                }
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

                            catchError(buildResult: 'SUCCESS', stageResult: 'FAILURE', message: 'Failed to create bundles.tar.gz') {
                                sh '''
                                bundleName=amd64-integration
                                echo "Creating ${bundleName}-bundles.tar.gz"
                                # exclude overlay2 directories
                                find bundles -path '*/root/*overlay2' -prune -o -type f \\( -name '*-report.json' -o -name '*.log' -o -name '*.prof' -o -name '*-report.xml' \\) -print | xargs tar -czf ${bundleName}-bundles.tar.gz
                                '''

                                archiveArtifacts artifacts: '*-bundles.tar.gz', allowEmptyArchive: true
                            }
                        }
                        cleanup {
                            sh 'make clean'
                            deleteDir()
                        }
                    }
                }
            }
        }
    }
}
