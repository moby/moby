class docker {

    # update this with latest docker binary distro
    $docker_url = "http://docker.io.s3.amazonaws.com/builds/$kernel/$hardwaremodel/docker-master.tgz"
    # update this with latest go binary distry
    $go_url = "http://go.googlecode.com/files/go1.0.3.linux-amd64.tar.gz"

    Package { ensure => "installed" }

    package { ["lxc", "debootstrap", "wget", "bsdtar", "git",
               "linux-image-3.5.0-25-generic",
               "linux-image-extra-3.5.0-25-generic",
               "virtualbox-guest-utils",
               "linux-headers-3.5.0-25-generic"]: }

    notify { "docker_url = $docker_url": withpath => true }

    exec { "debootstrap" :
        require => Package["debootstrap"],
        command => "/usr/sbin/debootstrap --arch=amd64 quantal /var/lib/docker/images/docker-ut",
        creates => "/var/lib/docker/images/docker-ut",
        timeout => 0
    }

    exec { "fetch-go":
        require => Package["wget"],
        command => "/usr/bin/wget -O - $go_url | /bin/tar xz -C /usr/local",
        creates => "/usr/local/go/bin/go",
    }

    exec { "fetch-docker" :
        require => Package["wget"],
        command => "/usr/bin/wget -O - $docker_url | /bin/tar xz -C /home/vagrant",
        creates => "/home/vagrant/docker-master"
    }

    file { "/etc/init/dockerd.conf":
        mode => 600,
        owner => "root",
        group => "root",
        content => template("docker/dockerd.conf"),
        require => [Exec["fetch-docker"], Exec["debootstrap"]]
    }

    exec { "copy-docker-bin" :
        require => Exec["fetch-docker"],
        command => "/bin/cp /home/vagrant/docker-master/docker /usr/local/bin",
        creates => "/usr/local/bin/docker"
    }

    exec { "copy-dockerd-bin" :
        require => Exec["fetch-docker"],
        command => "/bin/cp /home/vagrant/docker-master/dockerd /usr/local/bin",
        creates => "/usr/local/bin/dockerd"
    }

    exec { "vbox-add" :
        require => Package["linux-headers-3.5.0-25-generic"],
        command => "/etc/init.d/vboxadd setup",
    }

    service { "dockerd" :
        ensure => "running",
        start => "/sbin/initctl start dockerd",
        stop => "/sbin/initctl stop dockerd",
        require => File["/etc/init/dockerd.conf"],
        name => "dockerd",
        provider => "base"
    }


}
