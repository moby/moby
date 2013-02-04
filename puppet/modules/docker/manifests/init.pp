class docker {

    # update this with latest docker binary distro
    # XXX: this is actually a bzip2 file rather than gzip despite extension
    $docker_url = "https://dl.dropbox.com/u/20637798/docker.tar.gz"
    # update this with latest go binary distry
    $go_url = "http://go.googlecode.com/files/go1.0.3.linux-amd64.tar.gz"


    Package { ensure => "installed" }

    package { ["lxc", "debootstrap", "wget"]: }

    exec { "debootstrap" :
        require => Package["debootstrap"],
        command => "/usr/sbin/debootstrap --arch=amd64 quantal /var/lib/docker/images/ubuntu",
        creates => "/var/lib/docker/images/ubuntu",
        timeout => 0
    }

    exec { "fetch-go":
        require => Package["wget"],
        command => "/usr/bin/wget -O - $go_url | /bin/tar xz -C /usr/local",
        creates => "/usr/local/go/bin/go",
    }

    exec { "fetch-docker" :
        require => Package["wget"],
        command => "/usr/bin/wget -O - $docker_url | /bin/tar xj -C /home/vagrant",
        creates => "/home/vagrant/docker/dockerd"
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
        command => "/bin/cp /home/vagrant/docker/docker /usr/local/bin",
        creates => "/usr/local/bin/docker"
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
