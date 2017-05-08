# Some bits borrowed from the openstack-selinux package
Name: docker-engine-selinux
Version: %{_version}
Release: %{_release}%{?dist}
Summary: SELinux Policies for the open-source application container engine
BuildArch: noarch
Group: Tools/Docker

License: GPLv2
Source: %{name}.tar.gz

URL: https://dockerproject.org
Vendor: Docker
Packager: Docker <support@docker.com>

# Version of SELinux we were using
%if 0%{?fedora} == 20
%global selinux_policyver 3.12.1-197
%endif # fedora 20
%if 0%{?fedora} == 21
%global selinux_policyver 3.13.1-105
%endif # fedora 21
%if 0%{?fedora} >= 22
%global selinux_policyver 3.13.1-128
%endif # fedora 22
%if 0%{?centos} >= 7 || 0%{?rhel} >= 7 || 0%{?oraclelinux} >= 7
%global selinux_policyver 3.13.1-23
%endif # centos,rhel,oraclelinux 7

%global selinuxtype targeted
%global moduletype  services
%global modulenames docker

Requires(post): selinux-policy-base >= %{selinux_policyver}, selinux-policy-targeted >= %{selinux_policyver}, policycoreutils, policycoreutils-python libselinux-utils
BuildRequires: selinux-policy selinux-policy-devel

# conflicting packages
Conflicts: docker-selinux

# Usage: _format var format
#   Expand 'modulenames' into various formats as needed
#   Format must contain '$x' somewhere to do anything useful
%global _format() export %1=""; for x in %{modulenames}; do %1+=%2; %1+=" "; done;

# Relabel files
%global relabel_files() \
    /sbin/restorecon -R %{_bindir}/docker %{_localstatedir}/run/docker.sock %{_localstatedir}/run/docker.pid %{_sysconfdir}/docker %{_localstatedir}/log/docker %{_localstatedir}/log/lxc %{_localstatedir}/lock/lxc %{_usr}/lib/systemd/system/docker.service /root/.docker &> /dev/null || : \

%description
SELinux policy modules for use with Docker

%prep
%if 0%{?centos} <= 6
%setup -n %{name}
%else
%autosetup -n %{name}
%endif

%build
make SHARE="%{_datadir}" TARGETS="%{modulenames}"

%install

# Install SELinux interfaces
%_format INTERFACES $x.if
install -d %{buildroot}%{_datadir}/selinux/devel/include/%{moduletype}
install -p -m 644 $INTERFACES %{buildroot}%{_datadir}/selinux/devel/include/%{moduletype}

# Install policy modules
%_format MODULES $x.pp.bz2
install -d %{buildroot}%{_datadir}/selinux/packages
install -m 0644 $MODULES %{buildroot}%{_datadir}/selinux/packages

%post
#
# Install all modules in a single transaction
#
if [ $1 -eq 1 ]; then
    %{_sbindir}/setsebool -P -N virt_use_nfs=1 virt_sandbox_use_all_caps=1
fi
%_format MODULES %{_datadir}/selinux/packages/$x.pp.bz2
%{_sbindir}/semodule -n -s %{selinuxtype} -i $MODULES
if %{_sbindir}/selinuxenabled ; then
    %{_sbindir}/load_policy
    %relabel_files
    if [ $1 -eq 1 ]; then
	restorecon -R %{_sharedstatedir}/docker
    fi
fi

%postun
if [ $1 -eq 0 ]; then
    %{_sbindir}/semodule -n -r %{modulenames} &> /dev/null || :
    if %{_sbindir}/selinuxenabled ; then
        %{_sbindir}/load_policy
        %relabel_files
    fi
fi

%files
%doc LICENSE
%defattr(-,root,root,0755)
%attr(0644,root,root) %{_datadir}/selinux/packages/*.pp.bz2
%attr(0644,root,root) %{_datadir}/selinux/devel/include/%{moduletype}/*.if

%changelog
* Tue Dec 1 2015 Jessica Frazelle <acidburn@docker.com> 1.9.1-1
- add licence to rpm
- add selinux-policy and docker-engine-selinux rpm
