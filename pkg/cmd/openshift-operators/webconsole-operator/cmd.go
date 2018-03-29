package webconsole_operator

import (
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/client/leaderelectionconfig"

	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	aggregatorinstall "k8s.io/kube-aggregator/pkg/apis/apiregistration/install"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	webconsoleconfigv1 "github.com/openshift/api/webconsole/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/controller"
	"github.com/openshift/origin/pkg/cmd/server/origin"
)

const (
	RecommendedWebConsoleOperatorName = "openshift-webconsole-operator"
)

type WebConsoleOperator struct {
	Output io.Writer
}

var longDescription = templates.LongDesc(`
	Install the OpenShift webconsoles`)

func NewWebConsoleOperatorCommand(name string, out, errout io.Writer) *cobra.Command {
	options := &WebConsoleOperator{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Install the OpenShift webconsoles",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			// TODO: register our own scheme
			aggregatorinstall.Install(legacyscheme.GroupFactoryRegistry, legacyscheme.Registry, legacyscheme.Scheme)
			webconsoleconfigv1.AddToScheme(legacyscheme.Scheme)

			kcmdutil.CheckErr(options.Validate())

			origin.StartProfiler()

			if err := options.RunWebConsoleOperator(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(errout, "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(errout, "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	return cmd
}

func (o *WebConsoleOperator) Validate() error {
	return nil
}

func (o *WebConsoleOperator) RunWebConsoleOperator() error {
	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	operator := &controller.WebConsoleOperatorStarter{
		ClientConfig: clientConfig,
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "openshift-webconsole"})
	rl, err := resourcelock.New(
		resourcelock.ConfigMapsResourceLock,
		"openshift-core-operators",
		RecommendedWebConsoleOperatorName,
		kubeClient.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      string(uuid.NewUUID()),
			EventRecorder: eventRecorder,
		})
	if err != nil {
		return err
	}
	leaderelection.RunOrDie(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: leaderelectionconfig.DefaultLeaseDuration,
		RenewDeadline: leaderelectionconfig.DefaultRenewDeadline,
		RetryPeriod:   leaderelectionconfig.DefaultRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: operator.Run,
			OnStoppedLeading: func() {
				glog.Fatalf("leaderelection lost")
			},
		},
	})

	return fmt.Errorf("exiting")
}
