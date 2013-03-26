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

	# Set up VirtualBox guest utils
	package { "virtualbox-guest-utils": }
    exec { "vbox-add" :
        command => "/etc/init.d/vboxadd setup",
        require => [
			Package["virtualbox-guest-utils"],
			Package["linux-headers-3.5.0-25-generic"], ],
    }
}

class ec2 {
}

class rax {
}

class docker {

    # update this with latest docker binary distro
    $docker_url = "http://get.docker.io/builds/$kernel/$hardwaremodel/docker-master.tgz"
    # update this with latest go binary distry
    $go_url = "http://go.googlecode.com/files/go1.0.3.linux-amd64.tar.gz"

    Package { ensure => "installed" }

    package { ["lxc", "debootstrap", "wget", "bsdtar", "git",
               "pkg-config", "libsqlite3-dev",
               "linux-image-3.5.0-25-generic",
               "linux-image-extra-3.5.0-25-generic",
               "linux-headers-3.5.0-25-generic"]: }

    notify { "docker_url = $docker_url": withpath => true }

    $ec2_version = file("/etc/ec2_version", "/dev/null")
    $rax_version = inline_template("<%= %x{/usr/bin/xenstore-read vm-data/provider_data/provider} %>")

    if ($ec2_version) {
	include ec2
    } elsif ($rax_version) {
        include rax
    } else {
    # virtualbox is the vagrant default, so it should be safe to assume
        include virtualbox
    }

    user { "vagrant":
        ensure => present,
        comment => "Vagrant User",
        shell => "/bin/bash",
        home => "/home/vagrant",
        groups => [
            "sudo",
            "vagrant",
            "ubuntu",
        ],
        require => [
            Group["sudo"],
            Group["vagrant"],
            Group["ubuntu"],
        ],
    }

	group { "ubuntu":
		ensure => present,
	}

	group { "vagrant":
		ensure => present,
	}

	group { "sudo":
		ensure => present,
	}

	file { "/usr/local/bin":
		ensure => directory,
		owner => root,
		group => root,
		mode => 755,
	}

    exec { "fetch-go":
        require => Package["wget"],
        command => "/usr/bin/wget -O - $go_url | /bin/tar xz -C /usr/local",
        creates => "/usr/local/go/bin/go",
    }

    exec { "fetch-docker" :
        command => "/usr/bin/wget -O - $docker_url | /bin/tar xz -C /tmp",
        require => Package["wget"],
    }

    file { "/etc/init/dockerd.conf":
        mode => 600,
        owner => "root",
        group => "root",
        content => template("docker/dockerd.conf"),
        require => Exec["copy-docker-bin"],
    }

    file { "/home/vagrant":
        ensure => directory,
        mode => 644,
        require => User["vagrant"],
    }

    file { "/home/vagrant/.profile":
        mode => 644,
        owner => "vagrant",
        group => "vagrant",
        content => template("docker/profile"),
        require => File["/home/vagrant"],
    }

    exec { "copy-docker-bin" :
        command => "/usr/bin/sudo /bin/cp -f /tmp/docker-master/docker /usr/local/bin/",
        require => [ Exec["fetch-docker"], File["/usr/local/bin"] ],
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
