node default {
    exec {
        "apt_update" : 
            command => "/usr/bin/apt-get update"
    }

    Package {
        require => Exec['apt_update']
    }

    group { "puppet":
        ensure => "present"
    }

    include "docker"

}
