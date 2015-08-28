#!/bin/bash

path=$PATH
path=${path//:/ }

execpath(){
    if [ -L $2 ] ; then
        bin=`ls -l $2`
	A=${bin##*-> }
	if [[ ${A:0:1} != "/" ]] ; then
            echo "setcap" $3 $1/$A
            setcap $3 $1/$A
        else
            echo "setcap" $3 $A
            setcap $3 $A
        fi
    else
        echo "setcap" $3 $2
        setcap $3 $2
    fi
}

execsetcap(){
    for dir in $path
    do
        file=$dir/$1
        if [ -f $file ] ; then
            execpath $dir $file $2
            break
        fi
    done
}

execsetcap docker cap_chown,cap_dac_override,cap_fsetid,cap_fowner,cap_mknod,cap_net_raw,cap_net_admin,cap_setgid,cap_setuid,cap_setfcap,cap_setpcap,cap_net_bind_service,cap_sys_chroot,cap_kill,cap_audit_write,cap_sys_admin=ep
execsetcap iptables cap_net_admin,cap_net_raw=ei
execsetcap mkfs.ext4 cap_dac_override=ei
execsetcap tune2fs cap_dac_override=ei
