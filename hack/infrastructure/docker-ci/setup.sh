#!/usr/bin/env bash

# Set timezone
echo "GMT" >/etc/timezone
dpkg-reconfigure --frontend noninteractive tzdata

# Set ssh superuser
mkdir -p /data/buildbot /var/run/sshd /run
useradd -m -d /home/sysadmin -s /bin/bash -G sudo,docker -p '*' sysadmin
sed -Ei 's/(\%sudo.*) ALL/\1 NOPASSWD:ALL/' /etc/sudoers
cd /home/sysadmin
mkdir .ssh
chmod 700 .ssh
cat > .ssh/authorized_keys << 'EOF'
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQC7ALVhwQ68q1SjrKaAduOuOEAcWmb8kDZf5qA7T1fM8AP07EDC7nSKRJ8PXUBGTOQfxm89coJDuSJsTAZ+1PvglXhA0Mq6+knc6ZrZY+SuZlDIDAk4TOdVPoDZnmR1YW2McxHkhcGIOKeC8MMig5NeEjtgQwXzauUSPqeh8HMlLZRMooFYyyluIpn7NaCLzyWjwAQz2s3KyI7VE7hl+ncCrW86v+dciEdwqtzNoUMFb3iDpPxaiCl3rv+SB7co/5eUDTs1FZvUcYMXKQuf8R+2ZKzXOpwr0Zs8sKQXvXavCeWykwGgXLBjVkvrDcHuDD6UXCW63UKgmRECpLZaMBVIIRWLEEgTS5OSQTcxpMVe5zUW6sDvXHTcdPwWrcn1dE9F/0vLC0HJ4ADKelLX5zyTpmXGbuZuntIf1JO67D/K/P++uV1rmVIH+zgtOf23w5rX2zKb4BSTqP0sv61pmWV7MEVoEz6yXswcTjS92tb775v7XLU9vKAkt042ORFdE4/++hejhL/Lj52IRgjt1CJZHZsR9JywJZrz3kYuf8eU2J2FYh0Cpz5gmf0f+12Rt4HztnZxGPP4KuMa66e4+hpx1jynjMZ7D5QUnNYEmuvJByopn8HSluuY/kS5MMyZCZtJLEPGX4+yECX0Di/S0vCRl2NyqfCBqS+yXXT5SA1nFw== docker-test@docker.io
EOF
chmod 600 .ssh/authorized_keys
chown -R sysadmin .ssh

# Fix docker group id for use of host dockerd by sysadmin
sed -Ei 's/(docker:x:)[^:]+/\1999/' /etc/group

# Create buildbot configuration
cd /data/buildbot; buildbot create-master master
cp -a /data/buildbot/master/master.cfg.sample \
    /data/buildbot/master/master.cfg
cd /data/buildbot; \
    buildslave create-slave slave localhost:9989 buildworker pass
cp /docker-ci/buildbot/master.cfg /data/buildbot/master

# Patch github webstatus to capture pull requests
cp /docker-ci/buildbot/github.py /usr/local/lib/python2.7/dist-packages/buildbot/status/web/hooks
chown -R sysadmin.sysadmin /data

# Create nginx configuration
rm /etc/nginx/sites-enabled/default
cp /docker-ci/nginx/nginx.conf /etc/nginx/conf.d/buildbot.conf
/bin/echo -e '\ndaemon off;\n' >> /etc/nginx/nginx.conf

# Set supervisord buildbot, nginx and sshd processes
/bin/echo -e "\
[program:buildmaster]\n\
command=twistd --nodaemon --no_save -y buildbot.tac\n\
directory=/data/buildbot/master\n\
user=sysadmin\n\n\
[program:buildworker]\n\
command=twistd --nodaemon --no_save -y buildbot.tac\n\
directory=/data/buildbot/slave\n\
user=sysadmin\n" > \
    /etc/supervisor/conf.d/buildbot.conf
/bin/echo -e "[program:nginx]\ncommand=/usr/sbin/nginx\n" > \
    /etc/supervisor/conf.d/nginx.conf
/bin/echo -e "[program:sshd]\ncommand=/usr/sbin/sshd -D\n" > \
    /etc/supervisor/conf.d/sshd.conf
