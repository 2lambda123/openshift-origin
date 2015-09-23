MySQL + Ceph Persistent Volume
==============================

Here are examples showing how to run MySQL in the following environments:
1. in a container launched directly from docker (see the [mysql readme](MYSQL.md))
2. via an openshift/kubernetes pod (examples [2](mysql_ceph_host) and [3](mysql_ceph_pvc))
3. via an openshift application template (example [4](mysql_ceph_template)).

Both [local OSE-node storage](mysql_ceph_host) and [ceph-rbd block storage](mysql_ceph_plugin) under an ext4 file system to persist the database.

The next few sections are common across almost all of the examples and are also shown below:

### Environment:
The basic enviromnent used for all of the examples is described [here](ENV.md). It is assumed that ceph is already up and running, either on bare metal, in a VM, or containerized.

### Setting up Openshift Enterprise (OSE):
The steps needed to setup a simple OSE cluster with 1 master and 1 worker node are described [here](OSE.md).

### Setting up MySQL:
Follow the instructions [here](MYSQL.md) to initialize and validate containerized mysql.

### Specific Examples:
1. [mysql + local/host storage](mysql_ceph_host) - mysql database lives on the OSE host where the pod is scheduled
2. [mysql + ceph plugin](mysql_ceph_plugin) - mysql database resides in ceph, a rbd plugin is specfied
3. [mysql + ceph + pvc](mysql_ceph_pvc) - mysql database resides in ceph, a Persistent Volume (PV) and Persistent Volume Claim (PVC) are used
4. [mysql + ceph + template](mysql_ceph_template) -- same as the above example except the pod and pvc are defined in a single template file

