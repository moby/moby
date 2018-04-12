#!/usr/bin/env groovy
node {
    checkout scm
    def common = load("hack/jenkinsfile/common.jenkinsfile")
    
    stage("images-build") {
	parallel 'janky':{ ->
	    common.buildImage('janky', 'ubuntu-1604-aufs-edge')
	}, 'powerpc': { ->
	    common.buildImage("powerpc", 'ppc64le-ubuntu-1604')
	}, 'z': { ->
	    common.buildImage("z", 's390x-ubuntu-1604')
	}
    }

    stage("janky") {
	parallel 'validate':{ ->
	    common.validateStep('janky', 'ubuntu-1604-aufs-edge')
	}, 'janky.binary': { ->
	    common.binaryStep('janky', 'ubuntu-1604-aufs-edge')
	}, 'z.binary': { ->
	    common.binaryStep('z', 's390x-ubuntu-1604')
	}, 'powerpc.binary': { ->
	    common.binaryStep('powerpc', 'ppc64le-ubuntu-1604')
	}
    }

    stage("integration-triggers") {
	parallel 'janky-integration':{ ->
	    build job: 'vdemeester/moby-janky-integration' // FIXME(vdemeester) handle parameters
	}, 'powerpc-integration':{ ->
            build job: 'vdemeester/moby-powerpc-integration' // FIXME(vdemeester) handle parameters
	}, 'z-integration':{ ->
	    build job: 'vdemeester/moby-z-integration' // FIXME(vdemeester) handle parameters
	}
    }
}
