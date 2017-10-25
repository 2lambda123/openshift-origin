package config

import (
	"crypto/x509"
	"net/url"
	"reflect"
	"strings"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/third_party/forked/golang/netutil"
	x509request "k8s.io/apiserver/pkg/authentication/request/x509"
	restclient "k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
)

// GetClusterNicknameFromConfig returns host:port of the clientConfig.Host, with .'s replaced by -'s
func GetClusterNicknameFromConfig(clientCfg *restclient.Config) (string, error) {
	return GetClusterNicknameFromURL(clientCfg.Host)
}

// GetClusterNicknameFromURL returns host:port of the apiServerLocation, with .'s replaced by -'s
func GetClusterNicknameFromURL(apiServerLocation string) (string, error) {
	u, err := url.Parse(apiServerLocation)
	if err != nil {
		return "", err
	}
	hostPort := netutil.CanonicalAddr(u)

	// we need a character other than "." to avoid conflicts with.  replace with '-'
	return strings.Replace(hostPort, ".", "-", -1), nil
}

// GetUserNicknameFromConfig returns "username(as known by the server)/GetClusterNicknameFromConfig".  This allows tab completion for switching users to
// work easily and obviously.
func GetUserNicknameFromConfig(clientCfg *restclient.Config) (string, error) {
	userPartOfNick, err := getUserPartOfNickname(clientCfg)
	if err != nil {
		return "", err
	}

	clusterNick, err := GetClusterNicknameFromConfig(clientCfg)
	if err != nil {
		return "", err
	}

	return userPartOfNick + "/" + clusterNick, nil
}

func GetUserNicknameFromCert(clusterNick string, chain ...*x509.Certificate) (string, error) {
	userInfo, _, err := x509request.CommonNameUserConversion(chain)
	if err != nil {
		return "", err
	}

	return userInfo.GetName() + "/" + clusterNick, nil
}

func getUserPartOfNickname(clientCfg *restclient.Config) (string, error) {
	userClient, err := userclient.NewForConfig(clientCfg)
	if err != nil {
		return "", err
	}
	userInfo, err := userClient.User().Users().Get("~", metav1.GetOptions{})
	if kerrors.IsNotFound(err) || kerrors.IsForbidden(err) {
		// if we're talking to kube (or likely talking to kube), take a best guess consistent with login
		switch {
		case len(clientCfg.BearerToken) > 0:
			userInfo.Name = clientCfg.BearerToken
		case len(clientCfg.Username) > 0:
			userInfo.Name = clientCfg.Username
		}
	} else if err != nil {
		return "", err
	}

	return userInfo.Name, nil
}

// GetContextNicknameFromConfig returns "namespace/GetClusterNicknameFromConfig/username(as known by the server)".  This allows tab completion for switching projects/context
// to work easily.  First tab is the most selective on project.  Second stanza in the next most selective on cluster name.  The chances of a user trying having
// one projects on a single server that they want to operate against with two identities is low, so username is last.
func GetContextNicknameFromConfig(namespace string, clientCfg *restclient.Config) (string, error) {
	userPartOfNick, err := getUserPartOfNickname(clientCfg)
	if err != nil {
		return "", err
	}

	clusterNick, err := GetClusterNicknameFromConfig(clientCfg)
	if err != nil {
		return "", err
	}

	return namespace + "/" + clusterNick + "/" + userPartOfNick, nil
}

func GetContextNickname(namespace, clusterNick, userNick string) string {
	tokens := strings.SplitN(userNick, "/", 2)
	return namespace + "/" + clusterNick + "/" + tokens[0]
}

// CreateConfig takes a clientCfg and builds a config (kubeconfig style) from it.
func CreateConfig(namespace string, clientCfg *restclient.Config) (*clientcmdapi.Config, error) {
	clusterNick, err := GetClusterNicknameFromConfig(clientCfg)
	if err != nil {
		return nil, err
	}

	userNick, err := GetUserNicknameFromConfig(clientCfg)
	if err != nil {
		return nil, err
	}

	contextNick, err := GetContextNicknameFromConfig(namespace, clientCfg)
	if err != nil {
		return nil, err
	}

	config := clientcmdapi.NewConfig()

	credentials := clientcmdapi.NewAuthInfo()
	credentials.Token = clientCfg.BearerToken
	credentials.ClientCertificate = clientCfg.TLSClientConfig.CertFile
	if len(credentials.ClientCertificate) == 0 {
		credentials.ClientCertificateData = clientCfg.TLSClientConfig.CertData
	}
	credentials.ClientKey = clientCfg.TLSClientConfig.KeyFile
	if len(credentials.ClientKey) == 0 {
		credentials.ClientKeyData = clientCfg.TLSClientConfig.KeyData
	}
	config.AuthInfos[userNick] = credentials

	cluster := clientcmdapi.NewCluster()
	cluster.Server = clientCfg.Host
	cluster.CertificateAuthority = clientCfg.CAFile
	if len(cluster.CertificateAuthority) == 0 {
		cluster.CertificateAuthorityData = clientCfg.CAData
	}
	cluster.InsecureSkipTLSVerify = clientCfg.Insecure
	config.Clusters[clusterNick] = cluster

	context := clientcmdapi.NewContext()
	context.Cluster = clusterNick
	context.AuthInfo = userNick
	context.Namespace = namespace
	config.Contexts[contextNick] = context
	config.CurrentContext = contextNick

	return config, nil
}

// MergeConfig adds the additional Config stanzas to the startingConfig.  It blindly stomps clusters and users, but
// it searches for a matching context before writing a new one.
func MergeConfig(startingConfig, addition clientcmdapi.Config) (*clientcmdapi.Config, error) {
	ret := startingConfig

	for requestedKey, value := range addition.Clusters {
		ret.Clusters[requestedKey] = value
	}

	for requestedKey, value := range addition.AuthInfos {
		ret.AuthInfos[requestedKey] = value
	}

	resolvedCurrentContext := addition.CurrentContext

	requestedContextNamesToActualContextNames := map[string]string{}
	for requestedKey, newContext := range addition.Contexts {
		// only add the new context if there is no existing context
		// with the same authInfo and cluster info - regardless of context name.
		ctxInfoExists := false
		for existingCtxName, existingContext := range startingConfig.Contexts {
			if existingContext.AuthInfo == newContext.AuthInfo &&
				existingContext.Cluster == newContext.Cluster {
				// if we are skipping the currentContext of the config we are adding on top of
				// the existing config, update the resolvedCurrentContext to the name of the
				// existing context that shares the same authInfo and clusterName as the newContext.
				if requestedKey == addition.CurrentContext {
					resolvedCurrentContext = existingCtxName
				}

				// copy existing namespace and extensions from what would have been the new context
				existingContext.Namespace = newContext.Namespace
				existingContext.Extensions = newContext.Extensions

				ctxInfoExists = true
				break
			}
		}
		if ctxInfoExists {
			continue
		}

		actualContext := clientcmdapi.NewContext()
		actualContext.AuthInfo = newContext.AuthInfo
		actualContext.Cluster = newContext.Cluster
		actualContext.Namespace = newContext.Namespace
		actualContext.Extensions = newContext.Extensions

		if existingName := FindExistingContextName(startingConfig, *actualContext); len(existingName) > 0 {
			// if this already exists, just move to the next, our job is done
			requestedContextNamesToActualContextNames[requestedKey] = existingName
			continue
		}

		requestedContextNamesToActualContextNames[requestedKey] = requestedKey
		ret.Contexts[requestedKey] = actualContext
	}

	if len(addition.CurrentContext) > 0 {
		if newCurrentContext, exists := requestedContextNamesToActualContextNames[addition.CurrentContext]; exists {
			ret.CurrentContext = newCurrentContext
		} else {
			ret.CurrentContext = resolvedCurrentContext
		}
	}

	return &ret, nil
}

// FindExistingContextName finds the nickname for the passed context
func FindExistingContextName(haystack clientcmdapi.Config, needle clientcmdapi.Context) string {
	for key, context := range haystack.Contexts {
		context.LocationOfOrigin = ""
		if reflect.DeepEqual(context, needle) {
			return key
		}
	}

	return ""
}
