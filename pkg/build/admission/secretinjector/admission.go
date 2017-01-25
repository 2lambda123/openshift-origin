package secretinjector

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/glog"
	authclient "github.com/openshift/origin/pkg/auth/client"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/util/urlpattern"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/client/restclient"
)

func init() {
	admission.RegisterPlugin("openshift.io/BuildConfigSecretInjector", func(c clientset.Interface, config io.Reader) (admission.Interface, error) {
		return &secretInjector{
			Handler: admission.NewHandler(admission.Create),
		}, nil
	})
}

type secretInjector struct {
	*admission.Handler
	restClientConfig restclient.Config
}

func (si *secretInjector) Admit(attr admission.Attributes) (err error) {
	bc, ok := attr.GetObject().(*buildapi.BuildConfig)
	if !ok {
		return nil
	}

	if bc.Spec.Source.SourceSecret != nil || bc.Spec.Source.Git == nil {
		return nil
	}

	impersonatingConfig := si.restClientConfig
	impersonatingConfig.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return authclient.NewImpersonatingRoundTripper(attr.GetUserInfo(), si.restClientConfig.WrapTransport(rt))
	}

	client, err := coreclient.NewForConfig(&impersonatingConfig)
	if err != nil {
		glog.V(2).Infof("secretinjector: could not create client: %v", err)
		return nil
	}

	namespace := attr.GetNamespace()

	url, err := url.Parse(bc.Spec.Source.Git.URI)
	if err != nil {
		glog.V(2).Infof(`secretinjector: buildconfig "%s/%s": URI %q parse failed: %v`, namespace, bc.GetName(), bc.Spec.Source.Git.URI, err)
		return nil
	}

	secrets, err := client.Secrets(namespace).List(api.ListOptions{})
	if err != nil {
		glog.V(2).Infof("secretinjector: failed to list Secrets: %v", err)
		return nil
	}

	patterns := []*urlpattern.URLPattern{}
	for _, secret := range secrets.Items {
		if secret.Type == api.SecretTypeBasicAuth && url.Scheme == "ssh" ||
			secret.Type == api.SecretTypeSSHAuth && url.Scheme != "ssh" {
			continue
		}

		for k, v := range secret.GetAnnotations() {
			if strings.HasPrefix(k, buildapi.BuildSourceSecretMatchURIAnnotationPrefix) {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}

				pattern, err := urlpattern.NewURLPattern(v)
				if err != nil {
					glog.V(2).Infof(`secretinjector: buildconfig "%s/%s": unparseable annotation %q: %v`, namespace, bc.GetName(), k, err)
					continue
				}

				pattern.Cookie = secret.GetName()
				patterns = append(patterns, pattern)
			}
		}
	}

	if match := urlpattern.Match(patterns, url); match != nil {
		secretName := match.Cookie.(string)
		glog.V(4).Infof(`secretinjector: matched secret "%s/%s" to buildconfig "%s"`, namespace, secretName, bc.GetName())
		bc.Spec.Source.SourceSecret = &api.LocalObjectReference{Name: secretName}
	}

	return nil
}

func (si *secretInjector) SetRESTClientConfig(restClientConfig restclient.Config) {
	si.restClientConfig = restClientConfig
}
