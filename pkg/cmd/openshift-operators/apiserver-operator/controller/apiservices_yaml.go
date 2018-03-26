package controller

const apiserviceListYaml = `apiVersion: v1
kind: List
items:
- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.apps.openshift.io
  spec:
    group: apps.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.authorization.openshift.io
  spec:
    group: authorization.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.build.openshift.io
  spec:
    group: build.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.image.openshift.io
  spec:
    group: image.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.network.openshift.io
  spec:
    group: network.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.oauth.openshift.io
  spec:
    group: oauth.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.project.openshift.io
  spec:
    group: project.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.quota.openshift.io
  spec:
    group: quota.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.route.openshift.io
  spec:
    group: route.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.security.openshift.io
  spec:
    group: security.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.template.openshift.io
  spec:
    group: template.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15

- apiVersion: apiregistration.k8s.io/v1beta1
  kind: APIService
  metadata:
    name: v1.user.openshift.io
  spec:
    group: user.openshift.io
    version: v1
    service:
      namespace: openshift-apiserver
      name: api
    insecureSkipTLSVerify: true
    groupPriorityMinimum: 9900
    versionPriority: 15
`
