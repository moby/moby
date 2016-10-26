// Only run on Linux atm
wrappedNode(label: 'docker') {
  deleteDir()
  stage "checkout"
  checkout scm

  documentationChecker("docs")
}
