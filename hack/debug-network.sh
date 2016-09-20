#!/bin/bash

# echoes args to stderr and exits
die () {
    echo "$*" 1>&2
    exit 1
}

# echoes the command provided as $@ and then runs it
echo_and_eval () {
    echo "> $*"
    echo ""
    eval "$@"
}

# runs the command provided as $@, and either returns silently with
# status 0 or else logs an error message with the command's output
try_eval () {
    tmpfile=`mktemp`
    if ! eval "$@" >& $tmpfile; then
	status=1
	echo "ERROR: Could not run '$*':"
	sed -e 's/^/  /' $tmpfile
	echo ""
    else
	status=0
    fi
    rm -f $tmpfile
    return $status
}

# The environment may contain sensitive information like passwords or private keys
filter_env () {
    awk '/ env:$/ { indent = index($0, "e"); skipping = 1; next } !skipping { print; } skipping { ch = substr($0, indent, 1); if (ch != " " && ch != "-") { skipping = 0; print } }'
}

log_service () {
    logpath=$1
    service=$2
    start_args=$3

    echo_and_eval  journalctl -u $service                         &> $logpath/journal-$service
    echo_and_eval  systemctl show $service                        &> $logpath/systemctl-show-$service

    config_file=$(get_config_path_from_service "$start_args" "$service")
    if [ -f "$config_file" ]; then
        echo_and_eval  cat $config_file                           &> $logpath/CONFIG-$service
    fi
}

log_system () {
    logpath=$1

    echo_and_eval  journalctl --boot                                  &> $logpath/journal-full
    echo_and_eval  nmcli --nocheck -f all dev show                    &> $logpath/nmcli-dev
    echo_and_eval  nmcli --nocheck -f all con show                    &> $logpath/nmcli-con
    echo_and_eval  head -1000 /etc/sysconfig/network-scripts/ifcfg-*  &> $logpath/ifcfg
    echo_and_eval  ip addr show                                       &> $logpath/addresses
    echo_and_eval  ip route show                                      &> $logpath/routes
    echo_and_eval  ip neighbor show                                   &> $logpath/arp
    echo_and_eval  iptables-save                                      &> $logpath/iptables
    echo_and_eval  cat /etc/hosts                                     &> $logpath/hosts
    echo_and_eval  cat /etc/resolv.conf                               &> $logpath/resolv.conf
    echo_and_eval  lsmod                                              &> $logpath/modules
    echo_and_eval  sysctl -a                                          &> $logpath/sysctl

    echo_and_eval  oc version                                         &> $logpath/version
    echo                                                             &>> $logpath/version
    echo_and_eval  docker version                                    &>> $logpath/version
    echo                                                             &>> $logpath/version
    echo_and_eval  cat /etc/system-release-cpe                       &>> $logpath/version
}

do_master () {
    if ! nodes=$(oc get nodes --template '{{range .items}}{{.spec.externalID}} {{end}}'); then
	if [ -z "$KUBECONFIG" -o ! -f "$KUBECONFIG" ]; then
	    die "KUBECONFIG is unset or incorrect"
	else
	    die "Could not get list of nodes"
	fi
    fi

    logmaster=$logdir/master
    mkdir -p $logmaster

    # Grab master service logs and config files
    if [ -n "$aos_master_service" ]; then
        log_service $logmaster $aos_master_service "master"
    fi
    if [ -n "$aos_master_controllers_service" ]; then
        log_service $logmaster $aos_master_controllers_service "master controllers"
    fi
    if [ -n "$aos_master_api_service" ]; then
        log_service $logmaster $aos_master_api_service "master api"
    fi

    # Log the generic system stuff
    log_system $logmaster

    # And the master specific stuff
    echo_and_eval  oc get nodes                      -o yaml               &> $logmaster/nodes
    echo_and_eval  oc get pods      --all-namespaces -o yaml  | filter_env &> $logmaster/pods
    echo_and_eval  oc get services  --all-namespaces -o yaml               &> $logmaster/services
    echo_and_eval  oc get endpoints --all-namespaces -o yaml               &> $logmaster/endpoints
    echo_and_eval  oc get routes    --all-namespaces -o yaml               &> $logmaster/aos_routes
    echo_and_eval  oc get clusternetwork             -o yaml               &> $logmaster/clusternetwork
    echo_and_eval  oc get hostsubnets                -o yaml               &> $logmaster/hostsubnets
    echo_and_eval  oc get netnamespaces              -o yaml               &> $logmaster/netnamespaces

    for node in $nodes; do
	reg_ip=$(oc get node $node --template '{{range .status.addresses}}{{if eq .type "InternalIP"}}{{.address}}{{end}}{{end}}')
	if [ -z "$reg_ip" ]; then
	    echo "Node $node: no IP address in OpenShift"
	    continue
	fi

	resolv_ip=$(getent ahostsv4 $node | awk '/STREAM/ { print $1; exit; }')

	if [ "$reg_ip" != "$resolv_ip" ]; then
	    echo "Node $node: the IP in OpenShift ($reg_ip) does not match DNS/hosts ($resolv_ip)"
	fi

	try_eval ping -c1 -W2 $node
    done

    # Outputs a list of nodes in the form "nodename IP"
    oc get nodes --template '{{range .items}}{{$name := .metadata.name}}{{range .status.addresses}}{{if eq .type "InternalIP"}}{{$name}} {{.address}}{{"\n"}}{{end}}{{end}}{{end}}' > $logdir/meta/nodeinfo

    # Outputs a list of pods in the form "minion-1 172.17.0.1 mypod namespace 10.1.0.2 e4f1d61b"
    oc get pods --all-namespaces --template '{{range .items}}{{if .status.containerStatuses}}{{if (index .status.containerStatuses 0).ready}}{{if not .spec.hostNetwork}}{{.spec.nodeName}} {{.status.hostIP}} {{.metadata.name}} {{.metadata.namespace}} {{.status.podIP}} {{printf "%.21s" (index .status.containerStatuses 0).containerID}}{{"\n"}}{{end}}{{end}}{{end}}{{end}}' | sed -e 's|docker://||' > $logdir/meta/podinfo

    # Outputs a list of services in the form "myservice namespace 172.30.0.99 tcp 5454"
    oc get services --all-namespaces --template '{{range .items}}{{if ne .spec.clusterIP "None"}}{{.metadata.name}} {{.metadata.namespace}} {{.spec.clusterIP}} {{(index .spec.ports 0).protocol}} {{(index .spec.ports 0).port}}{{"\n"}}{{end}}{{end}}' | sed -e 's/ TCP / tcp /g' -e 's/ UDP / udp /g' > $logdir/meta/serviceinfo
}

get_port_for_addr () {
    addr=$1
    # The nw_src line works with all current installs. The nw_dst line is needed for
    # older installs using the original single-tenant rules.
    sed -n -e "s/.*in_port=\([0-9]*\).*nw_src=${addr}.*/\1/p" \
	   -e "s/.*nw_dst=${addr}.*output://p" \
           $lognode/flows | head -1
}

get_vnid_for_addr () {
    addr=$1
    # On multitenant, the sed will match, and output something like "xd1", which we prefix
    # with "0" to get "0xd1". On non-multitenant, the sed won't match, and outputs nothing,
    # which we prefix with "0" to get "0". So either way, $base_pod_vnid is correct.
    echo 0$(sed -ne "s/.*reg0=0\(x[^,]*\),.*nw_dst=${addr}.*/\1/p" $lognode/flows | head -1)
}

do_pod_to_pod_connectivity_check () {
    where=$1
    namespace=$2
    base_pod_name=$3
    base_pod_addr=$4
    base_pod_pid=$5
    base_pod_port=$6
    base_pod_vnid=$7
    base_pod_ether=$8
    other_pod_name=$9
    other_pod_addr=${10}
    other_pod_nodeaddr=${11}

    echo $where pod, $namespace namespace: | tr '[a-z]' '[A-Z]'
    echo ""

    other_pod_port=$(get_port_for_addr $other_pod_addr)
    if [ -n "$other_pod_port" ]; then
	other_pod_vnid=$(get_vnid_for_addr $other_pod_addr)
	in_spec="in_port=${other_pod_port}"
    else
	case $namespace in
	    default)
		other_pod_vnid=0
		;;
	    same)
		other_pod_vnid=$base_pod_vnid
		;;
	    different)
		# VNIDs 1-10 are currently unused, so this is always different from $base_pod_vnid
		other_pod_vnid=6
		;;
	esac
	in_spec="in_port=1,tun_src=${other_pod_nodeaddr},tun_id=${other_pod_vnid}"
    fi

    echo "$base_pod_name -> $other_pod_name"
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},ip,nw_src=${base_pod_addr},nw_dst=${other_pod_addr}"
    echo ""
    echo "$other_pod_name -> $base_pod_name"
    echo_and_eval ovs-appctl ofproto/trace br0 "${in_spec},ip,nw_src=${other_pod_addr},nw_dst=${base_pod_addr},dl_dst=${base_pod_ether}"
    echo ""

    if nsenter -n -t $base_pod_pid -- ping -c 1 -W 2 $other_pod_addr  &> /dev/null; then
	echo "ping $other_pod_addr  ->  success"
    else
	echo "ping $other_pod_addr  ->  failed"
    fi

    echo ""
    echo ""
}

do_pod_external_connectivity_check () {
    base_pod_name=$1
    base_pod_addr=$2
    base_pod_pid=$3
    base_pod_port=$4
    base_pod_vnid=$5
    base_pod_ether=$6

    echo "EXTERNAL TRAFFIC:"
    echo ""
    echo "$base_pod_name -> example.com"
    # This address is from a range which is reserved for documentation examples
    # (RFC 5737) and not allowed to be used in private networks, so it should be
    # guaranteed to only match the default route.
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},ip,nw_src=${base_pod_addr},nw_dst=198.51.100.1"
    echo ""
    echo "example.com -> $base_pod_name"
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=2,ip,nw_src=198.51.100.1,nw_dst=${base_pod_addr},dl_dst=${base_pod_ether}"
    echo ""

    if nsenter -n -t $base_pod_pid -- ping -c 1 -W 2 www.redhat.com  &> /dev/null; then
	echo "ping www.redhat.com  ->  success"
    else
	echo "ping www.redhat.com  ->  failed"
    fi
}

do_pod_service_connectivity_check () {
    namespace=$1
    base_pod_name=$2
    base_pod_addr=$3
    base_pod_pid=$4
    base_pod_port=$5
    base_pod_vnid=$6
    base_pod_ether=$7
    service_name=$8
    service_addr=$9
    service_proto=${10}
    service_port=${11}

    echo service, $namespace namespace: | tr '[a-z]' '[A-Z]'
    echo ""

    echo "$base_pod_name -> $service_name"
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=${base_pod_port},reg0=${base_pod_vnid},${service_proto},nw_src=${base_pod_addr},nw_dst=${service_addr},${service_proto}_dst=${service_port}"
    echo ""
    echo "$service_name -> $base_pod_name"
    echo_and_eval ovs-appctl ofproto/trace br0 "in_port=2,${service_proto},nw_src=${service_addr},nw_dst=${base_pod_addr},dl_dst=${base_pod_ether}"
    echo ""

    # In bash, redirecting to /dev/tcp/HOST/PORT or /dev/udp/HOST/PORT opens a connection
    # to that HOST:PORT. Use this to test connectivity to the service; we can't use ping
    # like in the pod connectivity check because only connections to the correct port
    # get redirected by the iptables rules.
    if nsenter -n -t $base_pod_pid -- timeout 1 bash -c "echo -n '' > /dev/${service_proto}/${service_addr}/${service_port} 2>/dev/null"; then
	echo "connect ${service_addr}:${service_port}  ->  success"
    else
	echo "connect ${service_addr}:${service_port}  ->  failed"
    fi

    echo ""
    echo ""
}

get_config_path_from_service() {
    # 'node', 'master', 'master api', 'master controllers'
    service_type=$1
    # 'atomic-openshift-node.service', 'atomic-openshift-master-controllers.service', etc
    service_name=$2
    config=$(ps wwaux | grep -v grep | sed -ne "s/.*openshift start ${service_type} --.*config=\([^ ]*\.yaml\).*/\1/p")
    if [ -z "$config" ]; then
	config=$(systemctl show -p ExecStart $service_name | sed -ne 's/.*--config=\([^ ]*\).*/\1/p')
	if [ "$config" == "\${CONFIG_FILE}" ]; then
	    varfile=$(systemctl show $service_name | grep EnvironmentFile | sed -ne 's/EnvironmentFile=\([^ ]*\).*/\1/p')
	    if [ -f "$varfile" ]; then
	        config=$(cat $varfile | sed -ne 's/CONFIG_FILE=//p')
	    fi
	fi
    fi
    echo "$config"
}

do_node () {
    config_file=$(get_config_path_from_service "node" ${aos_node_service})
    if [ -z "$config_file" ]; then
	die "Could not find node-config.yaml from 'ps' or 'systemctl show'"
    fi
    node=$(sed -ne 's/^nodeName: //p' $config_file)
    if [ -z "$node" ]; then
	die "Could not find node name in $config_file"
    fi

    lognode=$logdir/nodes/$node
    mkdir -p $lognode

    # Grab node service logs and config file
    log_service $lognode $aos_node_service "node"

    # Log the generic system stuff
    log_system $lognode

    # Log some node-only information
    echo_and_eval  brctl show                              &> $lognode/bridges
    echo_and_eval  docker ps -a                            &> $lognode/docker-ps
    echo_and_eval  ovs-ofctl -O OpenFlow13 dump-flows br0  &> $lognode/flows
    echo_and_eval  ovs-ofctl -O OpenFlow13 show br0        &> $lognode/ovs-show
    echo_and_eval  tc qdisc show                           &> $lognode/tc-qdisc
    echo_and_eval  tc class show                           &> $lognode/tc-class
    echo_and_eval  tc filter show                          &> $lognode/tc-filter
    echo_and_eval  systemctl cat docker.service            &> $lognode/docker-unit-file
    echo_and_eval  cat `systemctl cat docker.service | grep EnvironmentFile.\*openshift-sdn | awk -F=- '{print $2}'` \
                                                           &> $lognode/docker-network-file


    # Iterate over all pods on this node, and log some data about them.
    # Remember the name, address, namespace, and pid of the first pod we find on
    # this node which is not in the default namespace
    base_pod_addr=
    while read pod_node pod_nodeaddr pod_name pod_ns pod_addr pod_id; do
	if [ "$pod_node" != "$node" ]; then
	    continue
	fi

	logpod=$lognode/pods/$pod_name
	mkdir -p $logpod

	pid=$(docker inspect -f '{{.State.Pid}}' $pod_id)
	if [ -z "$pid" ]; then
	    echo "$node:$pod_name: could not find pid of $pod"
	    continue
	fi

	echo_and_eval nsenter -n -t $pid -- ip addr  show  &> $logpod/addresses
	echo_and_eval nsenter -n -t $pid -- ip route show  &> $logpod/routes

	# If we haven't found a local pod yet, or if we have, but it's
	# in the default namespace, then make this the new base pod.
	if [ -z "$base_pod_addr" -o "$base_pod_ns" = "default" ]; then
	    base_pod_addr=$pod_addr
	    base_pod_ns=$pod_ns
	    base_pod_name=$pod_name
	    base_pod_pid=$pid
	fi
    done < $logdir/meta/podinfo

    if [ -z "$base_pod_addr" ]; then
	echo "No pods on $node, so no connectivity tests"
	return
    fi

    base_pod_port=$(get_port_for_addr $base_pod_addr)
    if [ -z "$base_pod_port" ]; then
	echo "Could not find port for ${base_pod_addr}!"
	return
    fi
    base_pod_vnid=$(get_vnid_for_addr $base_pod_addr)
    if [ -z "$base_pod_vnid" ]; then
	echo "Could not find VNID for ${base_pod_addr}!"
	return
    fi
    base_pod_ether=$(nsenter -n -t $base_pod_pid -- ip a | sed -ne "s/.*link.ether \([^ ]*\) .*/\1/p")
    if [ -z "$base_pod_ether" ]; then
	echo "Could not find MAC address for ${base_pod_addr}!"
	return
    fi

    unset did_local_default   did_local_same   did_local_different
    unset did_remote_default  did_remote_same  did_remote_different
    unset did_service_default did_service_same did_service_different
    if [ "$base_pod_ns" = "default" ]; then
	# These would be redundant with the "default" tests
	did_local_same=1
	did_remote_same=1
	did_service_same=1
    fi

    # Now find other pods of various types to test connectivity against
    touch $lognode/pod-connectivity
    while read pod_node pod_nodeaddr pod_name pod_ns pod_addr pod_id; do
	if [ "$pod_addr" = "$base_pod_addr" ]; then
	    continue
	fi

	if [ "$pod_node" = "$node" ]; then
	    where=local
	else
	    where=remote
	fi
	if [ "$pod_ns" = "default" ]; then
	    namespace=default
	elif [ "$pod_ns" = "$base_pod_ns" ]; then
	    namespace=same
	else
	    namespace=different
	fi

	if [ "$(eval echo \$did_${where}_${namespace})" = 1 ]; then
	    continue
	fi

	do_pod_to_pod_connectivity_check $where $namespace \
					 $base_pod_name $base_pod_addr \
					 $base_pod_pid $base_pod_port \
					 $base_pod_vnid $base_pod_ether \
					 $pod_name $pod_addr $pod_nodeaddr \
					 &>> $lognode/pod-connectivity
	eval did_${where}_${namespace}=1
    done < $logdir/meta/podinfo

    do_pod_external_connectivity_check $base_pod_name $base_pod_addr \
				       $base_pod_pid $base_pod_port \
				       $base_pod_vnid $base_pod_ether \
				       &>> $lognode/pod-connectivity

    # And now for services
    touch $lognode/service-connectivity
    while read service_name service_ns service_addr service_proto service_port; do
	if [ "$service_ns" = "default" ]; then
	    namespace=default
	elif [ "$service_ns" = "$base_pod_ns" ]; then
	    namespace=same
	else
	    namespace=different
	fi

	if [ "$(eval echo \$did_service_${namespace})" = 1 ]; then
	    continue
	fi

	do_pod_service_connectivity_check $namespace \
					  $base_pod_name $base_pod_addr \
					  $base_pod_pid $base_pod_port \
					  $base_pod_vnid $base_pod_ether \
					  $service_name $service_addr $service_proto $service_port \
					  &>> $lognode/service-connectivity
	eval did_service_${namespace}=1
    done < $logdir/meta/serviceinfo
}

run_self_via_ssh () {
    args=$1
    host=$2

    SSH_OPTS='-o StrictHostKeyChecking=no -o PasswordAuthentication=no'

    if ! try_eval ssh $SSH_OPTS root@$host /bin/true; then
	return 1
    fi

    if ! try_eval ssh $SSH_OPTS root@$host mkdir -m 0700 -p $logdir; then
	return 1
    fi

    if ! try_eval scp -pr $SSH_OPTS $logdir/meta root@$host:$logdir; then
	return 1
    fi

    ssh $SSH_OPTS root@$host $extra_env /bin/bash $logdir/meta/debug.sh $args
}

do_master_and_nodes ()
{
    master="$1"

    echo "Analyzing master"

    if [ -z "$master" ]; then
	do_master
    else
	if run_self_via_ssh --master $master < /dev/null; then
	    try_eval scp $SSH_OPTS -pr root@$master:$logdir/master $logdir
	else
	    return 1
	fi
    fi

    while read name addr; do
	echo ""
	echo "Analyzing $name ($addr)"

	if ip addr show | grep -q "inet $addr/"; then
	    # Running on master which is also a node
	    cp $self $logdir/debug.sh
	    /bin/bash $logdir/debug.sh --node
	else
	    run_self_via_ssh --node $addr < /dev/null && \
		try_eval scp $SSH_OPTS -pr root@$addr:$logdir/nodes $logdir
	fi
    done < $logdir/meta/nodeinfo
}

######## Main program starts here

for systemd_dir in /etc/systemd/system /usr/lib/systemd/system; do
    for name in openshift origin atomic-openshift; do
	if [ -f $systemd_dir/$name-master.service ]; then
	    aos_master_service=$name-master.service
	fi
	if [ -f $systemd_dir/$name-master-controllers.service ]; then
	    aos_master_controllers_service=$name-master-controllers.service
	fi
	if [ -f $systemd_dir/$name-master-api.service ]; then
	    aos_master_api_service=$name-master-api.service
	fi
	if [ -f $systemd_dir/$name-node.service ]; then
	    aos_node_service=$name-node.service
	fi
    done
done

case "$1" in
    --node)
	logdir=$(dirname $0 | sed -e 's|/meta$||')
	do_node
	exit 0
	;;

    --master)
	logdir=$(dirname $0 | sed -e 's|/meta$||')
	do_master
	exit 0
	;;

    "")
	if [ -z "$aos_master_service" ]; then
	    echo "Usage:"
	    echo "  [from master]"
	    echo "    $0"
	    echo "  Gathers data on the master and then connects to each node via ssh"
	    echo ""
	    echo "  [from any other machine]"
	    echo "    $0 MASTER-NAME"
	    echo "  Connects to MASTER-NAME via ssh and then connects to each node via ssh"
	    echo ""
	    echo "  The machine you run from must be able to ssh to each other machine"
	    echo "  via ssh with no password."
	    exit 1
	fi
	;;
esac

case "$0" in
    /*)
	self=$0
	;;
    *)
	self=$(pwd)/$0
	;;
esac

logdir=$(mktemp --tmpdir -d openshift-sdn-debug-XXXXXXXXX)
mkdir $logdir/meta
cp $self $logdir/meta/debug.sh
mkdir $logdir/master
mkdir $logdir/nodes
do_master_and_nodes "$1" |& tee $logdir/log

dumpname=openshift-sdn-debug-$(date --iso-8601).tgz
(cd $logdir; tar -cf - --transform='s/^\./openshift-sdn-debug/' .) | gzip -c > $dumpname
echo ""
echo "Output is in $dumpname"
