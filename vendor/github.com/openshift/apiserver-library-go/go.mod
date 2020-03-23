module github.com/openshift/apiserver-library-go

go 1.13

require (
	github.com/hashicorp/golang-lru v0.5.1
	github.com/openshift/api v0.0.0-20200210091934-a0e53e94816b
	github.com/openshift/build-machinery-go v0.0.0-20200211121458-5e3d6e570160
	github.com/openshift/client-go v0.0.0-20200116152001-92a2713fa240
	github.com/openshift/library-go v0.0.0-20200120084036-bb27e57e2f2b
	go.uber.org/atomic v1.3.3-0.20181018215023-8dc6146f7569 // indirect
	go.uber.org/multierr v1.1.1-0.20180122172545-ddea229ff1df // indirect
	k8s.io/api v0.18.0-beta.2
	k8s.io/apimachinery v0.18.0-beta.2
	k8s.io/apiserver v0.18.0-beta.2
	k8s.io/client-go v0.18.0-beta.2
	k8s.io/code-generator v0.18.0-beta.2
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.18.0-beta.2
)

replace (
	// library-go and client-go prebase-1.18.beta2
	github.com/openshift/api => github.com/openshift/api v0.0.0-20200311151921-fdf269f98861
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20200311173916-2981b842ff3e
	github.com/openshift/library-go => github.com/openshift/library-go v0.0.0-20200312205901-580d5a0dcaf3
	k8s.io/api => k8s.io/api v0.18.0-beta.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.0-beta.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.0-beta.2
	k8s.io/apiserver => k8s.io/apiserver v0.18.0-beta.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.0-beta.2
	k8s.io/client-go => k8s.io/client-go v0.18.0-beta.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.0-beta.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.0-beta.2
	k8s.io/code-generator => k8s.io/code-generator v0.18.0-beta.2
	k8s.io/component-base => k8s.io/component-base v0.18.0-beta.2
	k8s.io/cri-api => k8s.io/cri-api v0.18.0-beta.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.0-beta.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.0-beta.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.0-beta.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.0-beta.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.0-beta.2
	k8s.io/kubectl => k8s.io/kubectl v0.18.0-beta.2
	k8s.io/kubelet => k8s.io/kubelet v0.18.0-beta.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.0-beta.2
	k8s.io/metrics => k8s.io/metrics v0.18.0-beta.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.0-beta.2
)
