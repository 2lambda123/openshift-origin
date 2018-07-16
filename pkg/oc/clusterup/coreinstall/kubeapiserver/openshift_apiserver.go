package kubeapiserver

import (
	"io/ioutil"
	"path"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/golang/glog"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/tmpformac"
)

func MakeOpenShiftAPIServerConfig(existingMasterConfig string, routingSuffix, basedir string) (string, error) {
	configDir := path.Join(basedir, OpenShiftAPIServerDirName)
	glog.V(1).Infof("Copying kube-apiserver config to local directory %s", configDir)
	if err := tmpformac.CopyDirectory(existingMasterConfig, configDir); err != nil {
		return "", err
	}

	// update some listen information to include starting the DNS server
	masterconfigFilename := path.Join(configDir, "master-config.yaml")
	originalBytes, err := ioutil.ReadFile(masterconfigFilename)
	if err != nil {
		return "", err
	}
	configObj, err := runtime.Decode(configapilatest.Codec, originalBytes)
	if err != nil {
		return "", err
	}
	masterconfig := configObj.(*configapi.MasterConfig)
	masterconfig.ServingInfo.BindAddress = "0.0.0.0:8445"

	// hardcode the route suffix to the old default.  If anyone wants to change it, they can modify their config.
	masterconfig.RoutingConfig.Subdomain = routingSuffix

	// use the generated service serving cert
	masterconfig.ServingInfo.ServerCert.CertFile = "/var/serving-cert/tls.crt"
	masterconfig.ServingInfo.ServerCert.KeyFile = "/var/serving-cert/tls.key"

	// default openshift image policy admission
	if masterconfig.AdmissionConfig.PluginConfig == nil {
		masterconfig.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{}
	}

	// Add default ImagePolicyConfig into openshift api master config
	policyConfig := runtime.Unknown{Raw: []byte(`{"kind":"ImagePolicyConfig","apiVersion":"v1","executionRules":[{"name":"execution-denied",
"onResources":[{"resource":"pods"},{"resource":"builds"}],"reject":true,"matchImageAnnotations":[{"key":"images.openshift.io/deny-execution",
"value":"true"}],"skipOnResolutionFailure":true}]}`)}
	masterconfig.AdmissionConfig.PluginConfig["openshift.io/ImagePolicy"] = &configapi.AdmissionPluginConfig{Configuration: &policyConfig}

	configBytes, err := configapilatest.WriteYAML(masterconfig)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(masterconfigFilename, configBytes, 0644); err != nil {
		return "", err
	}

	return configDir, nil
}
