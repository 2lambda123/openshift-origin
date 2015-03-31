OpenShift Application Platform
==============================

[![GoDoc](https://godoc.org/github.com/openshift/origin?status.png)](https://godoc.org/github.com/openshift/origin)
[![Travis](https://travis-ci.org/openshift/origin.svg?branch=master)](https://travis-ci.org/openshift/origin)

This is the source repository for [OpenShift 3](https://openshift.github.io), based on top of [Docker](https://www.docker.io) containers and the
[Kubernetes](https://github.com/GoogleCloudPlatform/kubernetes) container cluster manager.
OpenShift adds developer and operational centric tools on top of Kubernetes to enable rapid application development,
easy deployment and scaling, and long-term lifecycle maintenance for small and large teams and applications.

**Features:**

* Push source code to the platform and have deployments automatically occur
* Easy to use client tools for building web applications from source code
  * Templatize the components of your system, reuse them, and iteratively deploy them over time
* Centralized administration and management of application component libraries
  * Roll out changes to software stacks to your entire organization in a controlled fashion
* Team and user isolation of containers, builds, and network communication in an easy multi-tenancy system
  * Limit, track, and manage the resources teams are using

**Learn More:**

* **[Technical Architecture Presentation](https://docs.google.com/presentation/d/1Isp5UeQZTo3gh6e59FMYmMs_V9QIQeBelmbyHIJ1H_g/pub?start=false&loop=false&delayms=3000)**
* **[System Architecture](https://github.com/openshift/openshift-pep/blob/master/openshift-pep-013-openshift-3.md)** design document
* The **[Trello Roadmap](https://ci.openshift.redhat.com/roadmap_overview.html)** covers the epics and stories being worked on (click through to individual items)
* **[Public Documentation](http://docs.openshift.org/latest/welcome/index.html)** site

For questions or feedback, reach us on [IRC on #openshift-dev](https://botbot.me/freenode/openshift-dev/) on Freenode or post to our [mailing list](https://lists.openshift.redhat.com/openshiftmm/listinfo/dev).

NOTE: OpenShift is in alpha and is not yet intended for production use. However we welcome feedback, suggestions, and testing as we approach our first beta.


Security Warning!!!
-------------------
OpenShift is a system which runs Docker containers on your machine.  In some cases (build operations and the registry service) it does so using privileged containers.  Those containers access your host's Docker daemon and perform `docker build` and `docker push` operations.  As such, you should be aware of the inherent security risks associated with performing `docker run` operations on arbitrary images as they have effective root access.  This is particularly relevant when running the OpenShift as a node directly on your laptop or primary workstation.  Only run code you trust.

For more information on the security of containers, see these articles:

* http://opensource.com/business/14/7/docker-security-selinux
* https://docs.docker.com/articles/security/

Running untrusted containers will become less scary as improvements are made upstream to Docker and Kubernetes, but until then please be conscious of the images you run.  Consider using images from trusted parties, building them yourself on OpenShift, or only running containers that run as non-root users.


Getting Started
---------------
The simplest way to run OpenShift Origin is in a Docker container:

    $ docker run -d --name "openshift-origin" --net=host --privileged \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v /tmp/openshift:/tmp/openshift \
        openshift/origin start

(you'll need to create the /tmp/openshift directory the first time).

Once the container is started, you can jump into a console inside the container and run the CLI.

    $ docker exec -it openshift-origin bash
    $ osc --help

If you just want to experiment with the API without worrying about security privileges, you can disable authorization checks by running this from the host system.  This command grants full access to anyone.

    $ docker exec -it openshift-origin bash -c "openshift admin --config=/var/lib/openshift/openshift.local.certificates/admin/.kubeconfig policy add-role-to-group cluster-admin system:authenticated system:unauthenticated"


### Start Developing

You can develop [locally on your host](CONTRIBUTING.adoc#develop-locally-on-your-host) or with a [virtual machine](CONTRIBUTING.adoc#develop-on-virtual-machine-using-vagrant), or if you want to just try out OpenShift [download the latest Linux server, or Windows and Mac OS X client pre-built binaries](CONTRIBUTING.adoc#download-from-github).

First, **get up and running with the** [**Contributing Guide**](CONTRIBUTING.adoc).

Once setup with a Go development environment and Docker, you can:

1.  Build the source code

        $ make clean build

2.  Start the OpenShift server

        $ make run

3.  In another terminal window, switch to the directory and start an app:

        $ cd $GOPATH/src/github.com/openshift/origin
        $ export KUBECONFIG=`pwd`/openshift.local.certificates/admin/.kubeconfig 
        $ _output/local/go/bin/osc create -f examples/hello-openshift/hello-pod.json

In your browser, go to [http://localhost:6061](http://localhost:6061) and you should see 'Welcome to OpenShift'.


### What's Just Happened?

The example above starts the ['openshift/hello-openshift' Docker image](https://github.com/openshift/origin/blob/master/examples/hello-openshift/hello-pod.json#L11) inside a Docker container, but managed by OpenShift and Kubernetes.

* At the Docker level, that image [listens on port 8080](https://github.com/openshift/origin/blob/master/examples/hello-openshift/hello_openshift.go#L16) within a container and [prints out a simple 'Hello OpenShift' message on access](https://github.com/openshift/origin/blob/master/examples/hello-openshift/hello_openshift.go#L9).
* At the Kubernetes level, we [map that bound port in the container](https://github.com/openshift/origin/blob/master/examples/hello-openshift/hello-pod.json#L13) [to port 6061 on the host](https://github.com/openshift/origin/blob/master/examples/hello-openshift/hello-pod.json#L14) so that we can access it via the host browser.
* When you created the container, Kubernetes decided which host to place the container on by looking at the available hosts and selecting one with available space.  The agent that runs on each node (part of the OpenShift all-in-one binary, called the Kubelet) saw that it was now supposed to run the container and instructed Docker to start the container.

OpenShift brings all of these pieces (and the client) together in a single, easy to use binary.  The following examples show the other OpenShift specific features that live above the Kubernetes runtime like image building and deployment flows.


### Next Steps

We highly recommend trying out the [OpenShift walkthrough](https://github.com/openshift/origin/blob/master/examples/sample-app/README.md), which shows some of the lower level pieces of of OpenShift that will be the foundation for user applications.  The walkthrough is accompanied by a blog series on [blog.openshift.com](https://blog.openshift.com/openshift-v3-deep-dive-docker-kubernetes/) that goes into more detail.  It's a great place to start, albeit at a lower level than OpenShift 2.

Both OpenShift and Kubernetes have a strong focus on documentation - see the following for more information about them:

* [OpenShift Documentation](http://docs.openshift.org/latest/welcome/index.html)
* [Kubernetes Getting Started](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/README.md)
* [Kubernetes Documentation](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/README.md)

You can see some other examples of using Kubernetes at a lower level - stay tuned for more high level OpenShift examples as well:

* [Kubernetes walkthrough](https://github.com/GoogleCloudPlatform/kubernetes/tree/master/examples/walkthrough)
* [Kubernetes guestbook](https://github.com/GoogleCloudPlatform/kubernetes/tree/master/examples/guestbook)

### Troubleshooting

If you run into difficulties running OpenShift, start by reading through the [troubleshooting guide](https://github.com/openshift/origin/blob/master/docs/debugging-openshift.md).


API
---

The OpenShift APIs are exposed at `https://localhost:8443/osapi/v1beta1/*`.

* Builds
 * `https://localhost:8443/osapi/v1beta1/builds`
 * `https://localhost:8443/osapi/v1beta1/buildConfigs`
 * `https://localhost:8443/osapi/v1beta1/buildLogs`
 * `https://localhost:8443/osapi/v1beta1/buildConfigHooks`
* Deployments
 * `https://localhost:8443/osapi/v1beta1/deployments`
 * `https://localhost:8443/osapi/v1beta1/deploymentConfigs`
* Images
 * `https://localhost:8443/osapi/v1beta1/images`
 * `https://localhost:8443/osapi/v1beta1/imageRepositories`
 * `https://localhost:8443/osapi/v1beta1/imageRepositoryMappings`
* Templates
 * `https://localhost:8443/osapi/v1beta1/templateConfigs`
* Routes
 * `https://localhost:8443/osapi/v1beta1/routes`
* Projects
 * `https://localhost:8443/osapi/v1beta1/projects`
* Users
 * `https://localhost:8443/osapi/v1beta1/users`
 * `https://localhost:8443/osapi/v1beta1/userIdentityMappings`
* OAuth
 * `https://localhost:8443/osapi/v1beta1/accessTokens`
 * `https://localhost:8443/osapi/v1beta1/authorizeTokens`
 * `https://localhost:8443/osapi/v1beta1/clients`
 * `https://localhost:8443/osapi/v1beta1/clientAuthorizations`

The Kubernetes APIs are exposed at `https://localhost:8443/api/v1beta1/*`:

* `https://localhost:8443/api/v1beta1/pods`
* `https://localhost:8443/api/v1beta1/services`
* `https://localhost:8443/api/v1beta1/replicationControllers`
* `https://localhost:8443/api/v1beta1/operations`

OpenShift and Kubernetes integrate with the [Swagger 2.0 API framework](http://swagger.io) which aims to make it easier to document and write clients for RESTful APIs.  When you start OpenShift, the Swagger API endpoint is exposed at `https://localhost:8443/swaggerapi`. The Swagger UI makes it easy to view your documentation - to view the docs for your local version of OpenShift start the server with CORS enabled:

    $ openshift start --cors-allowed-origins=.*

and then browse to http://openshift3swagger-claytondev.rhcloud.com (which runs a copy of the Swagger UI that points to localhost:8080 by default).  Expand the operations available on v1beta1 to see the schemas (and to try the API directly).

Management Console
------------------

The OpenShift API server also hosts the web-based management console. You can try out the management console at [http://localhost:8443/console](http://localhost:8443/console).

For more information on the console [checkout the README](assets/README.md) and the [docs](http://docs.openshift.org/latest/using_openshift/console.html).

![Management console overview](docs/screenshots/console_overview.png?raw=true)

FAQ
---

1. How does OpenShift relate to Kubernetes?

    OpenShift embeds Kubernetes and adds additional functionality to offer a simple, powerful, and
    easy-to-approach developer and operator experience for building applications in containers.
    Kubernetes today is focused around composing containerized applications - OpenShift adds
    building images, managing them, and integrating them into deployment flows.  Our goal is to do
    most of that work upstream, with integration and final packaging occurring in OpenShift.  As we
    iterate through the next few months, you'll see this repository focus more on integration and
    plugins, with more and more features becoming part of Kubernetes.

2. What about [geard](https://github.com/openshift/geard)?

    Geard started as a prototype vehicle for the next generation of the OpenShift node - as an
    orchestration endpoint, to offer integration with systemd, and to prototype network abstraction,
    routing, SSH access to containers, and Git hosting.  Its intended goal is to provide a simple
    way of reliably managing containers at scale, and to offer administrators tools for easily
    composing those applications (gear deploy).

    With the introduction of Kubernetes, the Kubelet, and the pull model it leverages from etcd, we
    believe we can implement the pull-orchestration model described in
    [orchestrating geard](https://github.com/openshift/geard/blob/master/docs/orchestrating_geard.md),
    especially now that we have a path to properly
    [limit host compromises from affecting the cluster](https://github.com/GoogleCloudPlatform/kubernetes/pull/860).  
    The pull-model has many advantages for end clients, not least of which that they are guaranteed
    to eventually converge to the correct state of the server. We expect that the use cases the geard
    endpoint offered will be merged into the Kubelet for consumption by admins.

    systemd and Docker integration offers efficient and clean process management and secure logging
    aggregation with the system.  We plan on introducing those capabilities into Kubernetes over
    time, especially as we work with the Docker upstream to limit the impact of the Docker daemon's
    parent child process relationship with containers, where death of the Docker daemon terminates
    the containers under it

    Network links and their ability to simplify how software connects to other containers is planned
    for Docker links v2 and is a capability we believe will be important in Kubernetes as well ([see issue 494 for more details](https://github.com/GoogleCloudPlatform/kubernetes/issues/494)).

    The geard deployment descriptor describes containers and their relationships and will be mapped
    to deployment on top of Kubernetes.  The geard commandline itself will likely be merged directly
    into the `openshift` command for all-in-one management of a cluster.

3. What can I run on OpenShift?

    OpenShift is designed to run any existing Docker images.  In addition you can define builds that will produce new Docker images from a Dockerfile.  However the real magic of OpenShift can be seen when using [Source-To-Image](https://github.com/openshift/source-to-image)(STI) builds which allow you to simply supply an application source repository which will be combined with an existing STI-enabled Docker image to produce a new runnable image that runs your application.  We are continuing to grow the ecosystem of STI-enabled images and documenting them [here](https://ci.openshift.redhat.com/openshift-docs-master-testing/latest/openshift_sti_images/overview.html).  We also have a few more experimental images available:

    * [Wildfly](https://github.com/openshift/wildfly-8-centos)

Contributing
------------

All contributions are welcome - OpenShift uses the Apache 2 license and does not require any contributor agreement to submit patches.  Please open issues for any bugs or problems you encounter, ask questions on the OpenShift IRC channel (#openshift-dev on freenode), or get involved in the [Kubernetes project](https://github.com/GoogleCloudPlatform/kubernetes) at the container runtime layer.

See [HACKING.md](https://github.com/openshift/origin/blob/master/HACKING.md) for more details on developing on OpenShift including how different tests are setup.

If you want to run the test suite, make sure you have your environment from above set up, and from the origin directory run:

```
# run the unit tests
$ make check

# run a simple server integration test
$ hack/test-cmd.sh

# run the integration server test suite
$ hack/test-integration.sh

# run the end-to-end test suite
$ hack/test-end-to-end.sh

# run all of the tests above
$ make test
```

You'll need [etcd](https://github.com/coreos/etcd) installed and on your path for the integration and end-to-end tests to run, and Docker must be installed to run the end-to-end tests.  To install etcd you should be able to run:

```
$ hack/install-etcd.sh
```

Some of the components of OpenShift run as Docker images, including the builders and deployment tools in `images/builder/docker/*` and 'images/deploy/*`.  To build them locally run

```
$ hack/build-images.sh
```


License
-------

OpenShift is licensed under the [Apache License, Version 2.0](http://www.apache.org/licenses/).
