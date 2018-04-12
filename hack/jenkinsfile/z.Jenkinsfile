#!/usr/bin/env groovy
node {
    checkout scm
    def common = load("hack/jenkinsfile/common.jenkinsfile")
    
    def test_steps = [
	'z.integration': { ->
	    common.testIntegrationStep('z', 's390x-ubuntu-1604')
	}, 'z.test-unit': { ->
	    common.testUnitStep('z', 's390x-ubuntu-1604')
	}
    ]

    for (s in common.getTestSuites()) {
	test_steps << common.genTestIntegrationCliStep(s, 'z', 's390x-ubuntu-1604')
    }

    stage("integration") {
	parallel(test_steps)
    }
}
