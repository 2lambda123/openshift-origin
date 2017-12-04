package controller

import (
	"fmt"
	"io/ioutil"
	"path"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/cert"
	kapi "k8s.io/kubernetes/pkg/api"
	kcontroller "k8s.io/kubernetes/pkg/controller"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

func envVars(host string, caData []byte, insecure bool, bearerTokenFile string) []kapi.EnvVar {
	envvars := []kapi.EnvVar{
		{Name: "KUBERNETES_MASTER", Value: host},
		{Name: "OPENSHIFT_MASTER", Value: host},
	}

	if len(bearerTokenFile) > 0 {
		envvars = append(envvars, kapi.EnvVar{Name: "BEARER_TOKEN_FILE", Value: bearerTokenFile})
	}

	if len(caData) > 0 {
		envvars = append(envvars, kapi.EnvVar{Name: "OPENSHIFT_CA_DATA", Value: string(caData)})
	} else if insecure {
		envvars = append(envvars, kapi.EnvVar{Name: "OPENSHIFT_INSECURE", Value: "true"})
	}

	return envvars
}

func getOpenShiftClientEnvVars(options configapi.MasterConfig) ([]kapi.EnvVar, error) {
	_, kclientConfig, err := configapi.GetInternalKubeClient(
		options.MasterClients.OpenShiftLoopbackKubeConfig,
		options.MasterClients.OpenShiftLoopbackClientConnectionOverrides,
	)
	if err != nil {
		return nil, err
	}
	return envVars(
		kclientConfig.Host,
		kclientConfig.CAData,
		kclientConfig.Insecure,
		path.Join(serviceaccountadmission.DefaultAPITokenMountPath, kapi.ServiceAccountTokenKey),
	), nil
}

// OpenshiftControllerConfig is the runtime (non-serializable) config object used to
// launch the set of openshift (not kube) controllers.
type OpenshiftControllerConfig struct {
	Initializers map[string]InitFunc

	ServiceAccountTokenControllerOptions ServiceAccountTokenControllerOptions

	ServiceAccountControllerOptions ServiceAccountControllerOptions

	BuildControllerConfig BuildControllerConfig

	DeployerControllerConfig         DeployerControllerConfig
	DeploymentConfigControllerConfig DeploymentConfigControllerConfig

	ImageTriggerControllerConfig         ImageTriggerControllerConfig
	ImageSignatureImportControllerConfig ImageSignatureImportControllerConfig
	ImageImportControllerConfig          ImageImportControllerConfig

	ServiceServingCertsControllerOptions ServiceServingCertsControllerOptions

	SDNControllerConfig       SDNControllerConfig
	UnidlingControllerConfig  UnidlingControllerConfig
	IngressIPControllerConfig IngressIPControllerConfig

	ClusterQuotaReconciliationControllerConfig ClusterQuotaReconciliationControllerConfig

	HorizontalPodAutoscalerControllerConfig HorizontalPodAutoscalerControllerConfig
}

// NewOpenShiftControllerInitializers returns a map of Openshift controller.
// If this function is called with nil config it will return the map still but
// running it this way will result in nil panics as some of the controller
// require further initialization based on config.
func NewOpenShiftControllerInitializers(config *OpenshiftControllerConfig) map[string]InitFunc {
	ret := map[string]InitFunc{}

	ret["openshift.io/serviceaccount"] = func(c ControllerContext) (bool, error) {
		return config.ServiceAccountControllerOptions.RunController(c)
	}

	ret["openshift.io/serviceaccount-pull-secrets"] = RunServiceAccountPullSecretsController
	ret["openshift.io/origin-namespace"] = RunOriginNamespaceController

	ret["openshift.io/service-serving-cert"] = func(c ControllerContext) (bool, error) {
		return config.ServiceServingCertsControllerOptions.RunController(c)
	}

	ret["openshift.io/build"] = func(c ControllerContext) (bool, error) {
		return config.BuildControllerConfig.RunController(c)
	}
	ret["openshift.io/build-config-change"] = RunBuildConfigChangeController

	ret["openshift.io/deployer"] = func(c ControllerContext) (bool, error) {
		return config.DeployerControllerConfig.RunController(c)
	}

	ret["openshift.io/deploymentconfig"] = func(c ControllerContext) (bool, error) {
		return config.DeploymentConfigControllerConfig.RunController(c)
	}

	ret["openshift.io/image-trigger"] = func(c ControllerContext) (bool, error) {
		return config.ImageTriggerControllerConfig.RunController(c)
	}

	ret["openshift.io/image-import"] = func(c ControllerContext) (bool, error) {
		return config.ImageImportControllerConfig.RunController(c)
	}

	ret["openshift.io/image-signature-import"] = func(c ControllerContext) (bool, error) {
		return config.ImageSignatureImportControllerConfig.RunController(c)
	}

	ret["openshift.io/templateinstance"] = RunTemplateInstanceController

	ret["openshift.io/sdn"] = func(c ControllerContext) (bool, error) {
		return config.SDNControllerConfig.RunController(c)
	}

	ret["openshift.io/unidling"] = func(c ControllerContext) (bool, error) {
		return config.UnidlingControllerConfig.RunController(c)
	}

	ret["openshift.io/ingress-ip"] = func(c ControllerContext) (bool, error) {
		return config.IngressIPControllerConfig.RunController(c)
	}

	ret["openshift.io/resourcequota"] = RunResourceQuotaManager

	ret["openshift.io/cluster-quota-reconciliation"] = func(c ControllerContext) (bool, error) {
		return config.ClusterQuotaReconciliationControllerConfig.RunController(c)
	}

	// overrides the Kube HPA controller config, so that we can point it at an HTTPS Heapster
	// in openshift-infra, and pass it a scale client that knows how to scale DCs
	ret["openshift.io/horizontalpodautoscaling"] = func(c ControllerContext) (bool, error) {
		return config.HorizontalPodAutoscalerControllerConfig.RunController(c)
	}

	return ret
}

func (c *OpenshiftControllerConfig) GetControllerInitializers() (map[string]InitFunc, error) {
	return NewOpenShiftControllerInitializers(c), nil
}

// NewOpenShiftControllerPreStartInitializers returns list of initializers for controllers
// that needed to be run before any other controller is started.
// Typically this has to done for the serviceaccount-token controller as it provides
// tokens to other controllers.
func (c *OpenshiftControllerConfig) ServiceAccountContentControllerInit() InitFunc {
	return c.ServiceAccountTokenControllerOptions.RunController
}

func BuildOpenshiftControllerConfig(options configapi.MasterConfig) (*OpenshiftControllerConfig, error) {
	var err error
	ret := &OpenshiftControllerConfig{}

	_, loopbackClientConfig, err := configapi.GetInternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}

	ret.ServiceAccountTokenControllerOptions.RootClientBuilder = kcontroller.SimpleControllerClientBuilder{
		ClientConfig: loopbackClientConfig,
	}

	if len(options.ServiceAccountConfig.PrivateKeyFile) > 0 {
		ret.ServiceAccountTokenControllerOptions.PrivateKey, err = cert.PrivateKeyFromFile(options.ServiceAccountConfig.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("error reading signing key for Service Account Token Manager: %v", err)
		}
	}
	if len(options.ServiceAccountConfig.MasterCA) > 0 {
		ret.ServiceAccountTokenControllerOptions.RootCA, err = ioutil.ReadFile(options.ServiceAccountConfig.MasterCA)
		if err != nil {
			return nil, fmt.Errorf("error reading master ca file for Service Account Token Manager: %s: %v", options.ServiceAccountConfig.MasterCA, err)
		}
		if _, err := cert.ParseCertsPEM(ret.ServiceAccountTokenControllerOptions.RootCA); err != nil {
			return nil, fmt.Errorf("error parsing master ca file for Service Account Token Manager: %s: %v", options.ServiceAccountConfig.MasterCA, err)
		}
	}
	if options.ControllerConfig.ServiceServingCert.Signer != nil && len(options.ControllerConfig.ServiceServingCert.Signer.CertFile) > 0 {
		certFile := options.ControllerConfig.ServiceServingCert.Signer.CertFile
		serviceServingCA, err := ioutil.ReadFile(certFile)
		if err != nil {
			return nil, fmt.Errorf("error reading ca file for Service Serving Certificate Signer: %s: %v", certFile, err)
		}
		if _, err := crypto.CertsFromPEM(serviceServingCA); err != nil {
			return nil, fmt.Errorf("error parsing ca file for Service Serving Certificate Signer: %s: %v", certFile, err)
		}

		// if we have a rootCA bundle add that too.  The rootCA will be used when hitting the default master service, since those are signed
		// using a different CA by default.  The rootCA's key is more closely guarded than ours and if it is compromised, that power could
		// be used to change the trusted signers for every pod anyway, so we're already effectively trusting it.
		if len(ret.ServiceAccountTokenControllerOptions.RootCA) > 0 {
			ret.ServiceAccountTokenControllerOptions.ServiceServingCA = append(ret.ServiceAccountTokenControllerOptions.ServiceServingCA, ret.ServiceAccountTokenControllerOptions.RootCA...)
			ret.ServiceAccountTokenControllerOptions.ServiceServingCA = append(ret.ServiceAccountTokenControllerOptions.ServiceServingCA, []byte("\n")...)
		}
		ret.ServiceAccountTokenControllerOptions.ServiceServingCA = append(ret.ServiceAccountTokenControllerOptions.ServiceServingCA, serviceServingCA...)
	}

	ret.ServiceAccountControllerOptions.ManagedNames = options.ServiceAccountConfig.ManagedNames

	storageVersion := options.EtcdStorageConfig.OpenShiftStorageVersion
	groupVersion := schema.GroupVersion{Group: "", Version: storageVersion}
	annotationCodec := kapi.Codecs.LegacyCodec(groupVersion)

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	ret.BuildControllerConfig = BuildControllerConfig{
		DockerImage:           imageTemplate.ExpandOrDie("docker-builder"),
		S2IImage:              imageTemplate.ExpandOrDie("sti-builder"),
		AdmissionPluginConfig: options.AdmissionConfig.PluginConfig,
		Codec: annotationCodec,
	}

	vars, err := getOpenShiftClientEnvVars(options)
	if err != nil {
		return nil, err
	}
	ret.DeployerControllerConfig = DeployerControllerConfig{
		ImageName:     imageTemplate.ExpandOrDie("deployer"),
		Codec:         annotationCodec,
		ClientEnvVars: vars,
	}
	ret.DeploymentConfigControllerConfig = DeploymentConfigControllerConfig{
		Codec: annotationCodec,
	}

	ret.ImageTriggerControllerConfig = ImageTriggerControllerConfig{
		HasBuilderEnabled: options.DisabledFeatures.Has(configapi.FeatureBuilder),
		// TODO: make these consts in configapi
		HasDeploymentsEnabled:  options.DisabledFeatures.Has("triggers.image.openshift.io/deployments"),
		HasDaemonSetsEnabled:   options.DisabledFeatures.Has("triggers.image.openshift.io/daemonsets"),
		HasStatefulSetsEnabled: options.DisabledFeatures.Has("triggers.image.openshift.io/statefulsets"),
		HasCronJobsEnabled:     options.DisabledFeatures.Has("triggers.image.openshift.io/cronjobs"),
	}
	ret.ImageImportControllerConfig = ImageImportControllerConfig{
		MaxScheduledImageImportsPerMinute:          options.ImagePolicyConfig.MaxScheduledImageImportsPerMinute,
		ResyncPeriod:                               10 * time.Minute,
		DisableScheduledImport:                     options.ImagePolicyConfig.DisableScheduledImport,
		ScheduledImageImportMinimumIntervalSeconds: options.ImagePolicyConfig.ScheduledImageImportMinimumIntervalSeconds,
	}
	ret.ImageSignatureImportControllerConfig = ImageSignatureImportControllerConfig{
		ResyncPeriod:          1 * time.Hour,
		SignatureFetchTimeout: 1 * time.Minute,
		SignatureImportLimit:  3,
	}

	ret.ServiceServingCertsControllerOptions = ServiceServingCertsControllerOptions{
		Signer: options.ControllerConfig.ServiceServingCert.Signer,
	}

	ret.SDNControllerConfig = SDNControllerConfig{
		NetworkConfig: options.NetworkConfig,
	}
	ret.UnidlingControllerConfig = UnidlingControllerConfig{
		ResyncPeriod: 2 * time.Hour,
	}
	ret.IngressIPControllerConfig = IngressIPControllerConfig{
		IngressIPSyncPeriod:  10 * time.Minute,
		IngressIPNetworkCIDR: options.NetworkConfig.IngressIPNetworkCIDR,
	}

	ret.ClusterQuotaReconciliationControllerConfig = ClusterQuotaReconciliationControllerConfig{
		DefaultResyncPeriod:            5 * time.Minute,
		DefaultReplenishmentSyncPeriod: 12 * time.Hour,
	}

	// TODO this goes away with a truly generic autoscaler
	ret.HorizontalPodAutoscalerControllerConfig = HorizontalPodAutoscalerControllerConfig{
		HeapsterNamespace: options.PolicyConfig.OpenShiftInfrastructureNamespace,
	}

	return ret, nil
}
