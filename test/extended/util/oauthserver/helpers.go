package oauthserver

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	restclient "k8s.io/client-go/rest"

	osinv1 "github.com/openshift/api/osin/v1"
	userv1 "github.com/openshift/api/user/v1"
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/openshift/oc/pkg/helpers/tokencmd"
)

var (
	osinScheme = runtime.NewScheme()
	codecs     = serializer.NewCodecFactory(osinScheme)
	encoder    = codecs.LegacyCodec(osinv1.GroupVersion)
)

func init() {
	utilruntime.Must(osinv1.Install(osinScheme))
}

func RequestTokenForUser(reqOpts *tokencmd.RequestTokenOptions, username, password string) (string, error) {
	reqOpts.Handler = &tokencmd.BasicChallengeHandler{
		Host:     reqOpts.ClientConfig.Host,
		Username: username,
		Password: password,
	}

	return reqOpts.RequestToken()
}

func GetRawExtensionForOsinProvider(obj runtime.Object) (*runtime.RawExtension, error) {
	objBytes := encode(obj)
	if objBytes == nil {
		return nil, fmt.Errorf("unable to encode the object: %v", obj)
	}
	return &runtime.RawExtension{Raw: objBytes}, nil
}

func GetUserForToken(config *restclient.Config, token, expectedUsername string) (*userv1.User, error) {
	userConfig := restclient.AnonymousClientConfig(config)
	userConfig.BearerToken = token
	userClient, err := userv1client.NewForConfig(userConfig)
	if err != nil {
		return nil, err
	}

	user, err := userClient.Users().Get("~", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return user, err
}

func GetDirPathFromConfigMapSecretName(name string) string {
	return fmt.Sprintf("%s/%s", configObjectsDir, name) // always concat with / in case this is run on windows
}

func GetPathFromConfigMapSecretName(name, key string) string {
	return fmt.Sprintf("%s/%s/%s", configObjectsDir, name, key)
}

func encode(obj runtime.Object) []byte {
	bytes, err := runtime.Encode(encoder, obj)
	if err != nil {
		return nil
	}
	return bytes
}
