#!/usr/bin/env groovy
node {
    checkout scm
    def common = load("hack/jenkinsfile/common.jenkinsfile")
    
    def test_steps = [
	'powerpc.integration': { ->
	    common.testIntegrationStep('powerpc', 'ppc64le-ubuntu-1604')
	}, 'powerpc.test-unit': { ->
	    common.testUnitStep('powerpc', 'ppc64le-ubuntu-1604')
	}
    ]

    for (s in common.getTestSuites()) {
	test_steps << common.genTestIntegrationCliStep(s, 'powerpc', 'ppc64le-ubuntu-1604')
    }

    stage("integration") {
	parallel(test_steps)
    }
}
