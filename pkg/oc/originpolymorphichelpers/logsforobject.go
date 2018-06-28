package originpolymorphichelpers

import (
	"errors"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsmanualclient "github.com/openshift/origin/pkg/apps/client/internalversion"
	appsclientinternal "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildmanualclient "github.com/openshift/origin/pkg/build/client/internalversion"
	buildclientinternal "github.com/openshift/origin/pkg/build/generated/internalclientset"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

func NewLogsForObjectFn(delegate polymorphichelpers.LogsForObjectFunc) polymorphichelpers.LogsForObjectFunc {
	return func(restClientGetter genericclioptions.RESTClientGetter, object, options runtime.Object, timeout time.Duration) (*rest.Request, error) {
		clientConfig, err := restClientGetter.ToRESTConfig()
		if err != nil {
			return nil, err
		}

		switch t := object.(type) {
		case *appsapi.DeploymentConfig:
			dopts, ok := options.(*appsapi.DeploymentLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a DeploymentLogOptions")
			}
			appsClient, err := appsclientinternal.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			return appsmanualclient.NewRolloutLogClient(appsClient.Apps().RESTClient(), t.Namespace).Logs(t.Name, *dopts), nil
		case *buildapi.Build:
			bopts, ok := options.(*buildapi.BuildLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a BuildLogOptions")
			}
			if bopts.Version != nil {
				return nil, errors.New("cannot specify a version and a build")
			}
			buildClient, err := buildclientinternal.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			return buildmanualclient.NewBuildLogClient(buildClient.Build().RESTClient(), t.Namespace).Logs(t.Name, *bopts), nil
		case *buildapi.BuildConfig:
			bopts, ok := options.(*buildapi.BuildLogOptions)
			if !ok {
				return nil, errors.New("provided options object is not a BuildLogOptions")
			}
			buildClient, err := buildclientinternal.NewForConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			logClient := buildmanualclient.NewBuildLogClient(buildClient.Build().RESTClient(), t.Namespace)
			builds, err := buildClient.Build().Builds(t.Namespace).List(metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			builds.Items = buildapi.FilterBuilds(builds.Items, buildapi.ByBuildConfigPredicate(t.Name))
			if len(builds.Items) == 0 {
				return nil, fmt.Errorf("no builds found for %q", t.Name)
			}
			if bopts.Version != nil {
				// If a version has been specified, try to get the logs from that build.
				desired := buildutil.BuildNameForConfigVersion(t.Name, int(*bopts.Version))
				return logClient.Logs(desired, *bopts), nil
			}
			sort.Sort(sort.Reverse(buildapi.BuildSliceByCreationTimestamp(builds.Items)))
			return logClient.Logs(builds.Items[0].Name, *bopts), nil

		default:
			return delegate(restClientGetter, object, options, timeout)
		}
	}
}
