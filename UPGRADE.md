# Upgrading

This document describes future changes that will affect your current resources used
inside of OpenShift. Each change contains description of the change and information
when that change will happen.


## Origin 1.0.x / OSE 3.0.x

1. Currently all build pods have a label named `build`. This label is being deprecated
  in favor of `openshift.io/build.name` in Origin 1.0.x (OSE 3.0.x) - both are supported.
  In Origin 1.1 we will only set the new label and remove support for the old label.
  See #3502.

1. Currently `oc exec` will attempt to `POST` to `pods/podname/exec`, if that fails it will
  fallback to a `GET` to match older policy roles.  In Origin 1.1 (OSE 3.1) the support for the
  old `oc exec` endpoint via `GET` will be removed.

1. The `pauseControllers` field in `master-config.yaml` is deprecated as of Origin 1.0.4 and will
  no longer be supported in Origin 1.1. After that, a warning will be printed on startup if it
  is set to true.

1. The `/ns/namespace-name/subjectaccessreview` endpoint is deprecated, use `/subjectaccessreview` 
(with the `namespace` field set) or `/ns/namespace-name/localsubjectaccessreview`.  In 
Origin 1.y / OSE 3.y, support for `/ns/namespace-name/subjectaccessreview` wil be removed.
At that time, the openshift docker registry image must be upgraded in order to continue functioning.

1. The `deploymentConfig.rollingParams.updatePercent` field is deprecated in
  favor of `deploymentConfig.rollingParams.maxUnavailable` and
  `deploymentConfig.rollingParams.maxSurge`. The `updatePercent` field will be
  removed  in Origin 1.1 (OSE 3.1).

1. The `volume.metadata` field is deprecated as of Origin 1.0.6 in favor of `volume.downwardAPI`.