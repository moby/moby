wrappedNode(label: "linux && x86_64") {
  deleteDir()
  checkout scm

  stage "build image"
  def img = docker.build("dockerbuildbot/containerd:${gitCommit()}")
  try {
    stage "run tests"
    sh "docker run --privileged --rm --name '${env.BUILD_TAG}' ${img.id} make test"
  } finally {
    sh "docker rmi -f ${img.id} ||:"
  }
}
