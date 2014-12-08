#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

yum update -y
yum install -y docker-io git vim golang e2fsprogs tmux httpie ctags hg

if [[ ! -d /data/src/github.com/openshift/origin ]]; then
  mkdir -p /data/src/github.com/openshift/origin
  chown vagrant:vagrant /data/src/github.com/openshift/origin
fi

function set_env {
  USER_DIR=$1
  if [[ $(grep GOPATH $USER_DIR/.bash_profile) = "" ]]; then
    touch $USER_DIR/.bash_profile
    echo "export GOPATH=/data" >> $USER_DIR/.bash_profile
    echo "export PATH=\$GOPATH/src/github.com/openshift/origin/_output/local/go/bin:\$GOPATH/bin:\$PATH" >> $USER_DIR/.bash_profile
    echo "cd \$GOPATH/src/github.com/openshift/origin" >> $USER_DIR/.bash_profile
  else
    echo "path variables for $USER_DIR already configured"
  fi
}

set_env /home/vagrant
set_env /root

systemctl enable docker
systemctl start docker

usermod -a -G docker vagrant

echo To install etcd, run hack/install-etcd.sh

sed -i s/^Defaults.*requiretty/\#Defaults\ requiretty/g /etc/sudoers
