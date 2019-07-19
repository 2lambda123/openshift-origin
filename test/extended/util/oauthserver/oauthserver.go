package oauthserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"path"
	"time"

	"github.com/RangelReale/osincli"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"

	configv1 "github.com/openshift/api/config/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/oc/pkg/helpers/tokencmd"
	"github.com/openshift/origin/test/extended/testdata"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	serviceURLFmt = "https://test-oauth-svc.%s.svc" // fill in the namespace

	servingCertDirPath  = "/var/config/system/secrets/serving-cert"
	servingCertPathCert = "/var/config/system/secrets/serving-cert/tls.crt"
	servingCertPathKey  = "/var/config/system/secrets/serving-cert/tls.key"

	routerCertsDirPath = "/var/config/system/secrets/router-certs"

	sessionSecretDirPath = "/var/config/system/secrets/session-secret"
	sessionSecretPath    = "/var/config/system/secrets/session-secret/session"

	oauthConfigPath  = "/var/config/system/configmaps/oauth-config"
	serviceCADirPath = "/var/config/system/configmaps/service-ca"

	configObjectsDir = "/var/oauth/configobjects/"

	RouteName = "test-oauth-route"
	SAName    = "e2e-oauth"
)

var (
	serviceCAPath = "/var/config/system/configmaps/service-ca/service-ca.crt" // has to be var so that we can use its address

	defaultProcMount         = corev1.DefaultProcMount
	volumesDefaultMode int32 = 420
)

// DeployOAuthServer - deployes an instance of an OpenShift OAuth server
// very simplified for now
// returns OAuth server url, cleanup function, error
func DeployOAuthServer(oc *exutil.CLI, idps []osinv1.IdentityProvider, configMaps []corev1.ConfigMap, secrets []corev1.Secret) (*tokencmd.RequestTokenOptions, func(), error) {
	oauthServerDataDir := exutil.FixturePath("testdata", "oauthserver")
	cleanups := func() {
		oc.AsAdmin().Run("delete").Args("clusterrolebinding", oc.Namespace()).Execute()
		oc.AsAdmin().Run("delete").Args("oauthclient", oc.Namespace()).Execute()
	}

	// create the CA bundle, Service, Route and SA
	for _, res := range []string{"cabundle-cm.yaml", "oauth-sa.yaml", "oauth-network.yaml"} {
		if err := oc.AsAdmin().Run("create").Args("-f", path.Join(oauthServerDataDir, res)).Execute(); err != nil {
			return nil, cleanups, err
		}
	}

	// the oauth server needs access to kube-system configmaps/extension-apiserver-authentication
	oauthSARolebinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: oc.Namespace(),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      SAName,
				Namespace: oc.Namespace(),
			},
		},
	}
	if _, err := oc.AdminKubeClient().RbacV1().ClusterRoleBindings().Create(oauthSARolebinding); err != nil {
		return nil, cleanups, err
	}

	// create the secrets and configmaps the OAuth server config requires to get the server going
	coreClient := oc.AdminKubeClient().CoreV1()
	cmClient := coreClient.ConfigMaps(oc.Namespace())
	secretsClient := coreClient.Secrets(oc.Namespace())

	for _, cm := range configMaps {
		if _, err := cmClient.Create(&cm); err != nil {
			return nil, cleanups, err
		}
	}

	for _, secret := range secrets {
		if _, err := secretsClient.Create(&secret); err != nil {
			return nil, cleanups, err
		}
	}

	// generate a session secret for the oauth server
	sessionSecret, err := randomSessionSecret()
	if err != nil {
		return nil, cleanups, err
	}
	if _, err := secretsClient.Create(sessionSecret); err != nil {
		return nil, cleanups, err
	}

	// get the route of the future OAuth server
	route, err := oc.AdminRouteClient().RouteV1().Routes(oc.Namespace()).Get(RouteName, metav1.GetOptions{})
	if err != nil {
		return nil, cleanups, err
	}
	routeURL := fmt.Sprintf("https://%s", route.Spec.Host)

	// prepare the config, inject it with the route URL and the IdP config we got
	config, err := oauthServerConfig(oc, routeURL, idps)
	if err != nil {
		return nil, cleanups, err
	}

	configBytes := encode(config)
	if configBytes == nil {
		return nil, cleanups, fmt.Errorf("error encoding the OSIN config")
	}

	// store the config in a ConfigMap that's to be mounted into the server's pod
	_, err = cmClient.Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "oauth-config",
		},
		Data: map[string]string{
			"oauth.conf": string(configBytes),
		},
	})
	if err != nil {
		return nil, cleanups, err
	}

	// get the OAuth server image that's used in the cluster
	image, err := getImage(oc)
	if err != nil {
		return nil, cleanups, err
	}

	// prepare the pod def, create secrets and CMs
	oauthServerPod, err := oauthServerPod(configMaps, secrets, image)
	if err != nil {
		return nil, cleanups, err
	}

	// finally create the oauth server, wait till it starts running
	if _, err := coreClient.Pods(oc.Namespace()).Create(oauthServerPod); err != nil {
		return nil, cleanups, err
	}

	err = wait.PollImmediate(1*time.Second, 45*time.Second, func() (bool, error) {
		pod, err := oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).Get("test-oauth-server", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if !exutil.CheckPodIsReady(*pod) {
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return nil, cleanups, err
	}

	if err = createOAuthClient(oc, routeURL); err != nil {
		return nil, cleanups, err
	}
	tokenReqOptions, err := getTokenOpts(oc.AdminConfig(), routeURL, oc.Namespace())
	if err != nil {
		return nil, cleanups, err
	}

	return tokenReqOptions, cleanups, nil
}

func oauthServerPod(configMaps []corev1.ConfigMap, secrets []corev1.Secret, image string) (*corev1.Pod, error) {
	oauthServerAsset := testdata.MustAsset("test/extended/testdata/oauthserver/oauth-pod.yaml")

	obj, err := helpers.ReadYAML(bytes.NewBuffer(oauthServerAsset), corev1.AddToScheme)
	if err != nil {
		return nil, err
	}

	oauthServerPod, ok := obj.(*corev1.Pod)
	if ok != true {
		return nil, err
	}

	volumes := oauthServerPod.Spec.Volumes
	volumeMounts := oauthServerPod.Spec.Containers[0].VolumeMounts

	for _, cm := range configMaps {
		volumes, volumeMounts = addCMMount(volumes, volumeMounts, &cm)
	}

	for _, sec := range secrets {
		volumes, volumeMounts = addSecretMount(volumes, volumeMounts, &sec)
	}

	oauthServerPod.Spec.Volumes = volumes
	oauthServerPod.Spec.Containers[0].VolumeMounts = volumeMounts
	oauthServerPod.Spec.Containers[0].Image = image

	return oauthServerPod, nil
}

func addCMMount(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, cm *corev1.ConfigMap) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes = append(volumes, corev1.Volume{
		Name: cm.ObjectMeta.Name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: cm.ObjectMeta.Name},
				DefaultMode:          &volumesDefaultMode,
			},
		},
	})

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      cm.ObjectMeta.Name,
		MountPath: GetDirPathFromConfigMapSecretName(cm.ObjectMeta.Name),
		ReadOnly:  true,
	})

	return volumes, volumeMounts
}

func addSecretMount(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount, secret *corev1.Secret) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes = append(volumes, corev1.Volume{
		Name: secret.ObjectMeta.Name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  secret.ObjectMeta.Name,
				DefaultMode: &volumesDefaultMode,
			},
		},
	})

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      secret.ObjectMeta.Name,
		MountPath: GetDirPathFromConfigMapSecretName(secret.ObjectMeta.Name),
		ReadOnly:  true,
	})

	return volumes, volumeMounts
}

func oauthServerConfig(oc *exutil.CLI, routeURL string, idps []osinv1.IdentityProvider) (*osinv1.OsinServerConfig, error) {
	adminConfigClient := configclient.NewForConfigOrDie(oc.AdminConfig()).ConfigV1()

	infrastructure, err := adminConfigClient.Infrastructures().Get("cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	console, err := adminConfigClient.Consoles().Get("cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	namedRouterCerts, err := routerCertsToSNIConfig(oc)
	if err != nil {
		return nil, err
	}

	return &osinv1.OsinServerConfig{
		GenericAPIServerConfig: configv1.GenericAPIServerConfig{
			ServingInfo: configv1.HTTPServingInfo{
				ServingInfo: configv1.ServingInfo{
					BindAddress: "0.0.0.0:6443",
					BindNetwork: "tcp4",
					// we have valid serving certs provided by service-ca
					// this is our main server cert which is used if SNI does not match
					CertInfo: configv1.CertInfo{
						CertFile: servingCertPathCert,
						KeyFile:  servingCertPathKey,
					},
					ClientCA:          "",
					NamedCertificates: namedRouterCerts,
					MinTLSVersion:     crypto.TLSVersionToNameOrDie(crypto.DefaultTLSVersion()),
					CipherSuites:      crypto.CipherSuitesToNamesOrDie(crypto.DefaultCiphers()),
				},
				MaxRequestsInFlight:   1000,
				RequestTimeoutSeconds: 5 * 60, // 5 minutes
			},
			AuditConfig: configv1.AuditConfig{},
			KubeClientConfig: configv1.KubeClientConfig{
				KubeConfig: "",
				ConnectionOverrides: configv1.ClientConnectionOverrides{
					QPS:   400,
					Burst: 400,
				},
			},
		},
		OAuthConfig: osinv1.OAuthConfig{
			MasterCA:                    &serviceCAPath, // we have valid serving certs provided by service-ca so we can use the service for loopback
			MasterURL:                   fmt.Sprintf(serviceURLFmt, oc.Namespace()),
			MasterPublicURL:             routeURL,
			LoginURL:                    infrastructure.Status.APIServerURL,
			AssetPublicURL:              console.Status.ConsoleURL, // set console route as valid 302 redirect for logout
			AlwaysShowProviderSelection: false,
			IdentityProviders:           idps,
			GrantConfig: osinv1.GrantConfig{
				Method:               osinv1.GrantHandlerDeny, // force denial as this field must be set per OAuth client
				ServiceAccountMethod: osinv1.GrantHandlerPrompt,
			},
			SessionConfig: &osinv1.SessionConfig{
				SessionSecretsFile:   sessionSecretPath,
				SessionMaxAgeSeconds: 5 * 60, // 5 minutes
				SessionName:          "ssn",
			},
			TokenConfig: osinv1.TokenConfig{
				AuthorizeTokenMaxAgeSeconds: 5 * 60,       // 5 minutes
				AccessTokenMaxAgeSeconds:    24 * 60 * 60, // 1 day
			},
		},
	}, nil
}

func routerCertsToSNIConfig(oc *exutil.CLI) ([]configv1.NamedCertificate, error) {
	routerSecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-config-managed").Get("router-certs", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	localRouterSecret := routerSecret.DeepCopy()
	localRouterSecret.ResourceVersion = ""
	localRouterSecret.Namespace = oc.Namespace()
	if _, err := oc.AdminKubeClient().CoreV1().Secrets(oc.Namespace()).Create(localRouterSecret); err != nil {
		return nil, err
	}

	var out []configv1.NamedCertificate
	for domain := range localRouterSecret.Data {
		out = append(out, configv1.NamedCertificate{
			Names: []string{"*." + domain}, // ingress domain is always a wildcard
			CertInfo: configv1.CertInfo{ // the cert and key are appended together
				CertFile: routerCertsDirPath + "/" + domain,
				KeyFile:  routerCertsDirPath + "/" + domain,
			},
		})
	}
	return out, nil
}

func randomSessionSecret() (*corev1.Secret, error) {
	skey, err := newSessionSecretsJSON()
	if err != nil {
		return nil, err
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "session-secret",
			Labels: map[string]string{
				"app": "test-oauth-server",
			},
		},
		Data: map[string][]byte{
			"session": skey,
		},
	}, nil
}

// this is less random than the actual secret generated in cluster-authentication-operator
func newSessionSecretsJSON() ([]byte, error) {
	const (
		sha256KeyLenBytes = sha256.BlockSize // max key size with HMAC SHA256
		aes256KeyLenBytes = 32               // max key size with AES (AES-256)
	)

	secrets := &legacyconfigv1.SessionSecrets{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SessionSecrets",
			APIVersion: "v1",
		},
		Secrets: []legacyconfigv1.SessionSecret{
			{
				Authentication: randomString(sha256KeyLenBytes), // 64 chars
				Encryption:     randomString(aes256KeyLenBytes), // 32 chars
			},
		},
	}
	secretsBytes, err := json.Marshal(secrets)
	if err != nil {
		return nil, fmt.Errorf("error marshalling the session secret: %v", err) // should never happen
	}

	return secretsBytes, nil
}

//randomString - random string of A-Z chars with len size
func randomString(size int) string {
	bytes := make([]byte, size)
	for i := 0; i < size; i++ {
		bytes[i] = byte(65 + rand.Intn(25))
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

// getImage will grab the hypershift image version from openshift-authentication ns
func getImage(oc *exutil.CLI) (string, error) {
	selector, _ := labels.Parse("app=oauth-openshift")
	pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-authentication").List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return "", err
	}
	return pods.Items[0].Spec.Containers[0].Image, nil
}

func getTokenOpts(config *restclient.Config, oauthServerURL, oauthClientName string) (*tokencmd.RequestTokenOptions, error) {
	tokenReqOptions := tokencmd.NewRequestTokenOptions(config, nil, "", "", false)
	// supply the info the client would otherwise ask from .well-known/oauth-authorization-server
	oauthClientConfig := &osincli.ClientConfig{
		ClientId:     oauthClientName,
		AuthorizeUrl: fmt.Sprintf("%s/oauth/authorize", oauthServerURL), // TODO: the endpoints are defined in vendor/github.com/openshift/library-go/pkg/oauth/oauthdiscovery/urls.go
		TokenUrl:     fmt.Sprintf("%s/oauth/token", oauthServerURL),
		RedirectUrl:  fmt.Sprintf("%s/oauth/token/implicit", oauthServerURL),
	}

	if err := osincli.PopulatePKCE(oauthClientConfig); err != nil {
		return nil, err
	}
	tokenReqOptions.OsinConfig = oauthClientConfig
	tokenReqOptions.Issuer = oauthServerURL

	return tokenReqOptions, nil
}

func createOAuthClient(oc *exutil.CLI, routeURL string) error {
	_, err := oc.AdminOAuthClient().OauthV1().OAuthClients().
		Create(&oauthv1.OAuthClient{
			ObjectMeta: metav1.ObjectMeta{
				Name: oc.Namespace(),
			},
			GrantMethod:           oauthv1.GrantHandlerAuto,
			RedirectURIs:          []string{fmt.Sprintf("%s/oauth/token/implicit", routeURL)},
			RespondWithChallenges: true,
		})
	return err
}
