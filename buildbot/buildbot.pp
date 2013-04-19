node default {
    $USER = 'vagrant'
    $ROOT_PATH = '/data/buildbot'
    $DOCKER_PATH = '/data/docker'

    exec {'apt_update': command => '/usr/bin/apt-get update' }
    Package { require => Exec['apt_update'] }
    group {'puppet': ensure => 'present'}

    # Install dependencies
    Package { ensure => 'installed' }
    package { ['python-dev','python-pip','supervisor','lxc','bsdtar','git','golang']: }

    file{[ '/data' ]:
        owner => $USER, group => $USER, ensure => 'directory' }

    file {'/var/tmp/requirements.txt':
        content => template('requirements.txt') }

    exec {'requirements':
        require => [ Package['python-dev'], Package['python-pip'],
            File['/var/tmp/requirements.txt'] ],
        cwd     => '/var/tmp',
        command => "/bin/sh -c '(/usr/bin/pip install -r requirements.txt;
            rm /var/tmp/requirements.txt)'" }

    exec {'buildbot-cfg-sh':
        require => [ Package['supervisor'], Exec['requirements']],
        path    => '/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin',
        cwd     => '/data',
        command => "$DOCKER_PATH/buildbot/buildbot-cfg/buildbot-cfg.sh" }
}
