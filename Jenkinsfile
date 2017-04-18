

def runTask(String label, String task) {
  { ->
    wrappedNode(label: label) {
      deleteDir()
      checkout scm
      withEnv(["APT_MIRROR=cdn-fastly.deb.debian.org"]) {
        withTool(['dobi']) {
          sh "dobi ${task}"
        }
      }
    }
  }
}



echo "Branch ${env.BRANCH_NAME}"
switch (env.BRANCH_NAME) {
case "MASTER":
default:
  parallel (
    vendor: runTask('docker', 'validate-vendor'),
    validate: runTask('docker', 'validate'),
    test_unit: runTask('linux && x86_64 && !aufs', 'test-unit'),
  )
}
