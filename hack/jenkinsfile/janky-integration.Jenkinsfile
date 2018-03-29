#!/usr/bin/env groovy
node {
    checkout scm
    def common = load("hack/jenkinsfile/common.jenkinsfile")
    
    def test_steps = [
	'janky.integration': { ->
	    common.testIntegrationStep('janky', 'ubuntu-1604-aufs-edge')
	}, 'janky.test-unit': { ->
	    common.testUnitStep('janky', 'ubuntu-1604-aufs-edge')
	}
    ]

    for (s in common.getTestSuites()) {
	test_steps << common.genTestIntegrationCliStep(s, 'janky', 'ubuntu-1604-aufs-edge')
    }

    stage("integration") {
	parallel(test_steps)
    }
}
