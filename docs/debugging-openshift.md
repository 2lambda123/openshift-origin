Troubleshooting
=================

This document contains some tips and suggestions for troubleshooting an OpenShift v3 deployment.

System Environment
------------------

1. Run as root

   Currently OpenShift v3 must be started as root in order to manipulate your iptables configuration.  The openshift commands (e.g. `openshift kubectl apply`) do not need to be run as root.

1. Properly configure or disable firewalld

   On Fedora or other distributions using firewalld: Add docker0 to the public zone

        $ firewall-cmd --zone=trusted --change-interface=docker0
        $ systemctl restart firewalld

    Alternatively you can disable it via:
    
        $ systemctl stop firewalld
        
1. Disable selinux  

    Eventually this will not be necessary but we are currently focused on features and will be revisiting selinux policies in the future.

        $ setenforce 0
        

Build Failures
--------------

To investigate a build failure, first check the build logs.  You can view the build logs via

    $ openshift kubectl build-logs [build_id]
        
and you can get the build id via:

    $ openshift kubectl get builds

the build id is in the first column.

If you're unable to retrieve the logs in this way, you can also get them directly from docker.  First you need to find the docker container that ran your build:

    $ docker ps -a | grep builder

The most recent container in that list should be the one that ran your build.  The container id is the first column.  You can then run:

    $ docker logs [container id]
        
Hopefully the logs will provide some indication of what it failed (e.g. failure to find the source repository, an actual build issue, failure to push the resulting image to the docker registry, etc).

Docker Registry
---------------

Most of the v3 flows today assume you are running a docker registry pod.  You should ensure that this local registry is running:

    $ openshift kubectl get services | grep registry

If it's not running, you can launch it via:

    $ openshift kubectl apply -f examples/sample-app/docker-registry-config.json

In addition, confirm the IP and Port reported in the services list matches the registry references in your configuration json (e.g. any image tags that contain a registry hostname).  

Probing Containers
------------------

In general you may want to investigate a particular container.  You can either gather the logs from a container via `docker logs [container id]` or use `docker exec -it [container id] /bin/sh` to enter the container's namespace and poke around.


Benign Errors/Messages
----------------------

There are a number of suspicious looking messages that appear in the openshift log output which can normally be ignored:

1. Failed to find an IP for pod (benign as long as it does not continuously repeat)

        E1125 14:51:49.665095 04523 endpoints_controller.go:74] Failed to find an IP for pod: {{ } {7e5769d2-74dc-11e4-bc62-3c970e3bf0b7 default /api/v1beta1/pods/7e5769d2-74dc-11e4-bc62-3c970e3bf0b7  41 2014-11-25 14:51:48 -0500 EST map[template:ruby-helloworld-sample deployment:database-1 deploymentconfig:database name:database] map[]} {{v1beta1 7e5769d2-74dc-11e4-bc62-3c970e3bf0b7 7e5769d2-74dc-11e4-bc62-3c970e3bf0b7 [] [{ruby-helloworld-database mysql []  [{ 0 3306 TCP }] [{MYSQL_ROOT_PASSWORD rrKAcyW6} {MYSQL_DATABASE root}] 0 0 [] <nil> <nil>  false }] {0x1654910 <nil> <nil>}} Running localhost.localdomain   map[]} {{   [] [] {<nil> <nil> <nil>}} Pending localhost.localdomain   map[]} map[]}

1. Proxy connection reset 

        E1125 14:52:36.605423 04523 proxier.go:131] I/O error: read tcp 10.192.208.170:57472: connection reset by peer

1. No network settings

        W1125 14:53:10.035539 04523 rest.go:231] No network settings: api.ContainerStatus{State:api.ContainerState{Waiting:(*api.ContainerStateWaiting)(0xc208b29b40), Running:(*api.ContainerStateRunning)(nil), Termination:(*api.ContainerStateTerminated)(nil)}, RestartCount:0, PodIP:"", Image:"kubernetes/pause:latest"}

Must Gather
-----------
If you find yourself still stuck, before seeking help in #openshift on freenode.net, please recreate your issue with verbose logging and gather the following:

1. OpenShift logs at level 4 (verbose logging):

        $ openshift start --loglevel=4 &> /tmp/openshift.log
        
1. Container logs  
    
    The following bit of scripting will pull logs for **all** containers that have been run on your system.  This might be excessive if you don't keep a clean history, so consider manually grabbing logs for the relevant containers instead:

        for container in $(docker ps -aq); do
            docker logs $container >& $LOG_DIR/container-$container.log
        done
