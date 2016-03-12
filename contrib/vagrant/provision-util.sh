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
    if os::provision::in-container; then
      # Default to disabling use of a release build for dind to allow
      # ci to validate a developer's dev cluster workflow.
      export OS_RELEASE=${OS_RELEASE:-n}
    fi
    ${origin_root}/hack/build-go.sh
  fi
}

os::provision::build-etcd() {
  local origin_root=$1
  local skip_build=$2

  if [ -f "${origin_root}/_tools/etcd/bin/etcd" ] &&
     [ "${skip_build}" = "true" ]; then
    echo "WARNING: Skipping etcd build due to OPENSHIFT_SKIP_BUILD=true"
  # Etcd is required for integration testing which isn't a use case
  # for dind.
  elif ! os::provision::in-container; then
    echo "Building etcd"
    ${origin_root}/hack/install-etcd.sh
  fi
}

os::provision::base-install() {
  local origin_root=$1
  local config_root=$2

  echo "Installing openshift"
  os::provision::install-cmds "${origin_root}"
  os::provision::install-sdn "${origin_root}"
  os::provision::set-os-env "${origin_root}" "${config_root}"
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

  # Indicate to nodes that it's safe to begin provisioning by removing
  # the stale marker.
  rm -f ${config_root}/openshift.local.config/.stale
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
     [ "${plugin}" != "${multitenant_plugin}" ]; then
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

os::provision::base-provision() {
  local origin_root=$1
  local is_master=${2:-false}

  # Add a convenience symlink to the gopath repo
  ln -sf "${origin_root}" /

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

      # Ensure that the master can access the kubelet for capabilities
      # like 'oc exec'.  Explicitly specifying the insertion location
      # is brittle but the tests should catch conflicts with the
      # package rules.
      iptables -I INPUT 4 -p tcp -m state --state NEW --dport 10250 -j ACCEPT
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
  local work_dir=${4:-${CONFIG_ROOT}/}

  local dind_env_var=
  if os::provision::in-container; then
    dind_env_var="OPENSHIFT_DIND=true"
  fi

  cat <<EOF > "/etc/systemd/system/${unit_name}.service"
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

os::provision::start-node-service() {
  local config_root=$1
  local node_name=$2

  # Copy over the certificates directory so that each node has a copy.
  cp -r "${config_root}/openshift.local.config" /
  if [ -d /home/vagrant ]; then
    chown -R vagrant.vagrant /openshift.local.config
  fi

  cmd="/usr/bin/openshift start node --loglevel=${LOG_LEVEL} \
--config=$(os::provision::get-node-config ${config_root} ${node_name})"
  os::provision::start-os-service "openshift-node" "OpenShift Node" "${cmd}" /
}

OS_WAIT_FOREVER=-1
os::provision::wait-for-condition() {
  local msg=$1
  # condition should be a string that can be eval'd.  When eval'd, it
  # should not output anything to stderr or stdout.
  local condition=$2
  local timeout=${3:-60}

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
  local condition="test ! -f ${config_root}/openshift.local.config/.stale -a \
-f ${config_file}"
  os::provision::wait-for-condition "${msg}" "${condition}" \
    "${OS_WAIT_FOREVER}"
}
