class virtualbox {
    Package { ensure => "installed" }

    # remove some files from the base vagrant image because they're old
    file { "/home/vagrant/docker-master":
        ensure => absent,
        recurse => true,
        force => true,
        purge => true,
    }
    file { "/usr/local/bin/dockerd":
        ensure => absent,
    }
    file { "/usr/local/bin/docker":
        ensure => absent,
    }

    # Set up VirtualBox guest utils
    package { "virtualbox-guest-utils": }
    exec { "vbox-add" :
        command => "/etc/init.d/vboxadd setup",
        require => [
            Package["virtualbox-guest-utils"],
            Package["linux-headers-3.5.0-25-generic"], ],
    }
}

class docker {
    # update this with latest go binary dist
    $go_url = "http://go.googlecode.com/files/go1.0.3.linux-amd64.tar.gz"

    Package { ensure => "installed" }

    package { ["lxc", "debootstrap", "wget", "bsdtar", "git",
               "pkg-config", "libsqlite3-dev",
               "linux-image-3.5.0-25-generic",
               "linux-image-extra-3.5.0-25-generic",
               "linux-headers-3.5.0-25-generic"]: }

    $ec2_version = file("/etc/ec2_version", "/dev/null")
    $rax_version = inline_template("<%= %x{/usr/bin/xenstore-read vm-data/provider_data/provider} %>")

    if ($ec2_version) {
        $vagrant_user = "ubuntu"
        $vagrant_home = "/home/ubuntu"
    } elsif ($rax_version) {
        $vagrant_user = "root"
        $vagrant_home = "/root"
    } else {
        # virtualbox is the vagrant default, so it should be safe to assume
        $vagrant_user = "vagrant"
        $vagrant_home = "/home/vagrant"
        include virtualbox
    }

    exec { "fetch-go":
        require => Package["wget"],
        command => "/usr/bin/wget -O - $go_url | /bin/tar xz -C /usr/local",
        creates => "/usr/local/go/bin/go",
    }

    file { "/etc/init/dockerd.conf":
        mode => 600,
        owner => "root",
        group => "root",
        content => template("docker/dockerd.conf"),
    }

    file { "/opt/go":
        owner => $vagrant_user,
        group => $vagrant_user,
        recurse => true,
    }

    file { "${vagrant_home}/.profile":
        mode => 644,
        owner => $vagrant_user,
        group => $vagrant_user,
        content => template("docker/profile"),
    }

     exec { "build-docker" :
        cwd  => "/opt/go/src/github.com/dotcloud/docker",
        user => $vagrant_user,
        environment => "GOPATH=/opt/go",
        command => "/usr/local/go/bin/go get -v ./... && /usr/local/go/bin/go install ./docker",
        creates => "/opt/go/bin/docker",
        logoutput => "on_failure",
        require => [ Exec["fetch-go"], File["/opt/go"] ],
    }

    service { "dockerd" :
        ensure => "running",
        start => "/sbin/initctl start dockerd",
        stop => "/sbin/initctl stop dockerd",
        require => [ Exec["build-docker"], File["/etc/init/dockerd.conf"] ],
        name => "dockerd",
        provider => "base"
    }
}
