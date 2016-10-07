

def runTask(String task) {
  wrappedNode(label: 'docker') {
    deleteDir()
    checkout scm
    withTool(['dobi']) {
        sh "dobi ${task}"
    }
  }
}



echo "Branch {env.BRANCH_NAME}"
switch (env.BRANCH_NAME) {
case "MASTER":
default:
  parallel {
    vendor: runTask('validate-vendor'),
    validate: runTask('validate'),
    test_unit: runTask('test-unit'),
  }
}
