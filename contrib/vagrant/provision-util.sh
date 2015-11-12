#!/bin/bash
source "${ORIGIN_ROOT}/contrib/node/install-sdn.sh"

os::provision::join() {
  local IFS="$1"

  shift
  echo "$*"
}

os::provision::build-origin() {
  local origin_root=$1
  local skip_build=$2

  # This optimization is intended for devcluster use so hard-coding the
  # arch in the path should be ok.
  if [ -f "${origin_root}/_output/local/bin/linux/amd64/oc" ] &&
     [ "${skip_build}" = "true" ]; then
    echo "WARNING: Skipping openshift build due to OPENSHIFT_SKIP_BUILD=true"
  else
    echo "Building openshift"
    ${origin_root}/hack/build-go.sh
  fi
}

os::provision::build-etcd() {
  local origin_root=$1
  local skip_build=$2

  # TODO(marun) Only build etcd for dind deployments if the networking
  # option requires etcdctl (e.g. flannel).
  if [ -f "${origin_root}/_tools/etcd/bin/etcd" ] &&
     [ "${skip_build}" = "true" ]; then
    echo "WARNING: Skipping etcd build due to OPENSHIFT_SKIP_BUILD=true"
  else
    echo "Building etcd"
    ${origin_root}/hack/install-etcd.sh
  fi
}

os::provision::install-cmds() {
  local deployed_root=$1

  cp ${deployed_root}/_output/local/bin/linux/amd64/{openshift,oc,osadm} /usr/bin
}

os::provision::add-to-hosts-file() {
  local ip=$1
  local name=$2
  local force=${3:-0}

  if ! grep -q "${ip}" /etc/hosts || [ "${force}" = "1" ]; then
    local entry="${ip}\t${name}"
    echo -e "Adding '${entry}' to hosts file"
    echo -e "${entry}" >> /etc/hosts
  fi
}

os::provision::setup-hosts-file() {
  local master_name=$1
  local master_ip=$2
  local -n node_names=$3
  local -n node_ips=$4

  # Setup hosts file to support ping by hostname to master
  os::provision::add-to-hosts-file "${master_ip}" "${master_name}"

  # Setup hosts file to support ping by hostname to each node in the cluster
  for (( i=0; i < ${#node_names[@]}; i++ )); do
    os::provision::add-to-hosts-file "${node_ips[$i]}" "${node_names[$i]}"
  done
}

os::provision::init-certs() {
  local config_root=$1
  local network_plugin=$2
  local master_name=$3
  local master_ip=$4
  local -n node_names=$5
  local -n node_ips=$6

  local server_config_dir=${config_root}/openshift.local.config
  local volumes_dir="/var/lib/openshift.local.volumes"
  local cert_dir="${server_config_dir}/master"

  pushd "${config_root}" > /dev/null

  # Master certs
  /usr/bin/openshift admin ca create-master-certs \
    --overwrite=false \
    --cert-dir="${cert_dir}" \
    --master="https://${master_ip}:8443" \
    --hostnames="${master_ip},${master_name}"

  # Certs for nodes
  for (( i=0; i < ${#node_names[@]}; i++ )); do
    local name=${node_names[$i]}
    local ip=${node_ips[$i]}
    /usr/bin/openshift admin create-node-config \
      --node-dir="${server_config_dir}/node-${name}" \
      --node="${name}" \
      --hostnames="${name},${ip}" \
      --master="https://${master_ip}:8443" \
      --network-plugin="${network_plugin}" \
      --node-client-certificate-authority="${cert_dir}/ca.crt" \
      --certificate-authority="${cert_dir}/ca.crt" \
      --signer-cert="${cert_dir}/ca.crt" \
      --signer-key="${cert_dir}/ca.key" \
      --signer-serial="${cert_dir}/ca.serial.txt" \
      --volume-dir="${volumes_dir}"
  done

  popd > /dev/null
}

os::provision::set-os-env() {
  local origin_root=$1
  local config_root=$2

  # Set up the KUBECONFIG environment variable for use by oc.
  #
  # Target .bashrc since docker exec doesn't invoke .bash_profile and
  # .bash_profile loads .bashrc anyway.
  local file_target=".bashrc"

  local vagrant_target="/home/vagrant/${file_target}"
  if [ -d $(dirname "${vagrant_target}") ]; then
    os::provision::set-bash-env "${origin_root}" "${config_root}" \
"${vagrant_target}"
  fi
  os::provision::set-bash-env "${origin_root}" "${config_root}" \
"/root/${file_target}"

  # Make symlinks to the bash completions for the openshift commands
  ln -s ${origin_root}/contrib/completions/bash/* /etc/bash_completion.d/
}

os::provision::get-admin-config() {
    local config_root=$1

    echo "${config_root}/openshift.local.config/master/admin.kubeconfig"
}

os::provision::get-node-config() {
    local config_root=$1
    local node_name=$2

    echo "${config_root}/openshift.local.config/node-${node_name}/node-config.yaml"
}

os::provision::set-bash-env() {
  local origin_root=$1
  local config_root=$2
  local target=$3

  local path=$(os::provision::get-admin-config "${config_root}")
  local config_line="export KUBECONFIG=${path}"
  if ! grep -q "${config_line}" "${target}" &> /dev/null; then
    echo "${config_line}" >> "${target}"
    echo "cd ${origin_root}" >> "${target}"
  fi
}

os::provision::get-network-plugin() {
  local plugin=$1
  local dind_management_script=${2:-false}

  local subnet_plugin="redhat/openshift-ovs-subnet"
  local multitenant_plugin="redhat/openshift-ovs-multitenant"
  local default_plugin="${subnet_plugin}"

  if [ "${plugin}" != "${subnet_plugin}" ] && \
     [ "${plugin}" != "${multitenant_plugin}" ] && \
     [ "${plugin}" != "flannel" ]; then
    # Disable output when being called from the dind management script
    # since it may be doing something other than launching a cluster.
    if [ "${dind_management_script}" = "false" ]; then
      if [ "${plugin}" != "" ]; then
        >&2 echo "Invalid network plugin: ${plugin}"
      fi
      >&2 echo "Using default network plugin: ${default_plugin}"
    fi
    plugin="${default_plugin}"
  fi
  echo "${plugin}"
}

os::provision::install-networking() {
  local plugin=$1
  local master_ip=$2
  local origin_root=$3
  local config_root=$4
  local is_master=${5:-false}

  echo "Configuring networking"

  if [[ "${plugin}" =~ redhat/ ]]; then
    os::provision::install-sdn "${origin_root}"
  elif [[ "${plugin}" = "flannel" ]]; then
    os::provision::install-flannel "${master_ip}" "${origin_root}" \
        "${config_root}" "${is_master}"
  else
    >&2 echo "Unable to deploy network plugin ${plugin}"
    exit 1
  fi
}

os::provision::install-flannel() {
  local master_ip=$1
  local origin_root=$2
  local config_root=$3
  local is_master=$4

  if os::provision::in-container; then
    local if_name=eth0
  else
    yum install -y flannel

    local conf_path=/etc/sysconfig/network-scripts/
    local if_to_edit=$(find ${conf_path}ifcfg-* | xargs grep -l VAGRANT-BEGIN)
    local if_name=`echo ${if_to_edit} | awk -F- '{ print $3 }'`
  fi

  local flannel_conf="/etc/systemd/system/flanneld.service.d/dind.conf"

  mkdir -p $(dirname "${flannel_conf}")

  cat <<EOF > "${flannel_conf}"
[Service]
# Running flanneld with '-listen' or '-remote' requires changing the
# unit type to 'simple'.  The default is type 'notify', which would
# result in waiting forever for a notification that wouldn't come.
Type=simple

# Changing the flanneld unit type prevents ExecStartPost from
# triggering at the correct time.  The post start task
# (mk-docker-opts.sh) must instead be run manually once flannel has
# written its configuration.
ExecStartPost=

# Clear the default ExecStart
ExecStart=
EOF

  if [ "${is_master}" = "true" ]; then
    os::provision::flannel-configure-master "${master_ip}" "${origin_root}" \
       "${config_root}" "${if_name}" "${flannel_conf}"
  else
    cat <<EOF >> "${flannel_conf}"
ExecStart=/usr/bin/flanneld -iface=${if_name} -ip-masq=false -v=5\
 -remote=${master_ip}:8080
EOF
  fi

  systemctl enable flanneld
  systemctl start flanneld

  # Ensure that docker on the nodes is configured for flannel.
  if [ "${is_master}" != "true" ]; then
    os::provision::flannel-configure-docker
  fi
}

os::provision::flannel-configure-master() {
  local master_ip=$1
  local origin_root=$2
  local config_root=$3
  local if_name=$4
  local flannel_conf=$5

  local cert_path="${config_root}/openshift.local.config/master"
  local ca_file="${cert_path}/ca.crt"
  local cert_file="${cert_path}/master.etcd-client.crt"
  local key_file="${cert_path}/master.etcd-client.key"
  local etcd_url="https://${master_ip}:4001"
  local etcd_key="/atomic.io/network"

  cat <<EOF >> "${flannel_conf}"
ExecStart=/usr/bin/flanneld -iface=${if_name} -ip-masq=false -v=5\
 -etcd-cafile=${ca_file} -etcd-certfile=${cert_file} -etcd-keyfile=${key_file}\
 -etcd-prefix=${etcd_key} -etcd-endpoints=${etcd_url} -listen=${master_ip}:8080
EOF

  local etcdctl_cmd="${origin_root}/_tools/etcd/bin/etcdctl \
-C ${etcd_url} --ca-file ${ca_file} --cert-file ${cert_file} --key-file \
${key_file}"

  local msg="etcd daemon to become available"
  local condition="os::provision::check-etcd ${etcdctl_cmd}"
  os::provision::wait-for-condition "${msg}" "${condition}"

  cat <<EOF > /etc/flannel-config.json
{
"Network": "10.1.0.0/16",
"SubnetLen": 24,
"Backend": {
    "Type": "udp",
    "Port": 8285
 }
}
EOF

  # Import default configuration into etcd
  ${etcdctl_cmd} set "${etcd_key}/config" < /etc/flannel-config.json \
      > /dev/null
}

os::provision::check-etcd() {
  $@ ls &> /dev/null
}

os::provision::flannel-configure-docker() {
  local msg="flannel configuration to become available"
  local condition="test -f /run/flannel/subnet.env"

  # The master flannel instance may take a while to become available.
  os::provision::wait-for-condition "${msg}" "${condition}" \
      "${OS_WAIT_FOREVER}"

  # Generate the docker configuration from the flannel configuration
  /usr/libexec/flannel/mk-docker-opts.sh -d /run/flannel/docker \
      -k DOCKER_NETWORK_OPTIONS

  echo "Reloading docker"
  systemctl stop docker
  ip link del docker0
  systemctl start docker
}

os::provision::base-provision() {
  local is_master=${1:-false}

  os::provision::fixup-net-udev

  os::provision::setup-hosts-file "${MASTER_NAME}" "${MASTER_IP}" NODE_NAMES \
    NODE_IPS

  os::provision::install-pkgs

  # Avoid enabling iptables on the master since it will
  # prevent access to the openshift api from outside the master.
  if [[ "${is_master}" != "true" ]]; then
    # Avoid enabling iptables when firewalld is already enabled.
    if ! systemctl is-enabled -q firewalld 2> /dev/null; then
      # A default deny firewall (either iptables or firewalld) is
      # installed by default on non-cloud fedora and rhel, so all
      # network plugins need to be able to work with one enabled.
      systemctl enable iptables.service
      systemctl start iptables.service
    fi
  fi
}

os::provision::fixup-net-udev() {
  if [ "${FIXUP_NET_UDEV}" == "true" ]; then
    NETWORK_CONF_PATH=/etc/sysconfig/network-scripts/
    rm -f ${NETWORK_CONF_PATH}ifcfg-enp*
    if [[ -f "${NETWORK_CONF_PATH}ifcfg-eth1" ]]; then
      sed -i 's/^NM_CONTROLLED=no/#NM_CONTROLLED=no/' ${NETWORK_CONF_PATH}ifcfg-eth1
      if ! grep -q "NAME=" ${NETWORK_CONF_PATH}ifcfg-eth1; then
        echo "NAME=openshift" >> ${NETWORK_CONF_PATH}ifcfg-eth1
      fi
      nmcli con reload
      nmcli dev disconnect eth1
      nmcli con up "openshift"
    fi
  fi
}

os::provision::in-container() {
  test -f /.dockerinit
}

os::provision::install-pkgs() {
  # Only install packages if not deploying to a container.  A
  # container is expected to have installed packages as part of image
  # creation.
  if ! os::provision::in-container; then
    yum install -y deltarpm
    yum update -y
    yum install -y docker-io git golang e2fsprogs hg net-tools bridge-utils \
      which ethtool bash-completion iptables-services

    systemctl enable docker
    systemctl start docker
  fi
}

os::provision::start-os-service() {
  local unit_name=$1
  local description=$2
  local exec_start=$3
  local work_dir=$4

  local dind_env_var=
  if os::provision::in-container; then
    dind_env_var="OPENSHIFT_DIND=true"
  fi

  cat <<EOF > "/usr/lib/systemd/system/${unit_name}.service"
[Unit]
Description=${description}
Requires=network.target
After=docker.target network.target

[Service]
Environment=${dind_env_var}
ExecStart=${exec_start}
WorkingDirectory=${work_dir}
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload > /dev/null
  systemctl enable "${unit_name}.service" &> /dev/null
  systemctl start "${unit_name}.service"
}

os::provision::copy-config() {
  local config_root=$1

  # Copy over the certificates directory so that each node has a copy.
  cp -r "${config_root}/openshift.local.config" /
  if [ -d /home/vagrant ]; then
    chown -R vagrant.vagrant /openshift.local.config
  fi
}

os::provision::start-node-service() {
  local config_root=$1
  local node_name=$2

  cmd="/usr/bin/openshift start node --loglevel=${LOG_LEVEL} \
--config=$(os::provision::get-node-config ${config_root} ${node_name})"
  os::provision::start-os-service "openshift-node" "OpenShift Node" "${cmd}" \
      "${config_root}"
}

OS_WAIT_FOREVER=-1
os::provision::wait-for-condition() {
  local msg=$1
  # condition should be a string that can be eval'd.  When eval'd, it
  # should not output anything to stderr or stdout.
  local condition=$2
  local timeout=${3:-30}

  local start_msg="Waiting for ${msg}"
  local error_msg="[ERROR] Timeout waiting for ${msg}"

  local counter=0
  while ! $(${condition}); do
    if [ "${counter}" = "0" ]; then
      echo "${start_msg}"
    fi

    if [[ "${counter}" -lt "${timeout}" ]] || \
       [[ "${timeout}" = "${OS_WAIT_FOREVER}" ]]; then
      counter=$((counter + 1))
      if [[ "${timeout}" != "${OS_WAIT_FOREVER}" ]]; then
        echo -n '.'
      fi
      sleep 1
    else
      echo -e "\n${error_msg}"
      return 1
    fi
  done

  if [ "${counter}" != "0" ]; then
    if [ "${timeout}" != "${OS_WAIT_FOREVER}" ]; then
      echo -e '\nDone'
    fi
  fi
}

os::provision::is-sdn-node-registered() {
  local node_name=$1

  oc get nodes "${node_name}" &> /dev/null
}

os::provision::disable-sdn-node() {
  local config_root=$1
  local node_name=$2

  export KUBECONFIG=$(os::provision::get-admin-config "${config_root}")

  local msg="sdn node to register with the master"
  local condition="os::provision::is-sdn-node-registered ${node_name}"
  os::provision::wait-for-condition "${msg}" "${condition}"

  echo "Disabling scheduling for the sdn node"
  osadm manage-node "${node_name}" --schedulable=false > /dev/null
}

os::provision::wait-for-node-config() {
  local config_root=$1
  local node_name=$2

  local msg="node configuration file"
  local config_file=$(os::provision::get-node-config "${config_root}" \
    "${node_name}")
  local condition="test -f ${config_file}"
  os::provision::wait-for-condition "${msg}" "${condition}" \
    "${OS_WAIT_FOREVER}"
}
