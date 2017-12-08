package buildconfig

import (
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/client"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	"github.com/openshift/origin/pkg/build/webhook"
)

type WebHook struct {
	groupVersion      schema.GroupVersion
	buildConfigClient buildclient.BuildInterface
	instantiator      client.BuildConfigInstantiator
	plugins           map[string]webhook.Plugin
}

// NewWebHookREST returns the webhook handler
func NewWebHookREST(buildConfigClient buildclient.BuildInterface, groupVersion schema.GroupVersion, plugins map[string]webhook.Plugin) *WebHook {
	return newWebHookREST(buildConfigClient, client.BuildConfigInstantiatorClient{Client: buildConfigClient}, groupVersion, plugins)
}

// this supports simple unit testing
func newWebHookREST(buildConfigClient buildclient.BuildInterface, instantiator client.BuildConfigInstantiator, groupVersion schema.GroupVersion, plugins map[string]webhook.Plugin) *WebHook {
	return &WebHook{
		groupVersion:      groupVersion,
		buildConfigClient: buildConfigClient,
		instantiator:      instantiator,
		plugins:           plugins,
	}
}

// New() responds with the status object.
func (h *WebHook) New() runtime.Object {
	return &buildapi.Build{}
}

// Connect responds to connections with a ConnectHandler
func (h *WebHook) Connect(ctx apirequest.Context, name string, options runtime.Object, responder rest.Responder) (http.Handler, error) {
	return &WebHookHandler{
		ctx:               ctx,
		name:              name,
		options:           options.(*kapi.PodProxyOptions),
		responder:         responder,
		groupVersion:      h.groupVersion,
		plugins:           h.plugins,
		buildConfigClient: h.buildConfigClient,
		instantiator:      h.instantiator,
	}, nil
}

// NewConnectionOptions identifies the options that should be passed to this hook
func (h *WebHook) NewConnectOptions() (runtime.Object, bool, string) {
	return &kapi.PodProxyOptions{}, true, "path"
}

// ConnectMethods returns the supported web hook types.
func (h *WebHook) ConnectMethods() []string {
	return []string{"POST"}
}

// WebHookHandler responds to web hook requests from the master.
type WebHookHandler struct {
	ctx               apirequest.Context
	name              string
	options           *kapi.PodProxyOptions
	responder         rest.Responder
	groupVersion      schema.GroupVersion
	plugins           map[string]webhook.Plugin
	buildConfigClient buildclient.BuildInterface
	instantiator      client.BuildConfigInstantiator
}

// ServeHTTP implements the standard http.Handler
func (h *WebHookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.ProcessWebHook(w, r, h.ctx, h.name, h.options.Path); err != nil {
		h.responder.Error(err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ProcessWebHook does the actual work of processing the webhook request
func (w *WebHookHandler) ProcessWebHook(writer http.ResponseWriter, req *http.Request, ctx apirequest.Context, name, subpath string) error {
	parts := strings.Split(strings.TrimPrefix(subpath, "/"), "/")
	if len(parts) != 2 {
		return errors.NewBadRequest(fmt.Sprintf("unexpected hook subpath %s", subpath))
	}
	secret, hookType := parts[0], parts[1]

	plugin, ok := w.plugins[hookType]
	if !ok {
		return errors.NewNotFound(buildapi.LegacyResource("buildconfighook"), hookType)
	}

	config, err := w.buildConfigClient.BuildConfigs(apirequest.NamespaceValue(ctx)).Get(name, metav1.GetOptions{})
	if err != nil {
		// clients should not be able to find information about build configs in
		// the system unless the config exists and the secret matches
		return errors.NewUnauthorized(fmt.Sprintf("the webhook %q for %q did not accept your secret", hookType, name))
	}

	revision, envvars, dockerStrategyOptions, proceed, err := plugin.Extract(config, secret, "", req)
	if !proceed {
		switch err {
		case webhook.ErrSecretMismatch, webhook.ErrHookNotEnabled:
			return errors.NewUnauthorized(fmt.Sprintf("the webhook %q for %q did not accept your secret", hookType, name))
		case webhook.MethodNotSupported:
			return errors.NewMethodNotSupported(buildapi.Resource("buildconfighook"), req.Method)
		}
		if _, ok := err.(*errors.StatusError); !ok && err != nil {
			return errors.NewInternalError(fmt.Errorf("hook failed: %v", err))
		}
		return err
	}
	warning := err

	buildTriggerCauses := generateBuildTriggerInfo(revision, hookType, secret)
	request := &buildapi.BuildRequest{
		TriggeredBy: buildTriggerCauses,
		ObjectMeta:  metav1.ObjectMeta{Name: name},
		Revision:    revision,
		Env:         envvars,
		DockerStrategyOptions: dockerStrategyOptions,
	}

	newBuild, err := w.instantiator.Instantiate(config.Namespace, request)
	if err != nil {
		return errors.NewInternalError(fmt.Errorf("could not generate a build: %v", err))
	}

	// Send back the build name so that the client can alert the user.
	if newBuildEncoded, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(w.groupVersion), newBuild); err != nil {
		utilruntime.HandleError(err)
	} else {
		writer.Write(newBuildEncoded)
	}

	return warning
}

func generateBuildTriggerInfo(revision *buildapi.SourceRevision, hookType, secret string) (buildTriggerCauses []buildapi.BuildTriggerCause) {
	hiddenSecret := fmt.Sprintf("%s***", secret[:(len(secret)/2)])
	switch {
	case hookType == "generic":
		buildTriggerCauses = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseGenericMsg,
				GenericWebHook: &buildapi.GenericWebHookCause{
					Revision: revision,
					Secret:   hiddenSecret,
				},
			})
	case hookType == "github":
		buildTriggerCauses = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseGithubMsg,
				GitHubWebHook: &buildapi.GitHubWebHookCause{
					Revision: revision,
					Secret:   hiddenSecret,
				},
			})
	case hookType == "gitlab":
		buildTriggerCauses = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseGitLabMsg,
				GitLabWebHook: &buildapi.GitLabWebHookCause{
					CommonWebHookCause: buildapi.CommonWebHookCause{
						Revision: revision,
						Secret:   hiddenSecret,
					},
				},
			})
	case hookType == "bitbucket":
		buildTriggerCauses = append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseBitbucketMsg,
				BitbucketWebHook: &buildapi.BitbucketWebHookCause{
					CommonWebHookCause: buildapi.CommonWebHookCause{
						Revision: revision,
						Secret:   hiddenSecret,
					},
				},
			})
	}
	return buildTriggerCauses
}
